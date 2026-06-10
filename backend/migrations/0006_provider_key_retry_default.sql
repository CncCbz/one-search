UPDATE providers
SET settings = jsonb_set(COALESCE(settings, '{}'::jsonb), '{key_retry_count}', '3'::jsonb, true),
    updated_at = now()
WHERE NOT (COALESCE(settings, '{}'::jsonb) ? 'key_retry_count');
