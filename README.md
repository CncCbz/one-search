# One Search Relay

One Search Relay 是一个 Web Search API 中转与聚合网关。它统一管理 Exa、You.com、Jina 等搜索平台的多个 API Key，对外提供统一的搜索接口、兼容第三方格式的搜索接口、缓存、调用日志、用量统计和 Web 管理台。

## 项目分析

这个仓库由三部分组成：

- **后端**：`backend/`，Go + chi HTTP 路由，负责鉴权、搜索编排、Provider Key 轮换、缓存、日志和 PostgreSQL 持久化。
- **前端**：`frontend/`，Vue 3 + Element Plus 管理台，负责平台配置、Key 管理、Token 管理、日志查看和搜索调试。
- **部署层**：根目录 `Dockerfile` + `deploy/nginx.conf` + `deploy/all-in-one-entrypoint.sh`，默认构建一个同时运行 Nginx、Go 后端和 PostgreSQL 的单容器服务。

核心请求链路：

```text
外部调用方
  -> /v1/search 或 /v1/compat/*
  -> API Token / 管理员 API Key 鉴权
  -> search.Orchestrator
  -> keypool.Manager 选择可用 Provider Key
  -> Exa / You.com / Jina Adapter
  -> 结果归一化、去重、缓存、日志与用量统计
```

管理链路：

```text
管理员登录管理台
  -> /api/admin/*
  -> 管理 Provider、Provider Key、外部 API Token、运行时设置、日志、审计日志
  -> 可生成一个拥有完整管理权限的管理员 API Key，供可信外部系统调用
```

## 主要功能

- 统一搜索接口：`POST /v1/search`。
- 兼容接口：Tavily-like、Serper-like、OpenAI-like。
- 首批 Provider：Exa、You.com、Jina。
- 多 Provider Key 管理：加密存储、别名、状态、权重、RPM、并发、日/月额度、失败冷却。
- 搜索模式：`parallel` 并发聚合、`fallback` 失败转移、`single` 单平台。
- 路由策略：固定顺序、优先级、权重、随机、按可用 Key 优先等。
- 缓存策略：全局缓存开关、TTL、单次请求 `default` / `bypass` / `refresh`。
- 外部 API Token：可限制允许调用的 Provider、RPM、日/月额度。
- 管理员 API Key：可用作外部系统的超级管理凭据。
- 观测能力：搜索日志、Provider 调用明细、用量统计、计费/额度摘要、审计日志。
- 安全能力：管理员登录限速、敏感 Key 加密存储、Token 只显示一次、安全响应头。

## 目录结构

```text
backend/                 Go 后端服务
  cmd/server/            服务入口
  internal/api/          路由、鉴权、中间件、HTTP Handler
  internal/search/       搜索编排、缓存键、官方额度查询
  internal/keypool/      Provider Key 选择、限速和并发控制
  internal/provider/     Exa / You.com / Jina Adapter
  internal/db/           PostgreSQL Store 和迁移执行
  migrations/            数据库迁移 SQL
frontend/                Vue 3 管理台
deploy/                  Nginx 和 all-in-one 容器启动脚本
Dockerfile               前后端构建 + Nginx + PostgreSQL 单容器镜像
docker-compose.yml       默认部署入口
.env.example             环境变量示例
docs/admin-api-key.md    管理员 API Key 调用文档
```

## 快速开始：Docker Compose

1. 复制环境变量文件：

```bash
cp .env.example .env
```

2. 编辑 `.env`，至少设置以下值：

```dotenv
POSTGRES_PASSWORD=请替换为强密码
ADMIN_USERNAME=admin
ADMIN_PASSWORD=请替换为强密码
ENCRYPTION_KEY=至少32字符的随机字符串
```

`ENCRYPTION_KEY` 用于加密数据库中的 Provider Key 和 Token。丢失后，已有密文无法解密。

3. 启动服务：

```bash
docker compose up --build -d
```

4. 访问：

- 管理台：http://localhost:5173
- 健康检查：http://localhost:5173/healthz

默认 Compose 只启动一个 `app` 容器。容器内包含：

- Nginx：监听容器内 80 端口，代理 `/api/`、`/v1/`、`/healthz` 到后端。
- Go 后端：监听容器内 `:8080`。
- PostgreSQL：监听容器内 `127.0.0.1:5432`。

数据库数据保存在 Docker volume：`postgres_data`。

## 首次初始化

首次启动时，如果数据库中不存在管理员用户，服务会使用 `.env` 中的 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 创建初始管理员。

后续重启不会覆盖已有管理员密码。如果需要修改密码，目前需要直接更新数据库中的 `admin_users.password_hash`，或扩展后台接口。

登录管理台后建议按以下顺序配置：

1. 在 **平台管理** 中确认 Exa、You.com、Jina 的 Base URL、启用状态、超时、权重和高级设置。
2. 在 **Key 管理** 中添加各平台 Provider Key。
3. 在 **API 令牌** 中创建给外部调用方使用的 `osr_` API Token。
4. 在 **系统设置** 中配置默认搜索模式、默认 Provider、缓存、兼容接口开关和鉴权开关。
5. 在 **搜索调试** 中验证搜索链路。
6. 如需外部系统调用管理接口，在 **系统设置 / 管理员 API Key** 中生成 `oak_` 管理员 API Key。

## 本地开发

本地开发推荐只用 Docker 启动 PostgreSQL，后端和前端在本机直接运行。

根目录 `.env.development` 仅在 `APP_ENV=development` 或显式 `LOAD_DEVELOPMENT_ENV=true` 时加载，并且不会覆盖已经存在的环境变量。

### 启动 PostgreSQL

```bash
docker run -d --name one-search-test-postgres \
  -e POSTGRES_DB=one_search \
  -e POSTGRES_USER=one_search \
  -e POSTGRES_PASSWORD=one_search \
  -p 15432:5432 \
  postgres:16-alpine
```

### 启动后端

```bash
cd backend
go run ./cmd/server
```

默认后端地址：http://localhost:18080（取决于 `.env.development` 或环境变量中的 `HTTP_ADDR`）。

### 启动前端

```bash
cd frontend
npm install
npm run dev
```

默认前端地址：http://localhost:5173

前端会读取 `VITE_API_BASE`。开发环境通常配置为后端地址，例如：

```dotenv
VITE_API_BASE=http://localhost:18080
```

## 关键环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ENV` | `development` / Compose 中为 `production` | 运行环境。生产环境会拒绝弱密钥。 |
| `HTTP_ADDR` | `:8080` | Go 后端监听地址。 |
| `POSTGRES_DB` | `one_search` | all-in-one 容器中的数据库名。 |
| `POSTGRES_USER` | `one_search` | all-in-one 容器中的数据库用户。 |
| `POSTGRES_PASSWORD` | 无 | 生产部署必填。 |
| `DATABASE_URL` | 本地 PostgreSQL URL | 后端连接数据库使用；all-in-one 启动脚本会自动生成。 |
| `RUN_MIGRATIONS` | `true` | 启动时是否执行迁移。 |
| `MIGRATIONS_DIR` | `migrations` | 迁移目录。all-in-one 中为 `/app/backend/migrations`。 |
| `ADMIN_USERNAME` | `admin` | 初始管理员用户名。 |
| `ADMIN_PASSWORD` | 开发环境默认为 `admin123456`，生产环境无默认值 | 仅在首次创建管理员时使用。 |
| `ENCRYPTION_KEY` | 开发环境有弱默认值，生产环境无默认值 | 加密 Provider Key、API Token、管理员 API Key，生产环境至少 32 字符。 |
| `API_AUTH_REQUIRED` | `true` | 是否要求 `/v1/*` 搜索接口使用 API Token 或管理员 API Key。 |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:8080` | 允许跨域的 Origin 列表。 |
| `REQUEST_BODY_LIMIT_BYTES` | `1048576` | 请求体大小限制。 |
| `ADMIN_SESSION_TTL_HOURS` | `24` | 管理员登录 Session 有效期。 |
| `ADMIN_LOGIN_MAX_ATTEMPTS` | `5` | 管理员登录失败限速阈值。 |
| `ADMIN_LOGIN_WINDOW_MS` | `300000` | 登录失败统计窗口。 |
| `ADMIN_LOGIN_LOCKOUT_MS` | `900000` | 登录锁定时长。 |
| `UPSTREAM_USER_AGENT` | `OneSearchRelay/0.1` | 请求上游 Provider 时使用的 User-Agent。 |
| `VITE_API_BASE` | 空 | 前端调用 API 的基础地址。生产 all-in-one 通常留空。 |

## 鉴权方式

### 管理员登录 Session

管理台通过用户名密码登录：

```bash
curl -X POST http://localhost:5173/api/admin/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

响应中的 `token` 是 `adm_` Session Token，可用于调用 `/api/admin/*` 管理接口：

```bash
curl http://localhost:5173/api/admin/dashboard \
  -H "Authorization: Bearer adm_xxx"
```

### 外部 API Token

外部搜索调用方通常使用在管理台创建的 `osr_` API Token：

```bash
curl -X POST http://localhost:5173/v1/search \
  -H "Authorization: Bearer osr_xxx" \
  -H 'Content-Type: application/json' \
  -d '{"query":"latest web search APIs","limit":10}'
```

也可以使用 `X-API-Key`：

```bash
curl -X POST http://localhost:5173/v1/search \
  -H "X-API-Key: osr_xxx" \
  -H 'Content-Type: application/json' \
  -d '{"query":"latest web search APIs"}'
```

### 管理员 API Key

管理员 API Key 以 `oak_` 开头，拥有完整管理员权限，也可以作为超级搜索 Token 调用 `/v1/*`。

支持两种请求头：

```http
Authorization: Bearer oak_xxx
```

或：

```http
X-API-Key: oak_xxx
```

详细开放接口、生成方式和调用示例见：[`docs/admin-api-key.md`](docs/admin-api-key.md)。

## 搜索 API

### 原生搜索接口

```bash
curl -X POST http://localhost:5173/v1/search \
  -H "Authorization: Bearer osr_xxx" \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "latest web search APIs",
    "providers": ["exa", "you", "jina"],
    "mode": "parallel",
    "limit": 10,
    "cache": "default",
    "include_raw": false
  }'
```

请求字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `query` | string | 必填，搜索关键词。 |
| `providers` | string[] | 可选，限定 Provider：`exa`、`you`、`jina`。为空时使用系统默认值。 |
| `mode` | string | 可选：`parallel`、`fallback`、`single`。 |
| `limit` | number | 可选，返回结果数，最大 50。 |
| `freshness` | string | 可选，预留给兼容格式和 Provider 扩展。 |
| `dedupe` | boolean | 可选，是否按 URL 去重。 |
| `rerank` | boolean | 可选，当前为预留字段。 |
| `cache` | string | 可选：`default`、`bypass`、`refresh`。 |
| `include_raw` | boolean | 可选，是否在结果中包含上游原始条目。 |
| `options` | object | 可选，记录兼容接口或扩展参数。 |

响应示例：

```json
{
  "results": [
    {
      "title": "Example",
      "url": "https://example.com",
      "snippet": "...",
      "provider": "exa",
      "providers": ["exa"],
      "score": 0.9
    }
  ],
  "providers": [
    {
      "provider": "exa",
      "key_alias": "exa-main",
      "status": "success",
      "latency_ms": 123,
      "result_count": 10,
      "cached": false
    }
  ],
  "meta": {
    "request_id": "...",
    "mode": "parallel",
    "compat_format": "native",
    "latency_ms": 456,
    "total_results": 10,
    "deduped_results": 0,
    "cache_hit": false,
    "providers_queried": ["exa", "you", "jina"]
  }
}
```

### 兼容搜索接口

| 接口 | 说明 | 常见字段 |
| --- | --- | --- |
| `POST /v1/compat/tavily/search` | Tavily-like | `query`、`search_depth`、`topic`、`days`、`max_results`、`include_raw_content`、`include_domains`、`exclude_domains`、`providers`、`mode`、`cache` |
| `POST /v1/compat/serper/search` | Serper-like | `q`、`num`、`page`、`tbs`、`gl`、`hl`、`providers`、`mode`、`cache` |
| `POST /v1/compat/openai/responses-search` | OpenAI-like | `query` 或 `input`、`limit`、`providers`、`mode`、`cache` |

兼容接口是否启用由运行时设置控制：

- `compat_tavily_enabled`
- `compat_serper_enabled`
- `compat_openai_enabled`

禁用后对应接口返回 `404`。

## 管理接口概览

管理接口都位于 `/api/admin/*` 下。除 `POST /api/admin/login` 外，需要管理员 Session Token 或管理员 API Key。

常用接口：

| 接口 | 说明 |
| --- | --- |
| `POST /api/admin/login` | 管理员登录，返回 `adm_` Session Token。 |
| `POST /api/admin/logout` | 注销当前 Session。使用管理员 API Key 调用不会吊销该 Key。 |
| `GET /api/admin/dashboard` | 汇总用量、Provider、健康度和计费信息。 |
| `GET /api/admin/providers`、`PATCH /api/admin/providers/{name}` | 查看或更新 Provider 配置。 |
| `GET/POST/PATCH/DELETE /api/admin/keys` | 管理上游 Provider Key。 |
| `POST /api/admin/keys/{id}/test` | 测试某个 Provider Key。 |
| `POST /api/admin/keys/{id}/quota` | 查询并记录官方额度/账单信息。 |
| `GET/POST/PATCH/DELETE /api/admin/tokens` | 管理外部 API Token。 |
| `GET/PUT /api/admin/settings` | 查看或更新运行时设置。 |
| `GET/POST /api/admin/settings/admin-api-key` | 查看管理员 API Key 前缀或轮换管理员 API Key。 |
| `GET /api/admin/logs`、`GET /api/admin/logs/{id}` | 搜索日志列表和详情。 |
| `GET /api/admin/usage/summary` | 用量汇总。 |
| `GET /api/admin/usage/billing` | 按 Provider/计量单位汇总的账单信息。 |
| `GET /api/admin/providers/health` | Provider 健康状态。 |
| `GET /api/admin/metrics` | 网关指标聚合。 |
| `GET /api/admin/audit-logs` | 审计日志。 |
| `POST /api/admin/playground/search` | 管理台搜索调试接口。 |

管理员 API Key 可调用的完整接口表和示例见：[`docs/admin-api-key.md`](docs/admin-api-key.md)。

## Provider 配置说明

默认内置三个 Provider：

| Provider | 默认 Base URL | 搜索调用方式 | 说明 |
| --- | --- | --- | --- |
| `exa` | `https://api.exa.ai` | `POST /search`，`Authorization: Bearer <key>` | 创建 Exa Key 时还要求填写 Team Management `x-api-key`，用于官方 usage 查询。 |
| `you` | `https://ydc-index.io` | `GET /v1/search`，`X-API-Key: <key>` | 官方余额查询使用 `https://api.you.com/v1/billing/account_balance`。 |
| `jina` | `https://s.jina.ai` | `GET /{query}`，可选 `Authorization: Bearer <key>` | 官方额度信息从 `https://r.jina.ai/` 文本响应中解析。 |

Provider 的 `base_url` 从数据库实时读取，在管理台修改后无需重启后端。

Provider 高级设置保存在 `providers.settings` JSON 中，当前后端会读取：

| 设置键 | 说明 |
| --- | --- |
| `key_retry_count` | 单个 Provider 调用失败后最多换 Key 重试次数，默认 3，最大 20。 |
| `request_result_limit` | 单个 Provider 请求的结果数上限，最大 50。 |
| `retry_error_types` | 允许触发换 Key 重试的错误类型列表，例如 `auth`、`quota_exhausted`、`rate_limited`。 |
| `key_routing_strategy` | 同一 Provider 下多个 Key 的选择策略：默认轮询，也支持 `least_used`、`random`、`weighted_random`。 |

Provider Key 状态：

| 状态 | 说明 |
| --- | --- |
| `enabled` | 可用。 |
| `disabled` | 手动禁用或鉴权失败后禁用。 |
| `cooling` | 被限速后进入冷却。冷却时间过后可再次被选择。 |
| `exhausted` | 额度耗尽。 |

## 缓存和日志

缓存由运行时设置控制：

- `cache_enabled`：全局缓存开关。
- `cache_ttl_seconds`：缓存 TTL。
- `cache_max_results`：预留字段，当前主要用于设置展示。
- 请求级 `cache`：
  - `default`：遵循全局设置。
  - `bypass`：跳过读写缓存。
  - `refresh`：当前实现与 `bypass` 一样跳过缓存读写，并保留策略值用于日志和后续扩展。

日志写入 PostgreSQL：

- `search_requests`：每次搜索的请求、响应、状态和耗时。
- `provider_calls`：每个 Provider / 每次换 Key 重试的调用明细。
- `provider_call_usage`：Provider 返回的计量信息。
- `usage_daily`、`usage_meter_daily`：日级聚合。
- `audit_logs`：管理员操作和管理员 API Key 操作审计。

`log_retention_days` 控制搜索日志和审计日志保留天数；后台每小时执行一次清理，同时清理过期缓存。

## 安全说明

- 生产环境中 `ENCRYPTION_KEY` 不能为空、不能是已知弱默认值，且至少 32 字符。
- 生产环境中如果提供 `ADMIN_PASSWORD`，不能使用开发默认弱密码。
- Provider Key、外部 API Token、管理员 API Key 都会加密存储；鉴权使用 SHA-256 哈希匹配。
- Provider Key 列表只返回 `key_hint`，不会返回完整明文。
- 外部 API Token 和管理员 API Key 明文只会在创建/轮换响应中显示一次。
- 管理员登录失败会限速，并写入审计日志。
- 管理员 Session 存储在后端内存中，重启后失效。
- 响应包含 CSP、`X-Frame-Options: DENY`、`X-Content-Type-Options: nosniff` 等安全头。
- 管理员 API Key 拥有完整管理权限，建议只放在可信服务端环境中，并定期轮换。

## 常见操作

### 查看服务健康

```bash
curl http://localhost:5173/healthz
```

### 创建外部 API Token

```bash
curl -X POST http://localhost:5173/api/admin/tokens \
  -H "Authorization: Bearer adm_xxx" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "default-client",
    "scopes": ["search"],
    "allowed_providers": ["exa", "you", "jina"],
    "rate_limit_per_min": 60,
    "daily_quota": 1000,
    "monthly_quota": 30000
  }'
```

响应中的 `raw_token` 只显示一次，请立即保存。

### 创建 Provider Key

You.com 示例：

```bash
curl -X POST http://localhost:5173/api/admin/keys \
  -H "Authorization: Bearer adm_xxx" \
  -H 'Content-Type: application/json' \
  -d '{
    "provider_name": "you",
    "alias": "you-main",
    "key": "you-provider-key",
    "weight": 1,
    "rpm_limit": 60,
    "daily_quota": 0,
    "monthly_quota": 0,
    "max_concurrency": 2
  }'
```

Exa 示例：

```bash
curl -X POST http://localhost:5173/api/admin/keys \
  -H "Authorization: Bearer adm_xxx" \
  -H 'Content-Type: application/json' \
  -d '{
    "provider_name": "exa",
    "alias": "exa-main",
    "key": "exa-search-api-key",
    "exa_api_key_id": "exa-api-key-id-for-usage-query",
    "exa_service_key": "exa-team-management-x-api-key",
    "weight": 1,
    "rpm_limit": 60,
    "max_concurrency": 2
  }'
```

## 故障排查

- **生产环境启动失败，提示 `ENCRYPTION_KEY`**：检查 `.env` 是否设置至少 32 字符的随机值。
- **首次启动失败，提示 `ADMIN_PASSWORD is required`**：数据库中还没有管理员用户，生产环境必须设置 `ADMIN_PASSWORD`。
- **调用 `/v1/search` 返回 `api token required`**：创建并使用 `osr_` API Token，或使用 `oak_` 管理员 API Key；也可以在设置中关闭 `api_auth_required`。
- **Provider 返回 `no available key`**：对应 Provider 没有可用 Key，或 Key 被禁用、额度耗尽、冷却中、并发/RPM 已满。
- **兼容接口返回 404**：检查系统设置中对应兼容接口开关是否启用。
- **搜索成功但没有结果**：查看 `/api/admin/logs/{id}` 中的 Provider 调用明细和上游错误信息。

## 许可证

当前仓库未声明许可证。如需对外分发，请先补充 LICENSE。
