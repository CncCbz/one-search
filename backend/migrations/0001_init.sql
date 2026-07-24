CREATE TABLE IF NOT EXISTS admin_users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT ARRAY['search'],
    status TEXT NOT NULL DEFAULT 'enabled' CHECK (status IN ('enabled', 'disabled')),
    rate_limit_per_min INTEGER NOT NULL DEFAULT 0,
    daily_quota INTEGER NOT NULL DEFAULT 0,
    monthly_quota INTEGER NOT NULL DEFAULT 0,
    last_used_at TIMESTAMPTZ,
    usage_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS providers (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INTEGER NOT NULL DEFAULT 100,
    weight INTEGER NOT NULL DEFAULT 1,
    timeout_ms INTEGER NOT NULL DEFAULT 10000,
    default_cache_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    cache_ttl_seconds INTEGER NOT NULL DEFAULT 3600,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS provider_keys (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    alias TEXT NOT NULL,
    key_ciphertext TEXT NOT NULL,
    key_hint TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'enabled' CHECK (status IN ('enabled', 'disabled', 'cooling', 'exhausted')),
    weight INTEGER NOT NULL DEFAULT 1,
    rpm_limit INTEGER NOT NULL DEFAULT 0,
    daily_quota INTEGER NOT NULL DEFAULT 0,
    monthly_quota INTEGER NOT NULL DEFAULT 0,
    max_concurrency INTEGER NOT NULL DEFAULT 0,
    current_failures INTEGER NOT NULL DEFAULT 0,
    total_successes BIGINT NOT NULL DEFAULT 0,
    total_failures BIGINT NOT NULL DEFAULT 0,
    cooldown_until TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider_id, alias)
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS search_requests (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL UNIQUE,
    api_token_id BIGINT REFERENCES api_tokens(id) ON DELETE SET NULL,
    query TEXT NOT NULL,
    mode TEXT NOT NULL,
    compat_format TEXT NOT NULL DEFAULT 'native',
    providers TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    cache_policy TEXT NOT NULL DEFAULT 'default',
    cache_hit BOOLEAN NOT NULL DEFAULT FALSE,
    result_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'success' CHECK (status IN ('success', 'error')),
    error_message TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS provider_calls (
    id BIGSERIAL PRIMARY KEY,
    search_request_id BIGINT REFERENCES search_requests(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL,
    provider_id BIGINT REFERENCES providers(id) ON DELETE SET NULL,
    provider_key_id BIGINT REFERENCES provider_keys(id) ON DELETE SET NULL,
    provider_name TEXT NOT NULL,
    key_alias TEXT NOT NULL DEFAULT '',
    attempt_index INTEGER NOT NULL DEFAULT 1,
    will_retry BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'success' CHECK (status IN ('success', 'error', 'skipped')),
    error_type TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NOT NULL DEFAULT 0,
    result_count INTEGER NOT NULL DEFAULT 0,
    cached BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS search_cache (
    cache_key TEXT PRIMARY KEY,
    response_json JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    hit_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL DEFAULT 'admin',
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS provider_call_usage (
    id BIGSERIAL PRIMARY KEY,
    provider_call_id BIGINT REFERENCES provider_calls(id) ON DELETE CASCADE,
    search_request_id BIGINT REFERENCES search_requests(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL,
    api_token_id BIGINT REFERENCES api_tokens(id) ON DELETE SET NULL,
    provider_key_id BIGINT REFERENCES provider_keys(id) ON DELETE SET NULL,
    provider_name TEXT NOT NULL,
    unit TEXT NOT NULL,
    quantity NUMERIC NOT NULL DEFAULT 0,
    cost_usd NUMERIC,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS usage_daily (
    id BIGSERIAL PRIMARY KEY,
    usage_date DATE NOT NULL,
    api_token_id BIGINT REFERENCES api_tokens(id) ON DELETE SET NULL,
    provider_id BIGINT REFERENCES providers(id) ON DELETE SET NULL,
    provider_key_id BIGINT REFERENCES provider_keys(id) ON DELETE SET NULL,
    requests_total BIGINT NOT NULL DEFAULT 0,
    requests_success BIGINT NOT NULL DEFAULT 0,
    requests_failed BIGINT NOT NULL DEFAULT 0,
    cache_hits BIGINT NOT NULL DEFAULT 0,
    results_total BIGINT NOT NULL DEFAULT 0,
    latency_ms_total BIGINT NOT NULL DEFAULT 0,
    UNIQUE NULLS NOT DISTINCT (usage_date, api_token_id, provider_id, provider_key_id)
);

CREATE INDEX IF NOT EXISTS idx_provider_keys_provider_status ON provider_keys(provider_id, status);
CREATE INDEX IF NOT EXISTS idx_search_requests_created_at ON search_requests(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_provider_calls_request_id ON provider_calls(request_id);
CREATE INDEX IF NOT EXISTS idx_search_cache_expires_at ON search_cache(expires_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_provider_call_usage_request_id ON provider_call_usage(request_id);
CREATE INDEX IF NOT EXISTS idx_usage_daily_date ON usage_daily(usage_date DESC);

CREATE TABLE IF NOT EXISTS usage_meter_daily (
    id BIGSERIAL PRIMARY KEY,
    usage_date DATE NOT NULL,
    api_token_id BIGINT REFERENCES api_tokens(id) ON DELETE SET NULL,
    provider_key_id BIGINT REFERENCES provider_keys(id) ON DELETE SET NULL,
    provider_name TEXT NOT NULL DEFAULT '',
    unit TEXT NOT NULL,
    quantity_total NUMERIC NOT NULL DEFAULT 0,
    cost_usd_total NUMERIC NOT NULL DEFAULT 0,
    UNIQUE NULLS NOT DISTINCT (usage_date, api_token_id, provider_key_id, provider_name, unit)
);

CREATE INDEX IF NOT EXISTS idx_usage_meter_daily_date ON usage_meter_daily(usage_date DESC);

INSERT INTO providers (name, display_name, base_url, priority, weight, timeout_ms, default_cache_enabled, cache_ttl_seconds, settings)
VALUES
    ('exa', 'Exa', 'https://api.exa.ai', 10, 1, 12000, FALSE, 86400, '{"type":"neural","key_retry_count":3,"max_concurrency":0}'::jsonb),
    ('you', 'You.com', 'https://ydc-index.io', 20, 1, 10000, FALSE, 3600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb),
    ('jina', 'Jina', 'https://s.jina.ai', 30, 1, 15000, FALSE, 21600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb),
    ('tavily', 'Tavily', 'https://api.tavily.com', 40, 1, 15000, FALSE, 3600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb),
    ('firecrawl', 'Firecrawl', 'https://api.firecrawl.dev', 50, 1, 30000, FALSE, 3600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb),
    ('serper', 'Serper', 'https://google.serper.dev', 60, 1, 15000, FALSE, 3600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb),
    ('brave', 'Brave Search', 'https://api.search.brave.com/res/v1', 70, 1, 15000, FALSE, 3600, '{"key_retry_count":3,"max_concurrency":0}'::jsonb)
ON CONFLICT (name) DO NOTHING;

INSERT INTO settings (key, value)
VALUES
    ('runtime', '{"default_mode":"parallel","default_providers":["exa","you","jina","tavily","firecrawl","serper","brave"],"default_limit":10,"default_dedupe":true,"request_timeout_ms":20000,"cache_enabled":false,"cache_ttl_seconds":3600,"cache_max_results":20,"compat_tavily_enabled":true,"compat_serper_enabled":true,"compat_openai_enabled":true,"api_auth_required":true,"provider_health_window_minutes":15,"provider_routing_strategy":"fixed","log_retention_days":3,"search_logs_limit":100}'::jsonb)
ON CONFLICT (key) DO NOTHING;
