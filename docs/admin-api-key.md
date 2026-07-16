# 管理员 API Key 接口文档

管理员 API Key 是 One Search Relay 提供给可信外部系统使用的超级凭据。它可以调用需要管理员权限的 `/api/admin/*` 接口，也可以调用需要普通 API Token 的 `/v1/*` 搜索接口。

> 代码依据：后端在 `backend/internal/api/auth.go` 中的 `requireAdmin` 和 `requireAPIToken` 都会识别管理员 API Key；路由注册见 `backend/internal/api/handlers.go`。

## 1. 基本概念

- 管理员 API Key 前缀：`oak_`
- 外部搜索 API Token 前缀：`osr_`
- 管理员登录 Session Token 前缀：`adm_`
- 系统同时只保存一个管理员 API Key。每次重新生成都会让旧 Key 立即失效。
- 管理员 API Key 明文只在生成响应中显示一次，之后只能查看 `key_prefix`、`created_at`、`updated_at`。
- 管理员 API Key 拥有完整管理权限，请只保存在可信服务端环境中。

## 2. 生成和查看管理员 API Key

### 2.1 管理员登录

先使用管理员账号登录，获取 `adm_` Session Token：

```bash
curl -X POST http://localhost:5173/api/admin/login \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "admin",
    "password": "your-admin-password"
  }'
```

响应：

```json
{
  "token": "adm_xxx",
  "expires_at": "2026-06-12T00:00:00Z"
}
```

### 2.2 生成或轮换管理员 API Key

```bash
curl -X POST http://localhost:5173/api/admin/settings/admin-api-key \
  -H "Authorization: Bearer adm_xxx"
```

也可以用已有管理员 API Key 自我轮换：

```bash
curl -X POST http://localhost:5173/api/admin/settings/admin-api-key \
  -H "Authorization: Bearer oak_old_xxx"
```

响应示例：

```json
{
  "key": "oak_new_xxx",
  "key_prefix": "oak_new_",
  "created_at": "2026-06-11T10:00:00Z",
  "updated_at": "2026-06-11T10:00:00Z"
}
```

注意：

- `key` 只在这一次响应中出现。
- 轮换成功后旧的 `oak_` Key 立即不可用。
- 该操作会写入审计日志，actor 形如 `admin_api_key:<key_prefix>` 或 `admin`。

### 2.3 查看当前管理员 API Key 元信息

```bash
curl http://localhost:5173/api/admin/settings/admin-api-key \
  -H "Authorization: Bearer adm_xxx"
```

响应示例：

```json
{
  "key_prefix": "oak_abcd",
  "created_at": "2026-06-11T10:00:00Z",
  "updated_at": "2026-06-11T10:00:00Z"
}
```

如果尚未生成过管理员 API Key，响应为空对象：

```json
{}
```

## 3. 鉴权调用方式

管理员 API Key 支持两种请求头。

推荐方式：

```http
Authorization: Bearer oak_xxx
```

兼容方式：

```http
X-API-Key: oak_xxx
```

示例：

```bash
export BASE_URL=http://localhost:5173
export ADMIN_API_KEY=oak_xxx

curl "$BASE_URL/api/admin/dashboard" \
  -H "Authorization: Bearer $ADMIN_API_KEY"
```

等价写法：

```bash
curl "$BASE_URL/api/admin/dashboard" \
  -H "X-API-Key: $ADMIN_API_KEY"
```

## 4. 管理员 API Key 开放接口总览

除 `POST /api/admin/login` 是用户名密码登录入口外，管理员 API Key 可调用以下两类接口：

1. 所有受管理员权限保护的 `/api/admin/*` 接口。
2. 所有受 API Token 保护的 `/v1/*` 搜索接口。

`GET /healthz` 不需要鉴权。

### 4.1 管理接口 `/api/admin/*`

| 方法 | 路径 | 能否使用管理员 API Key | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/admin/logout` | 可以 | 注销管理员 Session。用管理员 API Key 调用时不会吊销 API Key 本身。 |
| `GET` | `/api/admin/me` | 可以 | 返回当前管理员身份信息。当前固定返回 `{"username":"admin"}`。 |
| `GET` | `/api/admin/dashboard` | 可以 | 获取用量、Provider、Provider 健康度、30 天账单摘要。 |
| `GET` | `/api/admin/providers` | 可以 | 获取 Provider 配置列表。 |
| `GET` | `/api/admin/providers/health` | 可以 | 获取 Provider 健康状态。 |
| `PATCH` | `/api/admin/providers/{name}` | 可以 | 更新 Provider 配置。`name` 为内置 Provider 名，例如 `exa`、`you`、`jina`、`tavily`、`firecrawl`、`serper`、`brave`。 |
| `GET` | `/api/admin/keys` | 可以 | 获取 Provider Key 列表，只返回脱敏信息。 |
| `POST` | `/api/admin/keys` | 可以 | 创建 Provider Key。 |
| `PATCH` | `/api/admin/keys/{id}` | 可以 | 更新 Provider Key。 |
| `POST` | `/api/admin/keys/{id}/test` | 可以 | 使用指定 Provider Key 发起测试搜索。 |
| `POST` | `/api/admin/keys/{id}/quota` | 可以 | 查询并保存该 Key 的官方额度/账单信息或本地估算额度。 |
| `DELETE` | `/api/admin/keys/{id}` | 可以 | 删除 Provider Key。 |
| `GET` | `/api/admin/tokens` | 可以 | 获取外部 API Token 列表，只返回前缀和配置。 |
| `POST` | `/api/admin/tokens` | 可以 | 创建外部 API Token，响应中的 `raw_token` 只显示一次。 |
| `PATCH` | `/api/admin/tokens/{id}` | 可以 | 更新 Token 配置或状态。 |
| `DELETE` | `/api/admin/tokens/{id}` | 可以 | 删除外部 API Token。 |
| `GET` | `/api/admin/settings` | 可以 | 获取运行时设置。 |
| `PUT` | `/api/admin/settings` | 可以 | 覆盖更新运行时设置。 |
| `GET` | `/api/admin/settings/admin-api-key` | 可以 | 查看管理员 API Key 元信息。 |
| `POST` | `/api/admin/settings/admin-api-key` | 可以 | 生成/轮换管理员 API Key。 |
| `GET` | `/api/admin/logs` | 可以 | 获取搜索日志列表。支持 `limit`。 |
| `GET` | `/api/admin/logs/{id}` | 可以 | 获取单条搜索日志详情和 Provider 调用明细。 |
| `GET` | `/api/admin/usage/summary` | 可以 | 获取总体用量汇总。 |
| `GET` | `/api/admin/usage/billing` | 可以 | 获取账单/计量汇总。支持 `days`。 |
| `GET` | `/api/admin/metrics` | 可以 | 获取网关指标聚合。 |
| `GET` | `/api/admin/audit-logs` | 可以 | 获取审计日志列表。支持 `limit`。 |
| `POST` | `/api/admin/playground/search` | 可以 | 管理台搜索调试接口，不绑定外部 API Token ID。 |

### 4.2 搜索接口 `/v1/*`

管理员 API Key 也可用于搜索接口，效果类似超级 API Token：

| 方法 | 路径 | 能否使用管理员 API Key | 说明 |
| --- | --- | --- | --- |
| `POST` | `/v1/search` | 可以 | 原生搜索接口。 |
| `POST` | `/v1/compat/tavily/search` | 可以 | Tavily-like 兼容搜索接口。 |
| `POST` | `/v1/compat/serper/search` | 可以 | Serper-like 兼容搜索接口。 |
| `POST` | `/v1/compat/openai/responses-search` | 可以 | OpenAI-like 兼容搜索接口。 |
| `GET` | `/v1/providers` | 可以 | 获取 Provider 配置列表。 |
| `GET` | `/v1/usage/summary` | 可以 | 获取用量汇总。 |

与普通 `osr_` API Token 不同，管理员 API Key 不会受到 `allowed_providers` 限制，也不会作为 `api_token_id` 写入用量归属。

## 5. 管理接口调用示例

以下示例假设：

```bash
export BASE_URL=http://localhost:5173
export ADMIN_API_KEY=oak_xxx
```

### 5.1 查看 Dashboard

```bash
curl "$BASE_URL/api/admin/dashboard" \
  -H "Authorization: Bearer $ADMIN_API_KEY"
```

响应包含：

- `usage`：总体请求数、成功/失败数、缓存命中、平均延迟。
- `providers`：Provider 配置。
- `provider_health`：Provider 健康度。
- `billing`：近 30 天计量/费用汇总。

### 5.2 查看 Provider 列表

```bash
curl "$BASE_URL/api/admin/providers" \
  -H "Authorization: Bearer $ADMIN_API_KEY"
```

响应示例：

```json
{
  "providers": [
    {
      "id": 1,
      "name": "exa",
      "display_name": "Exa",
      "base_url": "https://api.exa.ai",
      "enabled": true,
      "priority": 10,
      "weight": 1,
      "timeout_ms": 12000,
      "settings": { "key_retry_count": 3, "max_concurrency": 0 },
      "available_keys": 1
    }
  ]
}
```

### 5.3 更新 Provider 配置

```bash
curl -X PATCH "$BASE_URL/api/admin/providers/exa" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "display_name": "Exa",
    "base_url": "https://api.exa.ai",
    "enabled": true,
    "priority": 10,
    "weight": 1,
    "timeout_ms": 12000,
    "settings": {
      "key_retry_count": 3,
      "max_concurrency": 0,
      "request_result_limit": 10,
      "retry_error_types": ["auth", "quota_exhausted", "rate_limited"],
      "key_routing_strategy": "weighted_random"
    }
  }'
```

成功响应：

```json
{"status":"ok"}
```

说明：`PATCH /api/admin/providers/{name}` 会按请求体覆盖该 Provider 的主要配置字段，因此建议先 `GET /api/admin/providers`，在原对象基础上修改后再提交。`settings.max_concurrency` 是渠道级最大并发请求数，`0` 表示不限，正数表示该 Provider 同时在途请求上限。

### 5.4 创建 Provider Key

You.com 示例：

```bash
curl -X POST "$BASE_URL/api/admin/keys" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "provider_name": "you",
    "alias": "you-main",
    "key": "you-provider-key",
    "weight": 1,
    "rpm_limit": 60,
    "daily_quota": 0,
    "monthly_quota": 0
  }'
```

Exa 示例：

```bash
curl -X POST "$BASE_URL/api/admin/keys" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "provider_name": "exa",
    "alias": "exa-main",
    "key": "exa-search-api-key",
    "exa_api_key_id": "exa-api-key-id",
    "exa_service_key": "exa-team-management-x-api-key",
    "weight": 1,
    "rpm_limit": 60,
    "daily_quota": 0,
    "monthly_quota": 0
  }'
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `provider_name` | 内置 Provider 名：`exa`、`you`、`jina`、`tavily`、`firecrawl`、`serper`、`brave`。 |
| `alias` | Key 别名。同一 Provider 下唯一。 |
| `key` | 上游搜索 API Key，会加密存储。 |
| `exa_api_key_id` | Exa 官方 usage 查询使用的 API Key ID。Exa 可选但建议填写。 |
| `exa_service_key` | Exa Team Management `x-api-key`。创建 Exa Key 时当前后端要求必填。 |
| `weight` | 权重，默认 1。 |
| `rpm_limit` | 单 Key 每分钟限制，0 表示不限。 |
| `daily_quota` | 单 Key 日请求额度，0 表示不限。 |
| `monthly_quota` | 单 Key 月请求额度，0 表示不限。 |

### 5.5 更新 Provider Key

```bash
curl -X PATCH "$BASE_URL/api/admin/keys/1" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "alias": "you-main-updated",
    "status": "enabled",
    "weight": 2,
    "rpm_limit": 120,
    "daily_quota": 5000,
    "monthly_quota": 100000
  }'
```

可更新字段：

- `alias`
- `key`
- `exa_api_key_id`
- `exa_service_key`
- `status`：`enabled`、`disabled`、`cooling`、`exhausted`
- `weight`
- `rpm_limit`
- `daily_quota`
- `monthly_quota`

### 5.6 测试 Provider Key

```bash
curl -X POST "$BASE_URL/api/admin/keys/1/test" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "one search relay",
    "limit": 3
  }'
```

响应包含：

- `summary`：状态、错误类型、延迟、结果数等。
- `results`：归一化后的搜索结果。

### 5.7 查询额度/账单

```bash
curl -X POST "$BASE_URL/api/admin/keys/1/quota" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{}'
```

Exa 可传日期和分组参数：

```bash
curl -X POST "$BASE_URL/api/admin/keys/1/quota" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "start_date": "2026-06-01",
    "end_date": "2026-06-11",
    "group_by": "day"
  }'
```

支持情况：

| Provider | 查询方式 | 结果含义 |
| --- | --- | --- |
| `exa` | Team Management usage API | 指定周期用量/费用，不是剩余额度。 |
| `you` | Billing account balance API | 账户余额，单位 cents/USD。 |
| `jina` | `https://r.jina.ai/` 文本解析 | 解析 `Balance left`，单位 tokens。 |
| `tavily` | `GET https://api.tavily.com/usage` | 返回当前 Key usage/limit，按 credits 展示剩余额度。 |
| `firecrawl` | `GET https://api.firecrawl.dev/v2/team/credit-usage` | 返回团队 remainingCredits/planCredits 和账期。 |
| `serper` | 本地累计用量估算 | Serper 未公开独立余额接口；按默认总额度 2500 credits 减本地累计 credits 估算剩余额度，不额外请求上游。 |
| `brave` | `GET https://api.search.brave.com/res/v1/web/search` | Brave 通过 `X-RateLimit-*` 响应头返回剩余请求额度；查询本身会消耗一次成功请求。 |

### 5.8 创建外部 API Token

```bash
curl -X POST "$BASE_URL/api/admin/tokens" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "client-a",
    "scopes": ["search"],
    "allowed_providers": ["exa", "you"],
    "rate_limit_per_min": 60,
    "daily_quota": 1000,
    "monthly_quota": 30000
  }'
```

响应示例：

```json
{
  "token": {
    "id": 1,
    "name": "client-a",
    "token_prefix": "osr_abcd",
    "scopes": ["search"],
    "allowed_providers": ["exa", "you"],
    "status": "enabled",
    "rate_limit_per_min": 60,
    "daily_quota": 1000,
    "monthly_quota": 30000,
    "usage_count": 0,
    "created_at": "2026-06-11T10:00:00Z",
    "updated_at": "2026-06-11T10:00:00Z"
  },
  "raw_token": "osr_xxx"
}
```

`raw_token` 只显示一次。

### 5.9 更新外部 API Token

更新配额/允许 Provider：

```bash
curl -X PATCH "$BASE_URL/api/admin/tokens/1" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "client-a",
    "allowed_providers": ["exa", "you", "jina", "tavily", "firecrawl", "serper", "brave"],
    "rate_limit_per_min": 120,
    "daily_quota": 2000,
    "monthly_quota": 60000
  }'
```

启用/禁用 Token：

```bash
curl -X PATCH "$BASE_URL/api/admin/tokens/1" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"status":"disabled"}'
```

注意：当前后端逻辑中，如果请求体包含非空 `name`，则执行配置更新；如果 `name` 为空且 `status` 非空，则执行状态更新。

### 5.10 查看和更新运行时设置

查看：

```bash
curl "$BASE_URL/api/admin/settings" \
  -H "Authorization: Bearer $ADMIN_API_KEY"
```

更新：

```bash
curl -X PUT "$BASE_URL/api/admin/settings" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "default_mode": "parallel",
    "default_providers": ["exa", "you", "jina", "tavily", "firecrawl", "serper", "brave"],
    "default_limit": 10,
    "default_dedupe": true,
    "request_timeout_ms": 20000,
    "cache_enabled": false,
    "cache_ttl_seconds": 3600,
    "cache_max_results": 20,
    "compat_tavily_enabled": true,
    "compat_serper_enabled": true,
    "compat_openai_enabled": true,
    "api_auth_required": true,
    "provider_health_window_minutes": 15,
    "provider_routing_strategy": "fixed",
    "log_retention_days": 3
  }'
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `default_mode` | 默认搜索模式：`parallel`、`fallback`、`single`。 |
| `default_providers` | 默认 Provider 列表；新库初始化和默认 fallback 为 `exa`、`you`、`jina`、`tavily`、`firecrawl`、`serper`、`brave`。 |
| `default_limit` | 默认返回结果数，搜索时最大限制为 50。 |
| `default_dedupe` | 是否默认去重。 |
| `request_timeout_ms` | 单次搜索总超时。 |
| `cache_enabled` | 是否开启缓存。 |
| `cache_ttl_seconds` | 缓存 TTL；空结果会用更短 TTL（60s，且不超过该值）。 |
| `cache_max_results` | 单次响应最多缓存的结果条数，超出截断后写入；0 表示不截断。 |
| `compat_tavily_enabled` | 是否启用 Tavily-like 兼容接口。 |
| `compat_serper_enabled` | 是否启用 Serper-like 兼容接口。 |
| `compat_openai_enabled` | 是否启用 OpenAI-like 兼容接口。 |
| `api_auth_required` | `/v1/*` 是否需要 API Token 或管理员 API Key。 |
| `provider_health_window_minutes` | Provider 健康统计窗口。 |
| `provider_routing_strategy` | Provider 级路由策略：`fixed`、`priority`、`weighted`、`weighted_random`、`available_keys`、`random`。 |
| `log_retention_days` | 搜索日志和审计日志保留天数。 |

### 5.11 查看日志和审计日志

管理台的仪表盘、搜索日志和审计日志表格会限制在右侧内容区域内滚动，避免内容较多时触发浏览器全局滚动条。

搜索日志列表：

```bash
curl "$BASE_URL/api/admin/logs?limit=100" \
  -H "Authorization: Bearer $ADMIN_API_KEY"
```

搜索日志详情：

```bash
curl "$BASE_URL/api/admin/logs/1" \
  -H "Authorization: Bearer $ADMIN_API_KEY"
```

审计日志：

```bash
curl "$BASE_URL/api/admin/audit-logs?limit=100" \
  -H "Authorization: Bearer $ADMIN_API_KEY"
```

### 5.12 搜索调试接口

```bash
curl -X POST "$BASE_URL/api/admin/playground/search" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "latest AI search API",
    "providers": ["exa", "you"],
    "mode": "parallel",
    "limit": 5,
    "cache": "bypass"
  }'
```

该接口与 `/v1/search` 使用相同的原生搜索请求格式，但作为管理台调试接口，不要求外部 API Token，也不会绑定 `api_token_id`。

## 6. 搜索接口调用示例

管理员 API Key 可直接调用搜索接口。

### 6.1 原生搜索

```bash
curl -X POST "$BASE_URL/v1/search" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "latest web search APIs",
    "providers": ["exa", "you", "jina", "tavily", "firecrawl", "serper", "brave"],
    "mode": "parallel",
    "limit": 10,
    "cache": "default",
    "include_raw": false
  }'
```

### 6.2 Tavily-like

```bash
curl -X POST "$BASE_URL/v1/compat/tavily/search" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "latest web search APIs",
    "search_depth": "advanced",
    "max_results": 5,
    "include_raw_content": false,
    "providers": ["exa", "you"],
    "mode": "parallel",
    "cache": "default"
  }'
```

### 6.3 Serper-like

```bash
curl -X POST "$BASE_URL/v1/compat/serper/search" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "q": "latest web search APIs",
    "num": 5,
    "gl": "us",
    "hl": "en",
    "providers": ["you"],
    "cache": "bypass"
  }'
```

### 6.4 OpenAI-like

```bash
curl -X POST "$BASE_URL/v1/compat/openai/responses-search" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "input": "latest web search APIs",
    "limit": 5,
    "providers": ["exa", "jina"],
    "mode": "fallback"
  }'
```

### 6.5 查看 `/v1` Provider 和用量

```bash
curl "$BASE_URL/v1/providers" \
  -H "Authorization: Bearer $ADMIN_API_KEY"

curl "$BASE_URL/v1/usage/summary" \
  -H "Authorization: Bearer $ADMIN_API_KEY"
```

## 7. 状态码和错误格式

错误响应统一为：

```json
{
  "error": {
    "message": "admin login required",
    "status": 401
  }
}
```

常见状态码：

| 状态码 | 场景 |
| --- | --- |
| `400` | JSON 格式错误、路径参数错误、请求体非法。 |
| `401` | 缺少或使用了无效管理员 Session / 管理员 API Key / API Token。 |
| `403` | 普通 API Token 请求了未授权 Provider。管理员 API Key 不受此限制。 |
| `404` | 兼容接口被禁用时返回；或反向代理未命中路径。 |
| `429` | 管理员登录失败次数过多，或普通 API Token 触发 RPM 限制。 |
| `502` | Provider Key 测试或官方额度查询时上游失败。 |
| `500` | 数据库、配置、加解密或内部服务错误。 |

## 8. 审计行为

使用管理员 API Key 调用管理接口时，部分写操作会写入 `audit_logs`。actor 形如：

```text
admin_api_key:oak_abcd
```

会记录的典型动作包括：

- `provider.update`
- `provider_key.create`
- `provider_key.update`
- `provider_key.test`
- `provider_key.quota`
- `provider_key.delete`
- `api_token.create`
- `api_token.update`
- `api_token.status`
- `api_token.delete`
- `settings.update`
- `settings.admin_api_key.rotate`
- `admin.logout`

## 9. 使用建议

- 管理员 API Key 适合 CI/CD、内部运维平台、自动化配置同步、可信后端服务调用。
- 不建议把管理员 API Key 放入浏览器前端、移动端或第三方不可控环境。
- 如只需要搜索能力，应创建 `osr_` 外部 API Token，并用 `allowed_providers`、RPM、日/月额度做限制。
- 轮换管理员 API Key 前确认所有依赖方都能同步更新；轮换后旧 Key 会立即失效。
- 建议定期查看 `/api/admin/audit-logs`，确认管理操作来源符合预期。
