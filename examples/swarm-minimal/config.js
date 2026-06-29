/**
 * Provider config from environment for E2E runs.
 */
module.exports = {
  apifyActorId: process.env.APIFY_ACTOR_ID || 'apify/hello-world',
  lettaAgentId: process.env.LETTA_AGENT_ID || 'agent-uuid',
  supabaseTable: process.env.SUPABASE_TABLE || 'documents',
};
