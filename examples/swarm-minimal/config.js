/**
 * Provider config from environment for E2E runs.
 */
const dims = parseInt(process.env.EMBED_DIMENSIONS || '384', 10);

function stubEmbedding() {
  return Array.from({ length: dims }, (_, i) => (i === 0 ? 0.1 : 0));
}

module.exports = {
  apifyActorId: process.env.APIFY_ACTOR_ID || 'apify/hello-world',
  lettaAgentId: process.env.LETTA_AGENT_ID || 'agent-uuid',
  supabaseTable: process.env.SUPABASE_TABLE || 'documents',
  stubEmbedding,
};
