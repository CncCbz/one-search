package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func (s *Store) RuntimeSettings(ctx context.Context) (model.RuntimeSettings, error) {
	settings := model.RuntimeSettings{
		DefaultMode:         model.SearchModeParallel,
		DefaultProviders:    []string{model.ProviderExa, model.ProviderYou, model.ProviderJina},
		DefaultLimit:        10,
		DefaultDedupe:       true,
		RequestTimeoutMS:    20000,
		CacheEnabled:        false,
		CacheTTLSeconds:     3600,
		CacheMaxResults:     20,
		CompatTavilyEnabled: true,
		CompatSerperEnabled: true,
		CompatOpenAIEnabled: true,
		APIAuthRequired:     true,
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

func (s *Store) ListAvailableProviderKeys(ctx context.Context, providerName string) ([]model.APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT k.id, k.provider_id, p.name, k.alias, k.key_ciphertext, k.key_hint, k.status, k.weight,
		       k.rpm_limit, k.daily_quota, k.monthly_quota, k.max_concurrency, COALESCE(k.cooldown_until, '0001-01-01'::timestamptz)
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
		var ciphertext string
		if err := rows.Scan(&item.ID, &item.ProviderID, &item.ProviderName, &item.Alias, &ciphertext, &item.KeyHint, &item.Status, &item.Weight, &item.RPMLimit, &item.DailyQuota, &item.MonthlyQuota, &item.MaxConcurrency, &item.CooldownUntil); err != nil {
			return nil, err
		}
		plain, err := s.crypto.Decrypt(ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt provider key %s: %w", item.Alias, err)
		}
		item.Value = plain
		keys = append(keys, item)
	}
	return keys, rows.Err()
}

func (s *Store) GetAPIKeyByID(ctx context.Context, id int64) (model.APIKey, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT k.id, k.provider_id, p.name, k.alias, k.key_ciphertext, k.key_hint, k.status, k.weight,
		       k.rpm_limit, k.daily_quota, k.monthly_quota, k.max_concurrency, COALESCE(k.cooldown_until, '0001-01-01'::timestamptz)
		FROM provider_keys k
		JOIN providers p ON p.id = k.provider_id
		WHERE k.id=$1
	`, id)
	var item model.APIKey
	var ciphertext string
	if err := row.Scan(&item.ID, &item.ProviderID, &item.ProviderName, &item.Alias, &ciphertext, &item.KeyHint, &item.Status, &item.Weight, &item.RPMLimit, &item.DailyQuota, &item.MonthlyQuota, &item.MaxConcurrency, &item.CooldownUntil); err != nil {
		return model.APIKey{}, err
	}
	plain, err := s.crypto.Decrypt(ciphertext)
	if err != nil {
		return model.APIKey{}, fmt.Errorf("decrypt provider key %s: %w", item.Alias, err)
	}
	item.Value = plain
	return item, nil
}

func (s *Store) ListProviderKeys(ctx context.Context) ([]model.ProviderKeyView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT k.id, k.provider_id, p.name, k.alias, k.key_ciphertext, k.key_hint, k.status, k.weight, k.rpm_limit,
		       k.daily_quota, k.monthly_quota, k.max_concurrency, k.current_failures, k.total_successes,
		       k.total_failures,
		       COALESCE((SELECT SUM(u.requests_total) FROM usage_daily u WHERE u.provider_key_id=k.id AND u.usage_date=CURRENT_DATE), 0) AS daily_used,
		       COALESCE((SELECT SUM(u.requests_total) FROM usage_daily u WHERE u.provider_key_id=k.id AND u.usage_date >= date_trunc('month', CURRENT_DATE)::date), 0) AS monthly_used,
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
		if err := rows.Scan(&item.ID, &item.ProviderID, &item.ProviderName, &item.Alias, &ciphertext, &item.KeyHint, &item.Status, &item.Weight, &item.RPMLimit, &item.DailyQuota, &item.MonthlyQuota, &item.MaxConcurrency, &item.CurrentFailures, &item.TotalSuccesses, &item.TotalFailures, &item.DailyUsed, &item.MonthlyUsed, &item.CooldownUntil, &item.LastUsedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		plain, err := s.crypto.Decrypt(ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt provider key %s: %w", item.Alias, err)
		}
		item.Key = plain
		keys = append(keys, item)
	}
	return keys, rows.Err()
}

func (s *Store) CreateProviderKey(ctx context.Context, providerName, alias, plainKey string, weight, rpmLimit, dailyQuota, monthlyQuota, maxConcurrency int) (model.ProviderKeyView, error) {
	ciphertext, err := s.crypto.Encrypt(plainKey)
	if err != nil {
		return model.ProviderKeyView{}, err
	}
	keyHint := security.MaskSecret(plainKey)
	row := s.pool.QueryRow(ctx, `
		INSERT INTO provider_keys (provider_id, alias, key_ciphertext, key_hint, weight, rpm_limit, daily_quota, monthly_quota, max_concurrency)
		SELECT id, $2, $3, $4, $5, $6, $7, $8, $9 FROM providers WHERE name=$1
		RETURNING id
	`, providerName, alias, ciphertext, keyHint, weightOrDefault(weight), rpmLimit, dailyQuota, monthlyQuota, concurrencyOrDefault(maxConcurrency))
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

	_, err := s.pool.Exec(ctx, `
		UPDATE provider_keys
		SET alias=COALESCE($2::text, alias),
		    key_ciphertext=COALESCE($3::text, key_ciphertext),
		    key_hint=COALESCE($4::text, key_hint),
		    status=COALESCE($5::text, status),
		    weight=COALESCE($6::int, weight),
		    rpm_limit=COALESCE($7::int, rpm_limit),
		    daily_quota=COALESCE($8::int, daily_quota),
		    monthly_quota=COALESCE($9::int, monthly_quota),
		    max_concurrency=COALESCE($10::int, max_concurrency),
		    updated_at=now()
		WHERE id=$1
	`, id, stringPtrValue(patch.Alias), ciphertext, keyHint, stringPtrValue(patch.Status), intPtrValue(patch.Weight), intPtrValue(patch.RPMLimit), intPtrValue(patch.DailyQuota), intPtrValue(patch.MonthlyQuota), intPtrValue(patch.MaxConcurrency))
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
	_, err := s.pool.Exec(ctx, `DELETE FROM provider_keys WHERE id=$1`, id)
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
		SELECT id, name, token_hash, token_prefix, scopes, status, rate_limit_per_min, daily_quota, last_used_at, usage_count, created_at, updated_at
		FROM api_tokens
		WHERE token_hash=$1 AND status='enabled'
		  AND (daily_quota=0 OR COALESCE((SELECT SUM(u.requests_total) FROM usage_daily u WHERE u.api_token_id=api_tokens.id AND u.provider_id IS NULL AND u.provider_key_id IS NULL AND u.usage_date=CURRENT_DATE),0) < daily_quota)
	`, hash)
	var item model.APIToken
	if err := row.Scan(&item.ID, &item.Name, &item.TokenHash, &item.TokenPrefix, &item.Scopes, &item.Status, &item.RateLimitPerMin, &item.DailyQuota, &item.LastUsedAt, &item.UsageCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.APIToken{}, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE api_tokens SET last_used_at=now(), usage_count=usage_count+1 WHERE id=$1`, item.ID)
	return item, nil
}

func (s *Store) ListAPITokens(ctx context.Context) ([]model.APIToken, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, token_hash, COALESCE(token_ciphertext, ''), token_prefix, scopes, status, rate_limit_per_min, daily_quota, last_used_at, usage_count, created_at, updated_at
		FROM api_tokens ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.APIToken{}
	for rows.Next() {
		var item model.APIToken
		if err := rows.Scan(&item.ID, &item.Name, &item.TokenHash, &item.TokenCiphertext, &item.TokenPrefix, &item.Scopes, &item.Status, &item.RateLimitPerMin, &item.DailyQuota, &item.LastUsedAt, &item.UsageCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
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

func (s *Store) CreateAPIToken(ctx context.Context, name string, scopes []string, rateLimit, dailyQuota int) (model.APIToken, string, error) {
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
		INSERT INTO api_tokens (name, token_hash, token_ciphertext, token_prefix, scopes, rate_limit_per_min, daily_quota)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, token_hash, COALESCE(token_ciphertext, ''), token_prefix, scopes, status, rate_limit_per_min, daily_quota, last_used_at, usage_count, created_at, updated_at
	`, name, hash, tokenCiphertext, prefix, scopes, rateLimit, dailyQuota)
	var item model.APIToken
	if err := row.Scan(&item.ID, &item.Name, &item.TokenHash, &item.TokenCiphertext, &item.TokenPrefix, &item.Scopes, &item.Status, &item.RateLimitPerMin, &item.DailyQuota, &item.LastUsedAt, &item.UsageCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.APIToken{}, "", err
	}
	item.Token = rawToken
	return item, rawToken, nil
}

func (s *Store) UpdateAPITokenStatus(ctx context.Context, id int64, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE api_tokens SET status=$2, updated_at=now() WHERE id=$1`, id, status)
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
		_, err := tx.Exec(ctx, `
			INSERT INTO provider_calls (search_request_id, request_id, provider_key_id, provider_name, key_alias, status, error_type, error_message, latency_ms, result_count, cached)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, searchRequestID, input.RequestID, providerKey, call.ProviderName, call.KeyAlias, call.Status, call.ErrorType, call.ErrorMessage, int(call.LatencyMS), call.ResultCount, call.Cached)
		if err != nil {
			return err
		}
	}
	if err := upsertUsage(ctx, tx, input); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
		var providerKey interface{}
		if call.ProviderKeyID > 0 {
			providerKey = call.ProviderKeyID
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
		`, apiToken, providerKey, callSuccess, callFailed, callCacheHits, call.ResultCount, int(call.LatencyMS))
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
	row := s.pool.QueryRow(ctx, `
		SELECT id, request_id, query, mode, compat_format, providers, cache_policy, cache_hit, result_count, status, error_message, latency_ms, request_json, response_json, created_at
		FROM search_requests WHERE id=$1
	`, id)
	var item model.SearchLog
	if err := row.Scan(&item.ID, &item.RequestID, &item.Query, &item.Mode, &item.CompatFormat, &item.Providers, &item.CachePolicy, &item.CacheHit, &item.ResultCount, &item.Status, &item.ErrorMessage, &item.LatencyMS, &item.RequestJSON, &item.ResponseJSON, &item.CreatedAt); err != nil {
		return model.SearchLog{}, nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT provider_key_id, provider_name, key_alias, status, error_type, error_message, latency_ms, result_count, cached
		FROM provider_calls WHERE search_request_id=$1 ORDER BY id ASC
	`, id)
	if err != nil {
		return model.SearchLog{}, nil, err
	}
	defer rows.Close()
	calls := []model.ProviderCallLog{}
	for rows.Next() {
		var call model.ProviderCallLog
		if err := rows.Scan(&call.ProviderKeyID, &call.ProviderName, &call.KeyAlias, &call.Status, &call.ErrorType, &call.ErrorMessage, &call.LatencyMS, &call.ResultCount, &call.Cached); err != nil {
			return model.SearchLog{}, nil, err
		}
		calls = append(calls, call)
	}
	return item, calls, rows.Err()
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
