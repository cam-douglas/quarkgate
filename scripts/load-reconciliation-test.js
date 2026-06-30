#!/usr/bin/env node
/**
 * 1000-request reconciliation load test (QuarkGate MVP sign-off).
 * Uses cheap Supabase rpc.match_documents via /v1/quarkgate (not OpenRouter).
 */
const { spawnSync } = require('child_process');
const config = require('../examples/swarm-minimal/config');

const N = parseInt(process.env.LOAD_N || '1000', 10);
const BATCH = parseInt(process.env.LOAD_BATCH || '20', 10);
const WAIT_MAX_SEC = parseInt(process.env.LOAD_WAIT_SEC || '300', 10);
const base = (process.env.QUARKGATE_URL || 'http://127.0.0.1:8090/qg').replace(/\/$/, '');
const key = process.env.QUARKGATE_KEY;
const userId = process.env.QUARKGATE_USER_ID || '';

if (!key) {
  console.error('Set QUARKGATE_KEY');
  process.exit(1);
}

async function oneRequest(i) {
  const resp = await fetch(`${base}/v1/quarkgate`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${key}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      quarkgate_version: '1',
      provider: 'supabase',
      operation: 'rpc.match_documents',
      payload: { query_embedding: config.stubEmbedding(), match_count: 1 },
    }),
  });
  return resp.status;
}

function psql(sql) {
  const r = spawnSync('psql', [process.env.DATABASE_URL, '-t', '-A', '-c', sql], {
    encoding: 'utf8',
    env: process.env,
  });
  if (r.status !== 0) throw new Error(r.stderr || 'psql failed');
  return (r.stdout || '').trim();
}

function reconcileUser(uid) {
  const r = spawnSync('go', ['run', './cmd/admin', 'reconcile-user', uid], {
    cwd: require('path').join(__dirname, '..'),
    encoding: 'utf8',
    env: process.env,
  });
  return { code: r.status, out: (r.stdout || '') + (r.stderr || '') };
}

async function main() {
  console.log(`==> ${N} metered requests (supabase rpc.match_documents)`);
  let ok = 0;
  let fail = 0;
  const batch = BATCH;
  for (let i = 0; i < N; i += batch) {
    const slice = Math.min(batch, N - i);
    const results = await Promise.all(Array.from({ length: slice }, (_, j) => oneRequest(i + j)));
    for (const s of results) {
      if (s === 200) ok++;
      else fail++;
    }
    process.stdout.write(`\r  progress ${i + slice}/${N} ok=${ok} fail=${fail}`);
    await new Promise((r) => setTimeout(r, 50));
  }
  console.log('');

  console.log(`==> wait for ledger worker (up to ${WAIT_MAX_SEC}s)`);
  for (let w = 0; w < WAIT_MAX_SEC; w++) {
    const pending = psql("SELECT COUNT(*) FROM usage_logs WHERE status = 'pending';");
    if (pending === '0') break;
    if (w % 10 === 0) process.stdout.write(`\r  pending=${pending} (${w}s)`);
    await new Promise((r) => setTimeout(r, 1000));
  }
  const pending = psql("SELECT COUNT(*) FROM usage_logs WHERE status = 'pending';");
  console.log(`  pending usage_logs: ${pending}`);

  const uid = userId || psql('SELECT id FROM users LIMIT 1;');
  console.log(`==> reconcile-user ${uid}`);
  const rec = reconcileUser(uid);
  const recOut = rec.out.trim();
  console.log(recOut);
  const noDrift = recOut.includes('no drift');

  const cached = psql(`SELECT credit_balance_micro FROM users WHERE id = '${uid}';`);

  console.log('\n==> Summary');
  console.log(`  requests: ${N} ok=${ok} fail=${fail}`);
  console.log(`  credit_balance_micro: ${cached}`);
  console.log(`  reconcile: ${recOut}`);
  const pass = fail === 0 && pending === '0' && noDrift;
  console.log(pass ? 'PASS reconciliation load test' : 'FAIL reconciliation load test');
  process.exit(pass ? 0 : 1);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
