#!/usr/bin/env node
/**
 * Streaming p95: QuarkGate proxy vs direct OpenRouter (TTFT proxy overhead).
 * Pass criteria: p95 delta <= 50ms (MVP sign-off).
 */
const SAMPLES = parseInt(process.env.P95_SAMPLES || '30', 10);
const orKey = process.env.OPENROUTER_API_KEY;
const qgUrl = (process.env.P95_PROXY_URL || process.env.QUARKGATE_INTERNAL_URL || process.env.QUARKGATE_URL || 'http://127.0.0.1:8090/qg').replace(/\/$/, '');
const qgKey = process.env.QUARKGATE_KEY;

if (!orKey || !qgKey) {
  console.error('Set OPENROUTER_API_KEY and QUARKGATE_KEY');
  process.exit(1);
}

const body = {
  model: 'openai/gpt-4o-mini',
  messages: [{ role: 'user', content: 'Reply with exactly: ok' }],
  max_tokens: 8,
  stream: true,
};

function p95(arr) {
  if (!arr.length) return 0;
  const s = [...arr].sort((a, b) => a - b);
  return s[Math.min(s.length - 1, Math.floor(s.length * 0.95))];
}

async function ttftDirect() {
  const t0 = performance.now();
  const resp = await fetch('https://openrouter.ai/api/v1/chat/completions', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${orKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  });
  if (!resp.ok) throw new Error(`direct HTTP ${resp.status}`);
  const reader = resp.body.getReader();
  await reader.read();
  return performance.now() - t0;
}

async function ttftProxy() {
  const t0 = performance.now();
  const resp = await fetch(`${qgUrl}/v1/chat/completions`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${qgKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  });
  if (!resp.ok) throw new Error(`proxy HTTP ${resp.status}`);
  const reader = resp.body.getReader();
  await reader.read();
  return performance.now() - t0;
}

async function main() {
  console.log('==> warmup (3 pairs, excluded from stats)');
  for (let i = 0; i < 3; i++) {
    await ttftDirect();
    await ttftProxy();
  }

  const direct = [];
  const proxy = [];
  console.log(`==> ${SAMPLES} streaming TTFT samples (direct vs QuarkGate)`);
  for (let i = 0; i < SAMPLES; i++) {
    direct.push(await ttftDirect());
    proxy.push(await ttftProxy());
    process.stdout.write(`\r  sample ${i + 1}/${SAMPLES}`);
  }
  console.log('');
  const d95 = p95(direct);
  const p95v = p95(proxy);
  const overhead = p95v - d95;
  const hw = `${process.platform} ${process.arch} node ${process.version}`;

  console.log('\n==> Results');
  console.log(`  hardware: ${hw}`);
  console.log(`  direct  p95=${d95.toFixed(1)} ms`);
  console.log(`  proxy   p95=${p95v.toFixed(1)} ms`);
  console.log(`  p95 overhead (proxy - direct): ${overhead.toFixed(1)} ms`);
  const pass = overhead <= 50;
  console.log(pass ? 'PASS p95 overhead <= 50ms' : 'FAIL p95 overhead > 50ms');
  process.exit(pass ? 0 : 1);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
