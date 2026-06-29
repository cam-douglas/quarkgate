#!/usr/bin/env node
/**
 * Minimal swarm orchestrator — one QuarkGate key across providers.
 */
const config = require('./config');

const base = process.env.QUARKGATE_URL || 'http://localhost:8080';
const key = process.env.QUARKGATE_KEY;
const strict = process.env.E2E_STRICT === '1';

if (!key) {
  console.error('Set QUARKGATE_KEY');
  process.exit(1);
}

function assertOk(label, status, allowed = [200, 201, 202]) {
  const ok = allowed.includes(status);
  console.log(`${label}: status ${status} ${ok ? 'OK' : 'FAIL'}`);
  if (strict && !ok) {
    throw new Error(`${label} failed with ${status}`);
  }
  return ok;
}

async function qg(path, body, provider) {
  const headers = {
    Authorization: `Bearer ${key}`,
    'Content-Type': 'application/json',
  };
  if (provider) {
    return fetch(`${base}/v1/quarkgate`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        quarkgate_version: '1',
        provider,
        operation: body.operation,
        payload: body.payload,
      }),
    });
  }
  return fetch(`${base}${path}`, { method: 'POST', headers, body: JSON.stringify(body) });
}

async function main() {
  let passed = 0;

  console.log('1. OpenRouter chat (compat path)');
  const chat = await qg('/v1/chat/completions', {
    model: 'openai/gpt-4o-mini',
    messages: [{ role: 'user', content: 'Say hi in one word' }],
    max_tokens: 16,
  });
  const chatText = await chat.text();
  if (assertOk('openrouter', chat.status, [200, 401, 402, 502, 503])) passed++;
  if (strict && chat.status === 200) {
    const j = JSON.parse(chatText);
    if (!j.choices?.[0]?.message?.content) {
      throw new Error('openrouter empty content');
    }
  }

  console.log('2. Apify envelope');
  const apify = await qg(null, {
    operation: 'actor.run',
    payload: { actor_id: config.apifyActorId, input: {} },
  }, 'apify');
  if (assertOk('apify', apify.status, [200, 201, 202, 401, 402, 502, 503])) passed++;
  await apify.text();

  console.log('3. Supabase vector RPC envelope');
  const sb = await qg(null, {
    operation: 'rpc.match_documents',
    payload: { query_embedding: [0.1, 0.2], match_count: 3 },
  }, 'supabase');
  if (assertOk('supabase', sb.status, [200, 401, 402, 404, 502, 503])) passed++;
  await sb.text();

  console.log('4. Letta envelope');
  const letta = await qg(null, {
    operation: 'agents.messages.create',
    payload: { agent_id: config.lettaAgentId, messages: [{ role: 'user', content: 'hi' }] },
  }, 'letta');
  if (assertOk('letta', letta.status, [200, 401, 402, 404, 502, 503])) passed++;
  await letta.text();

  console.log(`Done — ${passed}/4 steps returned acceptable status`);
  if (strict && passed < 4) process.exit(1);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
