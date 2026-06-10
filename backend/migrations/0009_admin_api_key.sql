CREATE TABLE IF NOT EXISTS admin_api_keys (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    key_hash TEXT NOT NULL UNIQUE,
    key_ciphertext TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
