-- Seed MVP provider configs

INSERT INTO provider_configs (
    provider_slug, display_name, category, base_url, auth_injection, pricing_model,
    health_check_path, enabled, driver_module
) VALUES
(
    'openrouter',
    'OpenRouter',
    'llm',
    'https://openrouter.ai/api/v1',
    '{"header": "Authorization", "prefix": "Bearer ", "vault_label": "master_openrouter"}'::jsonb,
    '{"passthrough_usd": true, "platform_margin": 0.05, "base_rates_micro_per_unit": {"TOK_INPUT": 300, "TOK_OUTPUT": 900}, "minimum_charge_micro": 100}'::jsonb,
    '/models',
    true,
    'openrouter'
),
(
    'apify',
    'Apify',
    'scraper',
    'https://api.apify.com/v2',
    '{"header": "Authorization", "prefix": "Bearer ", "vault_label": "master_apify"}'::jsonb,
    '{"base_rates_micro_per_unit": {"COMPUTE_S": 50000, "API_CALL": 1000}, "minimum_charge_micro": 100}'::jsonb,
  '/acts',
    true,
    'apify'
),
(
    'letta',
    'Letta',
    'memory',
    'http://localhost:8283',
    '{"header": "Authorization", "prefix": "Bearer ", "vault_label": "master_letta"}'::jsonb,
    '{"base_rates_micro_per_unit": {"API_CALL": 1000, "TOK_INPUT": 300, "TOK_OUTPUT": 900}, "minimum_charge_micro": 100}'::jsonb,
    '/v1/health',
    true,
    'letta'
),
(
    'supabase',
    'Supabase',
    'memory',
    'http://localhost:54321',
    '{"header": "apikey", "prefix": "", "vault_label": "master_supabase"}'::jsonb,
    '{"base_rates_micro_per_unit": {"DB_READ": 10, "DB_WRITE": 50, "VEC_QUERY": 200}, "minimum_charge_micro": 10}'::jsonb,
    '/rest/v1/',
    true,
    'supabase'
);
