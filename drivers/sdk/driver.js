/**
 * QuarkGate Driver SDK types and helpers.
 */

function envelope(provider, operation, payload, hints) {
  return {
    quarkgate_version: '1',
    provider,
    operation,
    payload,
    metering_hints: hints,
  };
}

function downstream(url, method, headers, body, streaming, provider, operation, estimateMicro) {
  return {
    url,
    method,
    headers,
    body,
    streaming,
    provider,
    operation,
    estimate_micro: estimateMicro || 0,
  };
}

/** Helpers for driver stream metering fallbacks. */
function estimateOutputTokensFromChunk(data) {
  if (!data || data === '[DONE]') {
    return 0;
  }
  return Math.ceil(String(data).length / 4);
}

module.exports = { envelope, downstream, estimateOutputTokensFromChunk };
