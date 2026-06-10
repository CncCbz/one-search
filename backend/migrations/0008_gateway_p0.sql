ALTER TABLE api_tokens ADD COLUMN IF NOT EXISTS monthly_quota INTEGER NOT NULL DEFAULT 0;

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

CREATE INDEX IF NOT EXISTS idx_api_tokens_monthly_quota ON api_tokens(monthly_quota);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_provider_call_usage_request_id ON provider_call_usage(request_id);
CREATE INDEX IF NOT EXISTS idx_usage_meter_daily_date ON usage_meter_daily(usage_date DESC);
