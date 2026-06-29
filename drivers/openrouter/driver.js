const { downstream } = require('../sdk/driver');

function prepareRequest(ctx) {
  const { envelope, credential, baseURL } = ctx;
  const payload = envelope.payload || {};
  const streaming = Boolean(payload.stream);
  const url = `${baseURL.replace(/\/$/, '')}/chat/completions`;
  const headers = {
    Authorization: `Bearer ${credential}`,
    'Content-Type': 'application/json',
    'HTTP-Referer': 'https://quarkgate.dev',
    'X-Title': 'QuarkGate',
  };
  return downstream(url, 'POST', headers, payload, streaming, 'openrouter', 'chat.completions.create', estimateMaxCost(envelope));
}

function estimateMaxCost(envelope) {
  const hints = envelope.metering_hints || {};
  if (hints.estimated_max_credits) {
    return hints.estimated_max_credits * 1_000_000;
  }
  const payload = envelope.payload || {};
  const maxTokens = payload.max_tokens || 1024;
  return (maxTokens * 900) + 500_000;
}

function normalizeUsage(raw, envelope) {
  const out = { ...raw };
  if (raw.cost !== undefined && raw.cost_usd === undefined) {
    out.cost_usd = Number(raw.cost);
  }
  if (envelope && envelope.payload && envelope.payload.model) {
    out.model = envelope.payload.model;
  }
  return out;
}

function parseResponse(ctx) {
  const { body, streaming } = ctx;
  if (streaming) {
    return parseSSEUsage(body);
  }
  try {
    const j = JSON.parse(body);
    if (j.usage) return j.usage;
    if (j.data && j.data.usage) return j.data.usage;
  } catch (_) {
    /* ignore */
  }
  return {};
}

function parseSSEUsage(body) {
  const lines = body.split('\n');
  let usage = null;
  for (const line of lines) {
    const s = line.trim();
    if (!s.startsWith('data: ')) continue;
    const data = s.slice(6);
    if (data === '[DONE]') continue;
    try {
      const chunk = JSON.parse(data);
      if (chunk.usage) usage = chunk.usage;
    } catch (_) {
      /* ignore */
    }
  }
  return usage || {};
}

async function healthCheck(ctx) {
  const start = Date.now();
  const url = `${ctx.baseURL.replace(/\/$/, '')}/models`;
  try {
    const res = await fetch(url, {
      headers: { Authorization: `Bearer ${ctx.credential}` },
    });
    return {
      ok: res.ok,
      latency_ms: Date.now() - start,
      message: res.ok ? 'ok' : `status ${res.status}`,
    };
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
