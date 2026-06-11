UPDATE providers
SET settings = jsonb_set(COALESCE(settings, '{}'::jsonb), '{max_concurrency}', '0'::jsonb, true),
    updated_at = now()
WHERE NOT (COALESCE(settings, '{}'::jsonb) ? 'max_concurrency');
