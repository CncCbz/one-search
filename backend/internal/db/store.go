package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/security"
)

type Store struct {
	pool   *pgxpool.Pool
	crypto *security.Crypto
}

func NewStore(pool *pgxpool.Pool, crypto *security.Crypto) *Store {
	return &Store{pool: pool, crypto: crypto}
}

func (s *Store) EnsureAdmin(ctx context.Context, username, passwordHash string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO admin_users (username, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (username) DO UPDATE SET password_hash=EXCLUDED.password_hash, updated_at=now()
	`, username, passwordHash)
	return err
}

func (s *Store) GetAdminByUsername(ctx context.Context, username string) (model.AdminUser, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, username, password_hash, created_at FROM admin_users WHERE username=$1`, username)
	var user model.AdminUser
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt); err != nil {
		return model.AdminUser{}, err
	}
	return user, nil
}

func (s *Store) GetAdminAPIKey(ctx context.Context) (model.AdminAPIKey, error) {
	row := s.pool.QueryRow(ctx, `SELECT key_prefix, created_at, updated_at FROM admin_api_keys WHERE id=TRUE`)
	var item model.AdminAPIKey
	var createdAt, updatedAt time.Time
	if err := row.Scan(&item.KeyPrefix, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.AdminAPIKey{}, nil
		}
		return model.AdminAPIKey{}, err
	}
	item.CreatedAt = &createdAt
	item.UpdatedAt = &updatedAt
	return item, nil
}

func (s *Store) RotateAdminAPIKey(ctx context.Context) (model.AdminAPIKey, string, error) {
	rawToken, err := security.RandomToken("oak_")
	if err != nil {
		return model.AdminAPIKey{}, "", err
	}
	ciphertext, err := s.crypto.Encrypt(rawToken)
	if err != nil {
		return model.AdminAPIKey{}, "", err
	}
	hash := security.HashToken(rawToken)
	prefix := security.TokenPrefix(rawToken)
	row := s.pool.QueryRow(ctx, `
		INSERT INTO admin_api_keys (id, key_hash, key_ciphertext, key_prefix)
		VALUES (TRUE, $1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET key_hash=EXCLUDED.key_hash, key_ciphertext=EXCLUDED.key_ciphertext, key_prefix=EXCLUDED.key_prefix, updated_at=now()
		RETURNING key_prefix, created_at, updated_at
	`, hash, ciphertext, prefix)
	var item model.AdminAPIKey
	var createdAt, updatedAt time.Time
	if err := row.Scan(&item.KeyPrefix, &createdAt, &updatedAt); err != nil {
		return model.AdminAPIKey{}, "", err
	}
	item.Key = rawToken
	item.CreatedAt = &createdAt
	item.UpdatedAt = &updatedAt
	return item, rawToken, nil
}

func (s *Store) FindAdminAPIKey(ctx context.Context, token string) (model.AdminAPIKey, bool, error) {
	hash := security.HashToken(token)
	row := s.pool.QueryRow(ctx, `SELECT key_prefix, created_at, updated_at FROM admin_api_keys WHERE key_hash=$1`, hash)
	var item model.AdminAPIKey
	var createdAt, updatedAt time.Time
	if err := row.Scan(&item.KeyPrefix, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.AdminAPIKey{}, false, nil
		}
		return model.AdminAPIKey{}, false, err
	}
	item.CreatedAt = &createdAt
	item.UpdatedAt = &updatedAt
	return item, true, nil
}

func (s *Store) RuntimeSettings(ctx context.Context) (model.RuntimeSettings, error) {
	settings := model.RuntimeSettings{
		DefaultMode:                 model.SearchModeParallel,
		DefaultProviders:            []string{model.ProviderExa, model.ProviderYou, model.ProviderJina},
		DefaultLimit:                10,
		DefaultDedupe:               true,
		RequestTimeoutMS:            20000,
		CacheEnabled:                false,
		CacheTTLSeconds:             3600,
		CacheMaxResults:             20,
		CompatTavilyEnabled:         true,
		CompatSerperEnabled:         true,
		CompatOpenAIEnabled:         true,
		APIAuthRequired:             true,
		ProviderHealthWindowMinutes: 15,
		ProviderRoutingStrategy:     "fixed",
		LogRetentionDays:            3,
	}
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key='runtime'`).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return settings, nil
		}
		return settings, err
	}
	if err := json.Unmarshal(payload, &settings); err != nil {
		return settings, err
	}
	return settings, nil
}

func (s *Store) UpdateRuntimeSettings(ctx context.Context, settings model.RuntimeSettings) error {
	if settings.ProviderHealthWindowMinutes <= 0 {
		settings.ProviderHealthWindowMinutes = 15
	}
	if settings.ProviderRoutingStrategy == "" {
		settings.ProviderRoutingStrategy = "fixed"
	}
	if settings.LogRetentionDays <= 0 {
		settings.LogRetentionDays = 3
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES ('runtime', $1::jsonb, now())
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()
	`, string(payload))
	return err
}

func (s *Store) ListProviders(ctx context.Context) ([]model.ProviderConfig, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.name, p.display_name, p.base_url, p.enabled, p.priority, p.weight, p.timeout_ms,
		       p.default_cache_enabled, p.cache_ttl_seconds, p.settings,
		       COUNT(k.id) FILTER (WHERE k.status='enabled' OR (k.status='cooling' AND (k.cooldown_until IS NULL OR k.cooldown_until < now()))) AS available_keys
		FROM providers p
		LEFT JOIN provider_keys k ON k.provider_id = p.id
		GROUP BY p.id
		ORDER BY p.priority ASC, p.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	providers := []model.ProviderConfig{}
	for rows.Next() {
		var item model.ProviderConfig
		var settingsBytes []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.DisplayName, &item.BaseURL, &item.Enabled, &item.Priority, &item.Weight, &item.TimeoutMS, &item.DefaultCacheEnabled, &item.CacheTTLSeconds, &settingsBytes, &item.AvailableKeys); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(settingsBytes, &item.Settings)
		providers = append(providers, item)
	}
	return providers, rows.Err()
}

func (s *Store) UpdateProvider(ctx context.Context, provider model.ProviderConfig) error {
	settingsBytes, err := json.Marshal(provider.Settings)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE providers
		SET display_name=$2, base_url=$3, enabled=$4, priority=$5, weight=$6, timeout_ms=$7,
		    default_cache_enabled=$8, cache_ttl_seconds=$9, settings=$10::jsonb, updated_at=now()
		WHERE name=$1
	`, provider.Name, provider.DisplayName, provider.BaseURL, provider.Enabled, provider.Priority, provider.Weight, provider.TimeoutMS, provider.DefaultCacheEnabled, provider.CacheTTLSeconds, string(settingsBytes))
	return err
}

func (s *Store) ProviderKeyRoutingStrategy(ctx context.Context, providerName string) (string, error) {
	var strategy string
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(settings->>'key_routing_strategy', '') FROM providers WHERE name=$1`, providerName).Scan(&strategy)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return strategy, err
}

func (s *Store) ListAvailableProviderKeys(ctx context.Context, providerName string) ([]model.APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT k.id, k.provider_id, p.name, k.alias, k.key_ciphertext, k.key_hint,
		       COALESCE(k.exa_api_key_id, ''), COALESCE(k.exa_service_key_ciphertext, ''), COALESCE(k.exa_service_key_hint, ''),
		       k.status, k.weight, k.rpm_limit, k.daily_quota, k.monthly_quota, k.max_concurrency,
		       k.total_successes, k.total_failures, COALESCE(k.last_used_at, '0001-01-01'::timestamptz), COALESCE(k.cooldown_until, '0001-01-01'::timestamptz)
		FROM provider_keys k
		JOIN providers p ON p.id = k.provider_id
		WHERE p.name=$1 AND p.enabled=TRUE
		  AND (k.status='enabled' OR (k.status='cooling' AND (k.cooldown_until IS NULL OR k.cooldown_until < now())))
		  AND (k.daily_quota=0 OR COALESCE((SELECT SUM(u.requests_total) FROM usage_daily u WHERE u.provider_key_id=k.id AND u.usage_date=CURRENT_DATE),0) < k.daily_quota)
		  AND (k.monthly_quota=0 OR COALESCE((SELECT SUM(u.requests_total) FROM usage_daily u WHERE u.provider_key_id=k.id AND u.usage_date >= date_trunc('month', CURRENT_DATE)::date),0) < k.monthly_quota)
		ORDER BY k.weight DESC, k.last_used_at NULLS FIRST, k.id ASC
	`, providerName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []model.APIKey{}
	for rows.Next() {
		var item model.APIKey
		var ciphertext, exaServiceKeyCiphertext string
		if err := rows.Scan(&item.ID, &item.ProviderID, &item.ProviderName, &item.Alias, &ciphertext, &item.KeyHint, &item.ExaAPIKeyID, &exaServiceKeyCiphertext, &item.ExaServiceKeyHint, &item.Status, &item.Weight, &item.RPMLimit, &item.DailyQuota, &item.MonthlyQuota, &item.MaxConcurrency, &item.TotalSuccesses, &item.TotalFailures, &item.LastUsedAt, &item.CooldownUntil); err != nil {
			return nil, err
		}
		plain, err := s.crypto.Decrypt(ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt provider key %s: %w", item.Alias, err)
		}
		item.Value = plain
		if exaServiceKeyCiphertext != "" {
			exaServiceKey, err := s.crypto.Decrypt(exaServiceKeyCiphertext)
			if err != nil {
				return nil, fmt.Errorf("decrypt Exa x-api-key %s: %w", item.Alias, err)
			}
			item.ExaServiceKey = exaServiceKey
		}
		keys = append(keys, item)
	}
	return keys, rows.Err()
}

func (s *Store) GetAPIKeyByID(ctx context.Context, id int64) (model.APIKey, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT k.id, k.provider_id, p.name, k.alias, k.key_ciphertext, k.key_hint,
		       COALESCE(k.exa_api_key_id, ''), COALESCE(k.exa_service_key_ciphertext, ''), COALESCE(k.exa_service_key_hint, ''),
		       k.status, k.weight, k.rpm_limit, k.daily_quota, k.monthly_quota, k.max_concurrency,
		       k.total_successes, k.total_failures, COALESCE(k.last_used_at, '0001-01-01'::timestamptz), COALESCE(k.cooldown_until, '0001-01-01'::timestamptz)
		FROM provider_keys k
		JOIN providers p ON p.id = k.provider_id
		WHERE k.id=$1
	`, id)
	var item model.APIKey
	var ciphertext, exaServiceKeyCiphertext string
	if err := row.Scan(&item.ID, &item.ProviderID, &item.ProviderName, &item.Alias, &ciphertext, &item.KeyHint, &item.ExaAPIKeyID, &exaServiceKeyCiphertext, &item.ExaServiceKeyHint, &item.Status, &item.Weight, &item.RPMLimit, &item.DailyQuota, &item.MonthlyQuota, &item.MaxConcurrency, &item.TotalSuccesses, &item.TotalFailures, &item.LastUsedAt, &item.CooldownUntil); err != nil {
		return model.APIKey{}, err
	}
	plain, err := s.crypto.Decrypt(ciphertext)
	if err != nil {
		return model.APIKey{}, fmt.Errorf("decrypt provider key %s: %w", item.Alias, err)
	}
	item.Value = plain
	if exaServiceKeyCiphertext != "" {
		exaServiceKey, err := s.crypto.Decrypt(exaServiceKeyCiphertext)
		if err != nil {
			return model.APIKey{}, fmt.Errorf("decrypt Exa x-api-key %s: %w", item.Alias, err)
		}
		item.ExaServiceKey = exaServiceKey
	}
	return item, nil
}

func (s *Store) ListProviderKeys(ctx context.Context) ([]model.ProviderKeyView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT k.id, k.provider_id, p.name, k.alias, k.key_ciphertext, k.key_hint, COALESCE(k.exa_api_key_id, ''), COALESCE(k.exa_service_key_hint, ''),
		       k.status, k.weight, k.rpm_limit, k.daily_quota, k.monthly_quota, k.max_concurrency, k.current_failures, k.total_successes,
		       k.total_failures,
		       COALESCE((SELECT SUM(u.requests_total) FROM usage_daily u WHERE u.provider_key_id=k.id AND u.usage_date=CURRENT_DATE), 0) AS daily_used,
		       COALESCE((SELECT SUM(u.requests_total) FROM usage_daily u WHERE u.provider_key_id=k.id AND u.usage_date >= date_trunc('month', CURRENT_DATE)::date), 0) AS monthly_used,
		       COALESCE(k.official_quota_status, ''), COALESCE(k.official_quota_message, ''), COALESCE(k.official_quota_unit, ''),
		       COALESCE(k.official_quota_balance, 0)::float8, k.official_quota_balance IS NOT NULL,
		       COALESCE(k.official_quota_balance_usd, 0)::float8, k.official_quota_balance_usd IS NOT NULL,
		       COALESCE(k.official_quota_used_usd, 0)::float8, k.official_quota_used_usd IS NOT NULL,
		       COALESCE(k.official_quota_total_quantity, 0)::float8, k.official_quota_total_quantity IS NOT NULL,
		       COALESCE(k.official_quota_account_id, ''), k.official_quota_checked_at,
		       k.cooldown_until, k.last_used_at, k.created_at, k.updated_at
		FROM provider_keys k
		JOIN providers p ON p.id = k.provider_id
		ORDER BY p.priority ASC, k.alias ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []model.ProviderKeyView{}
	for rows.Next() {
		var item model.ProviderKeyView
		var ciphertext string
		var balance, balanceUSD, usedUSD, totalQuantity float64
		var hasBalance, hasBalanceUSD, hasUsedUSD, hasTotalQuantity bool
		if err := rows.Scan(&item.ID, &item.ProviderID, &item.ProviderName, &item.Alias, &ciphertext, &item.KeyHint, &item.ExaAPIKeyID, &item.ExaServiceKeyHint, &item.Status, &item.Weight, &item.RPMLimit, &item.DailyQuota, &item.MonthlyQuota, &item.MaxConcurrency, &item.CurrentFailures, &item.TotalSuccesses, &item.TotalFailures, &item.DailyUsed, &item.MonthlyUsed, &item.OfficialQuotaStatus, &item.OfficialQuotaMessage, &item.OfficialQuotaUnit, &balance, &hasBalance, &balanceUSD, &hasBalanceUSD, &usedUSD, &hasUsedUSD, &totalQuantity, &hasTotalQuantity, &item.OfficialQuotaAccountID, &item.OfficialQuotaCheckedAt, &item.CooldownUntil, &item.LastUsedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		plain, err := s.crypto.Decrypt(ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt provider key %s: %w", item.Alias, err)
		}
		item.Key = plain
		if hasBalance {
			item.OfficialQuotaBalance = float64Ptr(balance)
		}
		if hasBalanceUSD {
			item.OfficialQuotaBalanceUSD = float64Ptr(balanceUSD)
		}
		if hasUsedUSD {
			item.OfficialQuotaUsedUSD = float64Ptr(usedUSD)
		}
		if hasTotalQuantity {
			item.OfficialQuotaTotalQuantity = float64Ptr(totalQuantity)
		}
		keys = append(keys, item)
	}
	return keys, rows.Err()
}

func (s *Store) CreateProviderKey(ctx context.Context, providerName, alias, plainKey, exaAPIKeyID, exaServiceKey string, weight, rpmLimit, dailyQuota, monthlyQuota, maxConcurrency int) (model.ProviderKeyView, error) {
	ciphertext, err := s.crypto.Encrypt(plainKey)
	if err != nil {
		return model.ProviderKeyView{}, err
	}
	keyHint := security.MaskSecret(plainKey)
	exaServiceKeyCiphertext := ""
	exaServiceKeyHint := ""
	if strings.TrimSpace(exaServiceKey) != "" {
		exaServiceKeyCiphertext, err = s.crypto.Encrypt(strings.TrimSpace(exaServiceKey))
		if err != nil {
			return model.ProviderKeyView{}, err
		}
		exaServiceKeyHint = security.MaskSecret(strings.TrimSpace(exaServiceKey))
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO provider_keys (provider_id, alias, key_ciphertext, key_hint, exa_api_key_id, exa_service_key_ciphertext, exa_service_key_hint, weight, rpm_limit, daily_quota, monthly_quota, max_concurrency)
		SELECT id, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12 FROM providers WHERE name=$1
		RETURNING id
	`, providerName, alias, ciphertext, keyHint, strings.TrimSpace(exaAPIKeyID), exaServiceKeyCiphertext, exaServiceKeyHint, weightOrDefault(weight), rpmLimit, dailyQuota, monthlyQuota, concurrencyOrDefault(maxConcurrency))
	var id int64
	if err := row.Scan(&id); err != nil {
		return model.ProviderKeyView{}, err
	}
	keys, err := s.ListProviderKeys(ctx)
	if err != nil {
		return model.ProviderKeyView{}, err
	}
	for _, item := range keys {
		if item.ID == id {
			return item, nil
		}
	}
	return model.ProviderKeyView{}, pgx.ErrNoRows
}

func (s *Store) UpdateProviderKeyStatus(ctx context.Context, id int64, status string) error {
	_, err := s.UpdateProviderKey(ctx, id, model.ProviderKeyUpdate{Status: &status})
	return err
}

func (s *Store) UpdateProviderKey(ctx context.Context, id int64, patch model.ProviderKeyUpdate) (model.ProviderKeyView, error) {
	var ciphertext interface{}
	var keyHint interface{}
	if patch.Key != nil && *patch.Key != "" {
		crypted, err := s.crypto.Encrypt(*patch.Key)
		if err != nil {
			return model.ProviderKeyView{}, err
		}
		ciphertext = crypted
		keyHint = security.MaskSecret(*patch.Key)
	}
	var exaAPIKeyID interface{}
	if patch.ExaAPIKeyID != nil && strings.TrimSpace(*patch.ExaAPIKeyID) != "" {
		exaAPIKeyID = strings.TrimSpace(*patch.ExaAPIKeyID)
	}
	var exaServiceKeyCiphertext interface{}
	var exaServiceKeyHint interface{}
	if patch.ExaServiceKey != nil && strings.TrimSpace(*patch.ExaServiceKey) != "" {
		crypted, err := s.crypto.Encrypt(strings.TrimSpace(*patch.ExaServiceKey))
		if err != nil {
			return model.ProviderKeyView{}, err
		}
		exaServiceKeyCiphertext = crypted
		exaServiceKeyHint = security.MaskSecret(strings.TrimSpace(*patch.ExaServiceKey))
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE provider_keys
		SET alias=COALESCE($2::text, alias),
		    key_ciphertext=COALESCE($3::text, key_ciphertext),
		    key_hint=COALESCE($4::text, key_hint),
		    exa_api_key_id=COALESCE($5::text, exa_api_key_id),
		    exa_service_key_ciphertext=COALESCE($6::text, exa_service_key_ciphertext),
		    exa_service_key_hint=COALESCE($7::text, exa_service_key_hint),
		    status=COALESCE($8::text, status),
		    weight=COALESCE($9::int, weight),
		    rpm_limit=COALESCE($10::int, rpm_limit),
		    daily_quota=COALESCE($11::int, daily_quota),
		    monthly_quota=COALESCE($12::int, monthly_quota),
		    max_concurrency=COALESCE($13::int, max_concurrency),
		    updated_at=now()
		WHERE id=$1
	`, id, stringPtrValue(patch.Alias), ciphertext, keyHint, exaAPIKeyID, exaServiceKeyCiphertext, exaServiceKeyHint, stringPtrValue(patch.Status), intPtrValue(patch.Weight), intPtrValue(patch.RPMLimit), intPtrValue(patch.DailyQuota), intPtrValue(patch.MonthlyQuota), intPtrValue(patch.MaxConcurrency))
	if err != nil {
		return model.ProviderKeyView{}, err
	}
	return s.GetProviderKey(ctx, id)
}

func (s *Store) GetProviderKey(ctx context.Context, id int64) (model.ProviderKeyView, error) {
	keys, err := s.ListProviderKeys(ctx)
	if err != nil {
		return model.ProviderKeyView{}, err
	}
	for _, item := range keys {
		if item.ID == id {
			return item, nil
		}
	}
	return model.ProviderKeyView{}, pgx.ErrNoRows
}

func (s *Store) DeleteProviderKey(ctx context.Context, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM usage_daily WHERE provider_key_id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM provider_keys WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateProviderKeyOfficialQuota(ctx context.Context, id int64, quota model.ProviderKeyQuotaResult) error {
	message := quota.Message
	if message == "" && quota.Status == "error" {
		message = "official quota query failed"
	}
	exhausted := quota.Status == "success" && (quota.Provider == model.ProviderJina || quota.Provider == model.ProviderYou) && quota.Balance != nil && *quota.Balance <= 0
	_, err := s.pool.Exec(ctx, `
		UPDATE provider_keys
		SET official_quota_status=$2,
		    official_quota_message=$3,
		    official_quota_unit=$4,
		    official_quota_balance=$5,
		    official_quota_balance_usd=$6,
		    official_quota_used_usd=$7,
		    official_quota_total_quantity=$8,
		    official_quota_account_id=$9,
		    official_quota_checked_at=$10,
		    status=CASE WHEN $11 THEN 'exhausted' ELSE status END,
		    updated_at=now()
		WHERE id=$1
	`, id, quota.Status, message, quota.Unit, floatPtrValue(quota.Balance), floatPtrValue(quota.BalanceUSD), floatPtrValue(quota.TotalCostUSD), floatPtrValue(quota.TotalQuantity), quota.AccountID, quota.FetchedAt, exhausted)
	return err
}

func (s *Store) RecordKeyResult(ctx context.Context, key model.APIKey, success bool, errorType string) error {
	status := key.Status
	var cooldown *time.Time
	if success {
		status = "enabled"
	} else {
		switch errorType {
		case "auth":
			status = "disabled"
		case "quota_exhausted":
			status = "exhausted"
		case "rate_limited":
			status = "cooling"
			until := time.Now().Add(15 * time.Minute)
			cooldown = &until
		default:
			status = "enabled"
		}
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE provider_keys
		SET status=$2,
		    current_failures=CASE WHEN $3 THEN 0 ELSE current_failures + 1 END,
		    total_successes=CASE WHEN $3 THEN total_successes + 1 ELSE total_successes END,
		    total_failures=CASE WHEN $3 THEN total_failures ELSE total_failures + 1 END,
		    cooldown_until=$4,
		    last_used_at=now(),
		    updated_at=now()
		WHERE id=$1
	`, key.ID, status, success, cooldown)
	return err
}

func (s *Store) FindAPIToken(ctx context.Context, token string) (model.APIToken, error) {
	hash := security.HashToken(token)
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, token_hash, token_prefix, scopes, allowed_providers, status, rate_limit_per_min, daily_quota, monthly_quota, last_used_at, usage_count, created_at, updated_at
		FROM api_tokens
		WHERE token_hash=$1 AND status='enabled'
		  AND (daily_quota=0 OR COALESCE((SELECT SUM(u.requests_total) FROM usage_daily u WHERE u.api_token_id=api_tokens.id AND u.provider_id IS NULL AND u.provider_key_id IS NULL AND u.usage_date=CURRENT_DATE),0) < daily_quota)
		  AND (monthly_quota=0 OR COALESCE((SELECT SUM(u.requests_total) FROM usage_daily u WHERE u.api_token_id=api_tokens.id AND u.provider_id IS NULL AND u.provider_key_id IS NULL AND u.usage_date >= date_trunc('month', CURRENT_DATE)::date),0) < monthly_quota)
	`, hash)
	var item model.APIToken
	if err := row.Scan(&item.ID, &item.Name, &item.TokenHash, &item.TokenPrefix, &item.Scopes, &item.AllowedProviders, &item.Status, &item.RateLimitPerMin, &item.DailyQuota, &item.MonthlyQuota, &item.LastUsedAt, &item.UsageCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.APIToken{}, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE api_tokens SET last_used_at=now(), usage_count=usage_count+1 WHERE id=$1`, item.ID)
	return item, nil
}

func (s *Store) ListAPITokens(ctx context.Context) ([]model.APIToken, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, token_hash, COALESCE(token_ciphertext, ''), token_prefix, scopes, allowed_providers, status, rate_limit_per_min, daily_quota, monthly_quota, last_used_at, usage_count, created_at, updated_at
		FROM api_tokens ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.APIToken{}
	for rows.Next() {
		var item model.APIToken
		if err := rows.Scan(&item.ID, &item.Name, &item.TokenHash, &item.TokenCiphertext, &item.TokenPrefix, &item.Scopes, &item.AllowedProviders, &item.Status, &item.RateLimitPerMin, &item.DailyQuota, &item.MonthlyQuota, &item.LastUsedAt, &item.UsageCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if item.TokenCiphertext != "" {
			plain, err := s.crypto.Decrypt(item.TokenCiphertext)
			if err != nil {
				return nil, fmt.Errorf("decrypt api token %s: %w", item.Name, err)
			}
			item.Token = plain
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateAPIToken(ctx context.Context, name string, scopes []string, allowedProviders []string, rateLimit, dailyQuota, monthlyQuota int) (model.APIToken, string, error) {
	rawToken, err := security.RandomToken("osr_")
	if err != nil {
		return model.APIToken{}, "", err
	}
	if len(scopes) == 0 {
		scopes = []string{"search"}
	}
	tokenCiphertext, err := s.crypto.Encrypt(rawToken)
	if err != nil {
		return model.APIToken{}, "", err
	}
	hash := security.HashToken(rawToken)
	prefix := security.TokenPrefix(rawToken)
	row := s.pool.QueryRow(ctx, `
		INSERT INTO api_tokens (name, token_hash, token_ciphertext, token_prefix, scopes, allowed_providers, rate_limit_per_min, daily_quota, monthly_quota)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, name, token_hash, COALESCE(token_ciphertext, ''), token_prefix, scopes, allowed_providers, status, rate_limit_per_min, daily_quota, monthly_quota, last_used_at, usage_count, created_at, updated_at
	`, name, hash, tokenCiphertext, prefix, scopes, allowedProviders, rateLimit, dailyQuota, monthlyQuota)
	var item model.APIToken
	if err := row.Scan(&item.ID, &item.Name, &item.TokenHash, &item.TokenCiphertext, &item.TokenPrefix, &item.Scopes, &item.AllowedProviders, &item.Status, &item.RateLimitPerMin, &item.DailyQuota, &item.MonthlyQuota, &item.LastUsedAt, &item.UsageCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.APIToken{}, "", err
	}
	item.Token = rawToken
	return item, rawToken, nil
}

func (s *Store) UpdateAPITokenStatus(ctx context.Context, id int64, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE api_tokens SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	return err
}

func (s *Store) UpdateAPIToken(ctx context.Context, id int64, name string, allowedProviders []string, rateLimit, dailyQuota, monthlyQuota int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_tokens
		SET name=$2, allowed_providers=$3, rate_limit_per_min=$4, daily_quota=$5, monthly_quota=$6, updated_at=now()
		WHERE id=$1
	`, id, name, allowedProviders, rateLimit, dailyQuota, monthlyQuota)
	return err
}

func (s *Store) DeleteAPIToken(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM api_tokens WHERE id=$1`, id)
	return err
}

func (s *Store) RecordSearchLog(ctx context.Context, input model.SearchLogInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var searchRequestID int64
	var apiToken interface{}
	if input.APITokenID > 0 {
		apiToken = input.APITokenID
	}
	requestJSON := input.RequestJSON
	if len(requestJSON) == 0 {
		requestJSON = []byte("{}")
	}
	responseJSON := input.ResponseJSON
	if len(responseJSON) == 0 {
		responseJSON = []byte("{}")
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO search_requests (request_id, api_token_id, query, mode, compat_format, providers, cache_policy, cache_hit, result_count, status, error_message, latency_ms, request_json, response_json)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14::jsonb)
		RETURNING id
	`, input.RequestID, apiToken, input.Query, input.Mode, input.CompatFormat, input.Providers, input.CachePolicy, input.CacheHit, input.ResultCount, input.Status, input.ErrorMessage, int(input.LatencyMS), string(requestJSON), string(responseJSON)).Scan(&searchRequestID); err != nil {
		return err
	}
	for _, call := range input.Calls {
		var providerKey interface{}
		if call.ProviderKeyID > 0 {
			providerKey = call.ProviderKeyID
		}
		attemptIndex := call.AttemptIndex
		if attemptIndex <= 0 {
			attemptIndex = 1
		}
		var providerCallID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO provider_calls (search_request_id, request_id, provider_key_id, provider_name, key_alias, attempt_index, will_retry, status, error_type, error_message, latency_ms, result_count, cached)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			RETURNING id
		`, searchRequestID, input.RequestID, providerKey, call.ProviderName, call.KeyAlias, attemptIndex, call.WillRetry, call.Status, call.ErrorType, call.ErrorMessage, int(call.LatencyMS), call.ResultCount, call.Cached).Scan(&providerCallID)
		if err != nil {
			return err
		}
		if err := insertCallUsage(ctx, tx, searchRequestID, input, providerCallID, call); err != nil {
			return err
		}
	}
	if err := upsertUsage(ctx, tx, input); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertCallUsage(ctx context.Context, tx pgx.Tx, searchRequestID int64, input model.SearchLogInput, providerCallID int64, call model.ProviderCallLog) error {
	measurements := append([]model.UsageMeasurement{{Unit: "requests", Quantity: 1}}, call.Usage...)
	var apiToken interface{}
	if input.APITokenID > 0 {
		apiToken = input.APITokenID
	}
	var providerKey interface{}
	if call.ProviderKeyID > 0 {
		providerKey = call.ProviderKeyID
	}
	for _, measurement := range measurements {
		unit := strings.TrimSpace(strings.ToLower(measurement.Unit))
		if unit == "" || measurement.Quantity == 0 {
			continue
		}
		metadata := measurement.Metadata
		if metadata == nil {
			metadata = map[string]interface{}{}
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		var costUSD interface{}
		costTotal := 0.0
		if measurement.CostUSD != nil {
			costUSD = *measurement.CostUSD
			costTotal = *measurement.CostUSD
		}
		if unit == "usd" && costTotal == 0 {
			costTotal = measurement.Quantity
			costUSD = measurement.Quantity
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO provider_call_usage (provider_call_id, search_request_id, request_id, api_token_id, provider_key_id, provider_name, unit, quantity, cost_usd, metadata)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)
		`, providerCallID, searchRequestID, input.RequestID, apiToken, providerKey, call.ProviderName, unit, measurement.Quantity, costUSD, string(metadataJSON))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO usage_meter_daily (usage_date, api_token_id, provider_key_id, provider_name, unit, quantity_total, cost_usd_total)
			VALUES (CURRENT_DATE, $1, $2, $3, $4, $5, $6)
			ON CONFLICT (usage_date, api_token_id, provider_key_id, provider_name, unit) DO UPDATE SET
			quantity_total=usage_meter_daily.quantity_total+$5,
			cost_usd_total=usage_meter_daily.cost_usd_total+$6
		`, apiToken, providerKey, call.ProviderName, unit, measurement.Quantity, costTotal)
		if err != nil {
			return err
		}
	}
	return nil
}

func upsertUsage(ctx context.Context, tx pgx.Tx, input model.SearchLogInput) error {
	requestsSuccess := 0
	requestsFailed := 0
	if input.Status == "success" {
		requestsSuccess = 1
	} else {
		requestsFailed = 1
	}
	cacheHits := 0
	if input.CacheHit {
		cacheHits = 1
	}
	var apiToken interface{}
	if input.APITokenID > 0 {
		apiToken = input.APITokenID
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO usage_daily (usage_date, api_token_id, requests_total, requests_success, requests_failed, cache_hits, results_total, latency_ms_total)
		VALUES (CURRENT_DATE, $1, 1, $2, $3, $4, $5, $6)
		ON CONFLICT (usage_date, api_token_id, provider_id, provider_key_id) DO UPDATE SET
		requests_total=usage_daily.requests_total+1,
		requests_success=usage_daily.requests_success+$2,
		requests_failed=usage_daily.requests_failed+$3,
		cache_hits=usage_daily.cache_hits+$4,
		results_total=usage_daily.results_total+$5,
		latency_ms_total=usage_daily.latency_ms_total+$6
	`, apiToken, requestsSuccess, requestsFailed, cacheHits, input.ResultCount, int(input.LatencyMS))
	if err != nil {
		return err
	}
	for _, call := range input.Calls {
		if call.ProviderKeyID <= 0 {
			continue
		}
		callSuccess := 0
		callFailed := 0
		if call.Status == "success" {
			callSuccess = 1
		} else if call.Status == "error" {
			callFailed = 1
		}
		callCacheHits := 0
		if call.Cached {
			callCacheHits = 1
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO usage_daily (usage_date, api_token_id, provider_key_id, requests_total, requests_success, requests_failed, cache_hits, results_total, latency_ms_total)
			VALUES (CURRENT_DATE, $1, $2, 1, $3, $4, $5, $6, $7)
			ON CONFLICT (usage_date, api_token_id, provider_id, provider_key_id) DO UPDATE SET
			requests_total=usage_daily.requests_total+1,
			requests_success=usage_daily.requests_success+$3,
			requests_failed=usage_daily.requests_failed+$4,
			cache_hits=usage_daily.cache_hits+$5,
			results_total=usage_daily.results_total+$6,
			latency_ms_total=usage_daily.latency_ms_total+$7
		`, apiToken, call.ProviderKeyID, callSuccess, callFailed, callCacheHits, call.ResultCount, int(call.LatencyMS))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListSearchLogs(ctx context.Context, limit int) ([]model.SearchLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, request_id, query, mode, compat_format, providers, cache_policy, cache_hit, result_count, status, error_message, latency_ms, created_at
		FROM search_requests ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.SearchLog{}
	for rows.Next() {
		var item model.SearchLog
		if err := rows.Scan(&item.ID, &item.RequestID, &item.Query, &item.Mode, &item.CompatFormat, &item.Providers, &item.CachePolicy, &item.CacheHit, &item.ResultCount, &item.Status, &item.ErrorMessage, &item.LatencyMS, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetSearchLog(ctx context.Context, id int64) (model.SearchLog, []model.ProviderCallLog, error) {
	return s.getSearchLog(ctx, "id", id)
}

func (s *Store) GetSearchLogByRequestID(ctx context.Context, requestID string) (model.SearchLog, []model.ProviderCallLog, error) {
	return s.getSearchLog(ctx, "request_id", requestID)
}

func (s *Store) getSearchLog(ctx context.Context, field string, value interface{}) (model.SearchLog, []model.ProviderCallLog, error) {
	row := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, request_id, query, mode, compat_format, providers, cache_policy, cache_hit, result_count, status, error_message, latency_ms, request_json, response_json, created_at
		FROM search_requests WHERE %s=$1
	`, field), value)
	var item model.SearchLog
	if err := row.Scan(&item.ID, &item.RequestID, &item.Query, &item.Mode, &item.CompatFormat, &item.Providers, &item.CachePolicy, &item.CacheHit, &item.ResultCount, &item.Status, &item.ErrorMessage, &item.LatencyMS, &item.RequestJSON, &item.ResponseJSON, &item.CreatedAt); err != nil {
		return model.SearchLog{}, nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(provider_key_id, 0), provider_name, key_alias, attempt_index, will_retry, status, error_type, error_message, latency_ms, result_count, cached
		FROM provider_calls WHERE search_request_id=$1 ORDER BY id ASC
	`, item.ID)
	if err != nil {
		return model.SearchLog{}, nil, err
	}
	defer rows.Close()
	calls := []model.ProviderCallLog{}
	callIndexes := map[int64]int{}
	for rows.Next() {
		var call model.ProviderCallLog
		if err := rows.Scan(&call.ID, &call.ProviderKeyID, &call.ProviderName, &call.KeyAlias, &call.AttemptIndex, &call.WillRetry, &call.Status, &call.ErrorType, &call.ErrorMessage, &call.LatencyMS, &call.ResultCount, &call.Cached); err != nil {
			return model.SearchLog{}, nil, err
		}
		callIndexes[call.ID] = len(calls)
		calls = append(calls, call)
	}
	if err := rows.Err(); err != nil {
		return model.SearchLog{}, nil, err
	}
	usageRows, err := s.pool.Query(ctx, `
		SELECT provider_call_id, unit, quantity::float8, COALESCE(cost_usd, 0)::float8, cost_usd IS NOT NULL, metadata
		FROM provider_call_usage WHERE search_request_id=$1 ORDER BY id ASC
	`, item.ID)
	if err != nil {
		return model.SearchLog{}, nil, err
	}
	defer usageRows.Close()
	for usageRows.Next() {
		var callID int64
		var usage model.UsageMeasurement
		var costUSD float64
		var hasCost bool
		var metadataBytes []byte
		if err := usageRows.Scan(&callID, &usage.Unit, &usage.Quantity, &costUSD, &hasCost, &metadataBytes); err != nil {
			return model.SearchLog{}, nil, err
		}
		if hasCost {
			usage.CostUSD = float64Ptr(costUSD)
		}
		_ = json.Unmarshal(metadataBytes, &usage.Metadata)
		if index, ok := callIndexes[callID]; ok {
			calls[index].Usage = append(calls[index].Usage, usage)
		}
	}
	return item, calls, usageRows.Err()
}

func (s *Store) UsageSummary(ctx context.Context) (model.UsageSummary, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(requests_total),0), COALESCE(SUM(requests_success),0), COALESCE(SUM(requests_failed),0),
		       COALESCE(SUM(cache_hits),0), COALESCE(SUM(results_total),0), COALESCE(SUM(latency_ms_total),0)
		FROM usage_daily WHERE provider_id IS NULL AND provider_key_id IS NULL
	`)
	var summary model.UsageSummary
	var latencyTotal int64
	if err := row.Scan(&summary.RequestsTotal, &summary.RequestsSuccess, &summary.RequestsFailed, &summary.CacheHits, &summary.ResultsTotal, &latencyTotal); err != nil {
		return summary, err
	}
	if summary.RequestsTotal > 0 {
		summary.AverageLatency = float64(latencyTotal) / float64(summary.RequestsTotal)
	}
	return summary, nil
}

func (s *Store) BillingSummary(ctx context.Context, days int) (model.BillingSummary, error) {
	if days <= 0 || days > 366 {
		days = 30
	}
	rows, err := s.pool.Query(ctx, `
		SELECT provider_name, unit, COALESCE(SUM(quantity_total),0)::float8, COALESCE(SUM(cost_usd_total),0)::float8
		FROM usage_meter_daily
		WHERE usage_date >= CURRENT_DATE - ($1::int - 1)
		GROUP BY provider_name, unit
		ORDER BY provider_name ASC, unit ASC
	`, days)
	if err != nil {
		return model.BillingSummary{}, err
	}
	defer rows.Close()
	summary := model.BillingSummary{Days: days, Units: []model.UsageUnitSummary{}}
	for rows.Next() {
		var item model.UsageUnitSummary
		if err := rows.Scan(&item.ProviderName, &item.Unit, &item.QuantityTotal, &item.CostUSDTotal); err != nil {
			return summary, err
		}
		summary.Units = append(summary.Units, item)
	}
	return summary, rows.Err()
}

func (s *Store) ProviderHealth(ctx context.Context, windowMinutes int) ([]model.ProviderHealth, error) {
	if windowMinutes <= 0 || windowMinutes > 24*60 {
		windowMinutes = 15
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.name, p.display_name, p.enabled,
		       COUNT(k.id)::int,
		       COUNT(k.id) FILTER (WHERE k.status='enabled' OR (k.status='cooling' AND (k.cooldown_until IS NULL OR k.cooldown_until < now())))::int,
		       COUNT(k.id) FILTER (WHERE k.status='exhausted')::int,
		       COUNT(k.id) FILTER (WHERE k.status='disabled')::int,
		       COUNT(k.id) FILTER (WHERE k.status='cooling')::int
		FROM providers p
		LEFT JOIN provider_keys k ON k.provider_id=p.id
		GROUP BY p.id
		ORDER BY p.priority ASC, p.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.ProviderHealth{}
	for rows.Next() {
		var item model.ProviderHealth
		if err := rows.Scan(&item.ProviderName, &item.DisplayName, &item.Enabled, &item.TotalKeys, &item.AvailableKeys, &item.ExhaustedKeys, &item.DisabledKeys, &item.CoolingKeys); err != nil {
			return nil, err
		}
		item.WindowMinutes = windowMinutes
		item.LastCheckedAt = time.Now()
		item.Status = providerHealthStatus(item)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range items {
		stats, err := s.providerRecentStats(ctx, items[index].ProviderName, windowMinutes)
		if err != nil {
			return nil, err
		}
		items[index].RequestsTotal = stats.requestsTotal
		items[index].RequestsFailed = stats.requestsFailed
		items[index].LastError = stats.lastError
		if stats.requestsTotal > 0 {
			items[index].SuccessRate = float64(stats.requestsTotal-stats.requestsFailed) / float64(stats.requestsTotal)
		}
		if items[index].Status == "healthy" && stats.requestsTotal >= 5 && items[index].SuccessRate < 0.8 {
			items[index].Status = "degraded"
		}
	}
	return items, nil
}

type providerRecentStats struct {
	requestsTotal  int64
	requestsFailed int64
	lastError      string
}

func (s *Store) providerRecentStats(ctx context.Context, providerName string, windowMinutes int) (providerRecentStats, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)::bigint,
		       COUNT(*) FILTER (WHERE status='error')::bigint,
		       COALESCE((ARRAY_AGG(error_message ORDER BY created_at DESC) FILTER (WHERE error_message <> ''))[1], '')
		FROM provider_calls
		WHERE provider_name=$1 AND created_at >= now() - make_interval(mins => $2)
	`, providerName, windowMinutes)
	var stats providerRecentStats
	if err := row.Scan(&stats.requestsTotal, &stats.requestsFailed, &stats.lastError); err != nil {
		return stats, err
	}
	return stats, nil
}

func providerHealthStatus(item model.ProviderHealth) string {
	if !item.Enabled {
		return "disabled"
	}
	if item.TotalKeys == 0 {
		return "no_keys"
	}
	if item.AvailableKeys == 0 {
		return "down"
	}
	if item.ExhaustedKeys > 0 || item.CoolingKeys > 0 {
		return "degraded"
	}
	return "healthy"
}

func (s *Store) RecordAuditLog(ctx context.Context, input model.AuditLogInput) error {
	actor := strings.TrimSpace(input.Actor)
	if actor == "" {
		actor = "admin"
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_logs (request_id, actor, action, resource_type, resource_id, ip_address, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)
	`, input.RequestID, actor, input.Action, input.ResourceType, input.ResourceID, input.IPAddress, string(payload))
	return err
}

func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]model.AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, request_id, actor, action, resource_type, resource_id, ip_address, metadata, created_at
		FROM audit_logs ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.AuditLog{}
	for rows.Next() {
		var item model.AuditLog
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.RequestID, &item.Actor, &item.Action, &item.ResourceType, &item.ResourceID, &item.IPAddress, &metadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetCache(ctx context.Context, cacheKey string) ([]byte, bool, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `
		UPDATE search_cache SET hit_count=hit_count+1, updated_at=now()
		WHERE cache_key=$1 AND expires_at > now()
		RETURNING response_json
	`, cacheKey).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return payload, true, nil
}

func (s *Store) SetCache(ctx context.Context, cacheKey string, payload []byte, ttlSeconds int) error {
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO search_cache (cache_key, response_json, expires_at)
		VALUES ($1,$2::jsonb,$3)
		ON CONFLICT (cache_key) DO UPDATE SET response_json=EXCLUDED.response_json, expires_at=EXCLUDED.expires_at, updated_at=now()
	`, cacheKey, string(payload), expiresAt)
	return err
}

func (s *Store) DeleteExpiredCache(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM search_cache WHERE expires_at <= now()`)
	return err
}

func (s *Store) DeleteOldLogs(ctx context.Context, retentionDays int) (int64, int64, error) {
	if retentionDays <= 0 {
		retentionDays = 3
	}
	searchResult, err := s.pool.Exec(ctx, `DELETE FROM search_requests WHERE created_at < now() - make_interval(days => $1)`, retentionDays)
	if err != nil {
		return 0, 0, err
	}
	auditResult, err := s.pool.Exec(ctx, `DELETE FROM audit_logs WHERE created_at < now() - make_interval(days => $1)`, retentionDays)
	if err != nil {
		return searchResult.RowsAffected(), 0, err
	}
	return searchResult.RowsAffected(), auditResult.RowsAffected(), nil
}

func weightOrDefault(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func concurrencyOrDefault(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func stringPtrValue(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func intPtrValue(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func floatPtrValue(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func float64Ptr(value float64) *float64 {
	return &value
}
