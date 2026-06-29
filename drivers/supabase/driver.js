const { downstream } = require('../sdk/driver');

function prepareRequest(ctx) {
  const { envelope, credential, baseURL } = ctx;
  const payload = envelope.payload || {};
  const op = envelope.operation;

  if (op === 'rpc.match_documents') {
    const url = `${baseURL.replace(/\/$/, '')}/rest/v1/rpc/match_documents`;
    const headers = {
      apikey: credential,
      Authorization: `Bearer ${credential}`,
      'Content-Type': 'application/json',
      Prefer: 'return=representation',
    };
    return downstream(url, 'POST', headers, payload, false, 'supabase', op, 500_000);
  }

  const table = payload.table || 'documents';
  const method = payload.method || 'GET';
  let url = `${baseURL.replace(/\/$/, '')}/rest/v1/${table}`;
  if (payload.query) {
    url += `?${payload.query}`;
  }
  const headers = {
    apikey: credential,
    Authorization: `Bearer ${credential}`,
    'Content-Type': 'application/json',
    Prefer: payload.prefer || 'return=representation',
  };
  const body = method === 'GET' ? undefined : payload.body;
  return downstream(url, method, headers, body, false, 'supabase', 'rest.query', 100_000);
}

function estimateMaxCost(envelope) {
  const op = envelope.operation;
  if (op === 'rpc.match_documents') {
    return 500_000;
  }
  return 100_000;
}

function normalizeUsage(raw) {
  return { ...raw };
}

function parseResponse(ctx) {
  const usage = { db_reads: 0, db_writes: 0, vec_queries: 0 };
  const h = ctx.headers || {};
  const cr = h['content-range'] || h['Content-Range'];
  if (cr) {
    const parts = String(cr).split('/');
    if (parts.length === 2 && parts[1]) {
      usage.db_reads = parseInt(parts[1], 10) || 1;
    } else {
      usage.db_reads = 1;
    }
  }
  try {
    const j = JSON.parse(ctx.body || '[]');
    if (Array.isArray(j)) {
      usage.db_reads = j.length;
      if (ctx.envelope && ctx.envelope.operation === 'rpc.match_documents') {
        usage.vec_queries = 1;
      }
    }
  } catch (_) {
    usage.db_reads = 1;
  }
  if (ctx.envelope && ctx.envelope.operation === 'rpc.match_documents') {
    usage.vec_queries = 1;
  }
  return usage;
}

async function healthCheck(ctx) {
  const start = Date.now();
  try {
    const res = await fetch(`${ctx.baseURL.replace(/\/$/, '')}/rest/v1/`, {
      headers: { apikey: ctx.credential },
    });
    return { ok: res.status < 500, latency_ms: Date.now() - start, message: `status ${res.status}` };
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
