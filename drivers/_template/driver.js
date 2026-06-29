const { downstream } = require('../sdk/driver');

function prepareRequest(ctx) {
  throw new Error('implement prepareRequest');
}

function estimateMaxCost(envelope) {
  return 1_000_000;
}

function normalizeUsage(raw, envelope) {
  return { ...raw };
}

function parseResponse(ctx) {
  return {};
}

async function healthCheck(ctx) {
  return { ok: true, latency_ms: 0, message: 'implement healthCheck' };
}

module.exports = {
  prepareRequest,
  estimateMaxCost,
  normalizeUsage,
  parseResponse,
  healthCheck,
};
