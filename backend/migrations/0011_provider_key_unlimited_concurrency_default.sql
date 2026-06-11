ALTER TABLE provider_keys ALTER COLUMN max_concurrency SET DEFAULT 0;

UPDATE provider_keys
SET max_concurrency = 0
WHERE max_concurrency = 1
  AND NOT EXISTS (
      SELECT 1 FROM settings WHERE key = 'migration_provider_key_unlimited_concurrency_default'
  );

INSERT INTO settings (key, value)
VALUES ('migration_provider_key_unlimited_concurrency_default', 'true'::jsonb)
ON CONFLICT (key) DO NOTHING;
