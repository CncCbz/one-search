INSERT INTO providers (name, display_name, base_url, priority, weight, timeout_ms, default_cache_enabled, cache_ttl_seconds, settings)
VALUES
    ('tavily', 'Tavily', 'https://api.tavily.com', 40, 1, 15000, FALSE, 3600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb),
    ('firecrawl', 'Firecrawl', 'https://api.firecrawl.dev', 50, 1, 30000, FALSE, 3600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb),
    ('serper', 'Serper', 'https://google.serper.dev', 60, 1, 15000, FALSE, 3600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb),
    ('brave', 'Brave Search', 'https://api.search.brave.com/res/v1', 70, 1, 15000, FALSE, 3600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb)
ON CONFLICT (name) DO NOTHING;

UPDATE settings
SET value = jsonb_set(value, '{default_providers}', '["exa","you","jina","tavily","firecrawl","serper","brave"]'::jsonb, true),
    updated_at = now()
WHERE key = 'runtime'
  AND COALESCE(value->'default_providers', '[]'::jsonb) = '["exa","you","jina"]'::jsonb;
