/**
 * QuarkGate driver host — JSON stdin/stdout IPC from Go edge.
 */
const fs = require('fs');
const path = require('path');

function loadDriver(provider) {
  const jsPath = path.join(__dirname, '..', provider, 'driver.js');
  if (fs.existsSync(jsPath)) {
    return require(jsPath);
  }
  const pyPath = path.join(__dirname, '..', provider, 'driver.py');
  if (fs.existsSync(pyPath)) {
    throw new Error('use host.py for Python drivers');
  }
  throw new Error(`driver not found for ${provider}`);
}

function fail(err, code = 'DRIVER_ERROR') {
  console.error(JSON.stringify({ error: err.message || String(err), code }));
  process.exit(1);
}

async function main() {
  let input;
  try {
    input = JSON.parse(fs.readFileSync(0, 'utf8'));
  } catch (e) {
    fail(e);
  }

  const provider = input.provider;
  if (!provider) {
    fail(new Error('provider required'));
  }

  const driver = loadDriver(provider);
  const envelope = typeof input.envelope === 'string'
    ? JSON.parse(input.envelope)
    : input.envelope;

  switch (input.action) {
    case 'estimate': {
      const micro = driver.estimateMaxCost(envelope || { payload: {} });
      console.log(JSON.stringify({ estimate_micro: micro }));
      return;
    }
    case 'prepare': {
      const downstream = driver.prepareRequest({
        envelope,
        credential: input.credential,
        baseURL: input.baseURL,
      });
      console.log(JSON.stringify(downstream));
      return;
    }
    case 'normalize': {
      const raw = input.raw_usage || {};
      if (typeof driver.normalizeUsage === 'function') {
        const patched = driver.normalizeUsage(raw, envelope);
        console.log(JSON.stringify({ raw_usage: patched }));
      } else {
        console.log(JSON.stringify({ raw_usage: raw }));
      }
      return;
    }
    case 'parseResponse': {
      const headers = input.headers || {};
      const body = input.body || '';
      if (typeof driver.parseResponse === 'function') {
        const raw = driver.parseResponse({
          headers,
          body,
          streaming: Boolean(input.streaming),
          envelope,
        });
        console.log(JSON.stringify({ raw_usage: raw || {} }));
      } else {
        console.log(JSON.stringify({ raw_usage: {} }));
      }
      return;
    }
    case 'healthCheck': {
      if (typeof driver.healthCheck === 'function') {
        const result = await driver.healthCheck({
          baseURL: input.baseURL,
          credential: input.credential,
        });
        console.log(JSON.stringify(result));
      } else {
        console.log(JSON.stringify({ ok: true, latency_ms: 0, message: 'no healthCheck implemented' }));
      }
      return;
    }
    case 'poll': {
      if (typeof driver.pollRun !== 'function') {
        fail(new Error('poll not supported'));
      }
      const result = await driver.pollRun({
        baseURL: input.baseURL,
        credential: input.credential,
        poll_context: input.poll_context || {},
      });
      console.log(JSON.stringify(result));
      return;
    }
    default:
      fail(new Error(`unknown action: ${input.action}`));
  }
}

main().catch((err) => fail(err));
