# One Search Relay

One Search Relay 是一个 Web Search API 中转与聚合项目，用于统一管理 Exa、You.com、Jina 等平台的多个 API Key，并通过统一接口执行并发搜索、fallback 搜索、单平台搜索、结果去重聚合和缓存。

## Features

- 统一 `POST /v1/search` 搜索 API。
- Tavily-like、Serper-like、OpenAI-like 兼容入口。
- Exa、You.com、Jina 首批 Provider 适配器。
- 多 Provider Key 管理：加密存储、别名、状态、RPM、并发、冷却、禁用。
- 搜索模式可配置：`parallel`、`fallback`、`single`。
- 缓存可配置：全局开关、TTL、单次请求 `default/bypass/refresh`。
- 单管理员后台，支持创建多个外部 API Token。
- PostgreSQL 请求日志、provider 调用日志和用量聚合。
- Go 后端 + Vue 管理台 + Docker Compose 部署。

## Project Layout

```text
backend/       Go API server
frontend/      Vue 3 management dashboard
deploy/        Nginx reverse proxy config
docs/          API/database/cache docs
```

## Quick Start with Docker

```bash
cp .env.example .env
# Edit ADMIN_PASSWORD and ENCRYPTION_KEY before production use.
docker compose up --build
```

- 管理台：http://localhost:3000（开发模式也可使用 http://localhost:5173）
- 后端健康检查：http://localhost:8080/healthz
- 默认管理员：`.env` 中的 `ADMIN_USERNAME` / `ADMIN_PASSWORD`

首次登录后：

1. 在“平台管理”页面点击平台卡片，在弹窗中配置 Base URL 和绑定 Key。
2. 在“API 令牌”页面创建一个外部调用 Token。
3. 在“系统设置”页面选择默认搜索模式、默认平台和缓存策略。
4. 在“搜索调试”页面测试搜索。

## Local Development

Backend:

```bash
cd backend
go mod tidy
go run ./cmd/server
```

Frontend:

```bash
cd frontend
npm install
npm run dev
```

Vite 开发服务器会把 `/api` 和 `/v1` 代理到 `http://localhost:8080`。

## Native Search API

```bash
curl -X POST http://localhost:8080/v1/search \
  -H "Authorization: Bearer osr_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "latest web search APIs",
    "providers": ["exa", "you", "jina"],
    "mode": "parallel",
    "limit": 10,
    "cache": "default"
  }'
```

Response:

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

## Compatibility Endpoints

- `POST /v1/compat/tavily/search`
- `POST /v1/compat/serper/search`
- `POST /v1/compat/openai/responses-search`

These endpoints accept common third-party field names and map them to the internal search request. Project-specific extensions `providers`, `mode`, and `cache` are supported.

## Admin API

- `POST /api/admin/login`
- `GET /api/admin/dashboard`
- `GET/PATCH /api/admin/providers`
- `GET/POST/PATCH/DELETE /api/admin/keys`
- `GET/POST/PATCH/DELETE /api/admin/tokens`
- `GET/PUT /api/admin/settings`
- `GET /api/admin/logs`
- `POST /api/admin/playground/search`

## Security Notes

- Upstream provider keys are encrypted with `ENCRYPTION_KEY` before being stored in PostgreSQL.
- Full provider keys are never returned by API responses; only masked hints are shown.
- External search APIs are protected by API Tokens when `api_auth_required=true`.
- New API Tokens are shown only once when created.

## Provider Notes

- Exa uses `Authorization: Bearer <key>` and `/search`.
- You.com uses `X-API-Key: <key>` and `/search`.
- Jina uses `Authorization: Bearer <key>` when provided and `https://s.jina.ai/{query}`.

Provider response formats evolve, so adapters parse common field names defensively and normalize to the project schema.

## Docs

- `docs/api-schema.md`
- `docs/database.md`
- `docs/cache.md`
