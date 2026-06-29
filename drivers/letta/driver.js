const { downstream } = require('../sdk/driver');

function prepareRequest(ctx) {
  const { envelope, credential, baseURL } = ctx;
  const payload = envelope.payload || {};
  const agentId = payload.agent_id;
  if (!agentId) {
    throw new Error('agent_id required in payload');
  }
  const url = `${baseURL.replace(/\/$/, '')}/v1/agents/${agentId}/messages`;
  const headers = {
    Authorization: `Bearer ${credential}`,
    'Content-Type': 'application/json',
  };
  const body = {
    messages: payload.messages,
    stream: payload.stream || false,
  };
  return downstream(url, 'POST', headers, body, Boolean(body.stream), 'letta', 'agents.messages.create', estimateMaxCost(envelope));
}

function estimateMaxCost(envelope) {
  return 5_000_000;
}

function normalizeUsage(raw) {
  return { ...raw, api_calls: raw.api_calls || 1 };
}

function parseResponse(ctx) {
  try {
    const j = JSON.parse(ctx.body || '{}');
    const usage = { api_calls: 1 };
    if (j.usage) {
      Object.assign(usage, j.usage);
    }
    return usage;
  } catch (_) {
    return { api_calls: 1 };
  }
}

async function healthCheck(ctx) {
  const start = Date.now();
  try {
    const res = await fetch(`${ctx.baseURL.replace(/\/$/, '')}/v1/health`, {
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
  healthCheck,
};
