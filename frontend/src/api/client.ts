import { useSessionStore } from '../stores/session'

const API_BASE = import.meta.env.VITE_API_BASE || ''

export interface UsageSummary {
  requests_total: number
  requests_success: number
  requests_failed: number
  cache_hits: number
  results_total: number
  average_latency_ms: number
}

export interface ProviderConfig {
  id: number
  name: string
  display_name: string
  base_url: string
  enabled: boolean
  priority: number
  weight: number
  timeout_ms: number
  default_cache_enabled: boolean
  cache_ttl_seconds: number
  settings?: Record<string, unknown>
  available_keys?: number
}

export interface ProviderKey {
  id: number
  provider_id: number
  provider_name: string
  alias: string
  key_hint: string
  key: string
  status: string
  weight: number
  rpm_limit: number
  daily_quota: number
  monthly_quota: number
  max_concurrency: number
  current_failures: number
  total_successes: number
  total_failures: number
  daily_used: number
  monthly_used: number
  cooldown_until?: string
  last_used_at?: string
  created_at: string
  updated_at: string
}

export interface ApiToken {
  id: number
  name: string
  token_prefix: string
  token: string
  scopes: string[]
  allowed_providers: string[]
  status: string
  rate_limit_per_min: number
  daily_quota: number
  last_used_at?: string
  usage_count: number
  created_at: string
  updated_at: string
}

export interface RuntimeSettings {
  default_mode: string
  default_providers: string[]
  default_limit: number
  default_dedupe: boolean
  request_timeout_ms: number
  cache_enabled: boolean
  cache_ttl_seconds: number
  cache_max_results: number
  compat_tavily_enabled: boolean
  compat_serper_enabled: boolean
  compat_openai_enabled: boolean
  api_auth_required: boolean
}

export interface SearchLog {
  id: number
  request_id: string
  query: string
  mode: string
  compat_format: string
  providers: string[]
  cache_policy: string
  cache_hit: boolean
  result_count: number
  status: string
  error_message: string
  latency_ms: number
  request_json?: unknown
  response_json?: unknown
  created_at: string
}

export interface ProviderCallLog {
  provider_key_id: number
  provider_name: string
  key_alias: string
  status: string
  error_type: string
  error_message: string
  latency_ms: number
  result_count: number
  cached: boolean
}

export async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const session = useSessionStore()
  const headers = new Headers(options.headers || {})
  headers.set('Content-Type', 'application/json')
  if (session.token) {
    headers.set('Authorization', `Bearer ${session.token}`)
  }
  const response = await fetch(`${API_BASE}${path}`, { ...options, headers })
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}))
    throw new Error(payload?.error?.message || response.statusText)
  }
  return response.json() as Promise<T>
}

export const api = {
  login: (username: string, password: string) => apiFetch<{ token: string }>('/api/admin/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  dashboard: () => apiFetch<{ usage: UsageSummary; providers: ProviderConfig[] }>('/api/admin/dashboard'),
  providers: () => apiFetch<{ providers: ProviderConfig[] }>('/api/admin/providers'),
  updateProvider: (provider: ProviderConfig) => apiFetch('/api/admin/providers/' + provider.name, { method: 'PATCH', body: JSON.stringify(provider) }),
  keys: () => apiFetch<{ keys: ProviderKey[] }>('/api/admin/keys'),
  createKey: (payload: Record<string, unknown>) => apiFetch<ProviderKey>('/api/admin/keys', { method: 'POST', body: JSON.stringify(payload) }),
  updateKey: (id: number, payload: Record<string, unknown>) => apiFetch<ProviderKey>('/api/admin/keys/' + id, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteKey: (id: number) => apiFetch('/api/admin/keys/' + id, { method: 'DELETE' }),
  testKey: (id: number, payload: Record<string, unknown>) => apiFetch('/api/admin/keys/' + id + '/test', { method: 'POST', body: JSON.stringify(payload) }),
  tokens: () => apiFetch<{ tokens: ApiToken[] }>('/api/admin/tokens'),
  createToken: (payload: Record<string, unknown>) => apiFetch<{ token: ApiToken; raw_token: string }>('/api/admin/tokens', { method: 'POST', body: JSON.stringify(payload) }),
  updateToken: (id: number, payload: Record<string, unknown>) => apiFetch('/api/admin/tokens/' + id, { method: 'PATCH', body: JSON.stringify(payload) }),
  deleteToken: (id: number) => apiFetch('/api/admin/tokens/' + id, { method: 'DELETE' }),
  settings: () => apiFetch<RuntimeSettings>('/api/admin/settings'),
  updateSettings: (payload: RuntimeSettings) => apiFetch<RuntimeSettings>('/api/admin/settings', { method: 'PUT', body: JSON.stringify(payload) }),
  logs: () => apiFetch<{ logs: SearchLog[] }>('/api/admin/logs?limit=100'),
  logDetail: (id: number) => apiFetch<{ log: SearchLog; calls: ProviderCallLog[] }>('/api/admin/logs/' + id),
  usageSummary: () => apiFetch<UsageSummary>('/api/admin/usage/summary'),
  playgroundSearch: (payload: Record<string, unknown>) => apiFetch('/api/admin/playground/search', { method: 'POST', body: JSON.stringify(payload) })
}
