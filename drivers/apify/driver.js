const { downstream } = require('../sdk/driver');

const POLL_INTERVAL_MS = 2000;
const POLL_MAX_MS = 300000;

function prepareRequest(ctx) {
  const { envelope, credential, baseURL } = ctx;
  const payload = envelope.payload || {};
  const op = envelope.operation || 'actor.run';
  const actorId = payload.actor_id || payload.actorId;
  if (!actorId) {
    throw new Error('actor_id required');
  }
  const url = `${baseURL.replace(/\/$/, '')}/acts/${actorId}/runs`;
  const headers = {
    Authorization: `Bearer ${credential}`,
    'Content-Type': 'application/json',
  };
  const body = {
    input: payload.input || {},
    timeout: payload.timeout || 300,
  };
  return downstream(url, 'POST', headers, body, false, 'apify', op, estimateMaxCost(envelope));
}

function estimateMaxCost(envelope) {
  const payload = envelope.payload || {};
  const timeout = payload.timeout || 300;
  return timeout * 50_000 + 1_000_000;
}

function normalizeUsage(raw) {
  const out = { ...raw };
  if (raw.compute_seconds !== undefined) {
    out.compute_seconds = Number(raw.compute_seconds);
  } else if (raw.runTimeSecs !== undefined) {
    out.compute_seconds = Number(raw.runTimeSecs);
  }
  if (!out.api_calls) out.api_calls = 1;
  return out;
}

function parseResponse(ctx) {
  try {
    const j = JSON.parse(ctx.body || '{}');
    const data = j.data || j;
    const usage = {
      api_calls: 1,
      compute_seconds: data.stats?.runTimeSecs || data.runTimeSecs || 0,
      run_id: data.id,
      status: data.status,
    };
    if (data.status === 'RUNNING' || data.status === 'READY') {
      usage.needs_poll = true;
    }
    return usage;
  } catch (_) {
    return { api_calls: 1 };
  }
}

async function pollRun(ctx) {
  const { baseURL, credential, poll_context } = ctx;
  const runId = poll_context.run_id;
  if (!runId) {
    throw new Error('run_id required in poll_context');
  }
  const url = `${baseURL.replace(/\/$/, '')}/actor-runs/${runId}`;
  const headers = { Authorization: `Bearer ${credential}` };
  const start = Date.now();
  while (Date.now() - start < POLL_MAX_MS) {
    const res = await fetch(url, { headers });
    const data = await res.json();
    const status = data.data?.status || data.status;
    if (status === 'SUCCEEDED') {
      const secs = data.data?.stats?.runTimeSecs || data.stats?.runTimeSecs || 0;
      return {
        raw_usage: { compute_seconds: secs, api_calls: 1, status },
        done: true,
      };
    }
    if (status === 'FAILED' || status === 'ABORTED' || status === 'TIMED-OUT') {
      return {
        raw_usage: { compute_seconds: 0, api_calls: 1, status, failed: true },
        done: true,
      };
    }
    await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
  }
  return {
    raw_usage: { compute_seconds: POLL_MAX_MS / 1000, api_calls: 1, timeout: true },
    done: true,
  };
}

async function healthCheck(ctx) {
  const start = Date.now();
  try {
    const res = await fetch(`${ctx.baseURL.replace(/\/$/, '')}/acts`, {
      headers: { Authorization: `Bearer ${ctx.credential}` },
    });
    return { ok: res.ok, latency_ms: Date.now() - start, message: res.ok ? 'ok' : `status ${res.status}` };
  } catch (e) {
    return { ok: false, latency_ms: Date.now() - start, message: e.message };
  }
}

module.exports = {
  prepareRequest,
  estimateMaxCost,
  normalizeUsage,
  parseResponse,
  pollRun,
  healthCheck,
};
