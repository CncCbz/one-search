# One Search

自托管 Web Search API 中转 / 聚合网关。

统一接入 Exa、You.com、Jina、Tavily、Firecrawl、Serper、Brave，提供：

- 统一搜索接口 `POST /v1/search`（`parallel` / `fallback` / `single`）
- Tavily / Serper / OpenAI 兼容接口
- Web 管理台：Provider、Key、Token、调试、日志、用量、审计
- 可选 MCP（`search` 工具）

预览图
<p>
  <img src="./docs/images/搜索调试.png" alt="搜索调试" width="132" />
  <img src="./docs/images/仪表盘.png" alt="仪表盘" width="120" />
</p>

## 快速部署

需要 Docker 24+ / Compose v2，以及至少一个上游搜索 API Key。

```bash
git clone https://github.com/CncCbz/one-search.git
cd one-search
cp .env.example .env
```

编辑 `.env`，至少填写：

```dotenv
POSTGRES_PASSWORD=强密码
ADMIN_PASSWORD=管理员密码
ENCRYPTION_KEY=至少32字符   # openssl rand -base64 32
```

```bash
docker compose up --build -d
curl http://localhost:5173/healthz
```

打开 <http://localhost:5173>，用管理员账号登录。

## 首次配置

1. **平台管理**：启用要用的 Provider  
2. **Key 管理**：添加上游 API Key  
3. **搜索调试**：验证可用性  
4. **API 令牌**：创建业务用的 `osr_...` Token  

## 使用

```bash
curl -X POST http://localhost:5173/v1/search \
  -H "Authorization: Bearer osr_你的令牌" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "golang web search",
    "mode": "fallback",
    "limit": 5,
    "providers": ["brave", "tavily"]
  }'
```

也可用 `X-API-Key: osr_xxx`。

| 路径 | 说明 |
| --- | --- |
| `/` | 管理台 |
| `/healthz` | 健康检查 |
| `/v1/search` | 统一搜索 |
| `/v1/compat/tavily/search` | Tavily 兼容 |
| `/v1/compat/serper/search` | Serper 兼容 |
| `/v1/compat/openai/responses-search` | OpenAI 兼容 |
| `/mcp` | MCP（默认开启） |

完整接口见 [docs/admin-api-key.md](docs/admin-api-key.md)、[docs/mcp.md](docs/mcp.md)。

## MCP 配置

`.env.example` 默认 `MCP_ENABLED=true`，端点：

```text
http://localhost:5173/mcp
```

先在管理台创建 `osr_...` Token（`API_AUTH_REQUIRED=true` 时必须）。自检：

```bash
curl http://localhost:5173/mcp
# 应返回 enabled:true、tools:["search"]
```

### Codex

```bash
export ONE_SEARCH_API_TOKEN=osr_xxx
```

写入 `~/.codex/config.toml`：

```toml
[mcp_servers.one_search]
url = "http://localhost:5173/mcp"
bearer_token_env_var = "ONE_SEARCH_API_TOKEN"
enabled = true
tool_timeout_sec = 60
enabled_tools = ["search"]
```

启动 Codex 后输入 `/mcp`，应能看到 `one_search` / `search`。

### Claude Desktop / 通用 HTTP MCP

多数客户端填：

```json
{
  "url": "http://localhost:5173/mcp",
  "headers": {
    "Authorization": "Bearer osr_xxx"
  }
}
```

更多参数、错误码、排错见 [docs/mcp.md](docs/mcp.md)。

## 配置说明

### 常用（`.env.example` 已列出）

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `HOST_PORT` | `5173` | 宿主机端口 |
| `POSTGRES_PASSWORD` | — | **必填** |
| `ADMIN_USERNAME` | `admin` | 首次管理员用户名 |
| `ADMIN_PASSWORD` | — | 生产**必填** |
| `ENCRYPTION_KEY` | — | **必填**，≥32 字符，加密敏感 Key |
| `API_AUTH_REQUIRED` | `true` | `/v1/*`、MCP 是否强制 Token |
| `MCP_ENABLED` | `true` | 是否开启 MCP |

### 可选（一般不用改，需要时加到 `.env`）

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `APP_ENV` | Compose 下 `production` | 生产请保持 `production` |
| `POSTGRES_DB` / `POSTGRES_USER` | `one_search` | 库名 / 用户 |
| `HTTP_ADDR` | `:8080` | 容器内后端监听地址 |
| `MCP_PATH` | `/mcp` | MCP 路径 |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:8080` | CORS 白名单 |
| `DATABASE_URL` | 本地开发用 | all-in-one 会自动生成，无需手写 |
| `RUN_MIGRATIONS` | `true` | 启动时自动迁移 |
| `REQUEST_TIMEOUT_MS` | `20000` | 上游请求超时 |
| `REQUEST_BODY_LIMIT_BYTES` | `1048576` | 请求体上限 |
| `SERVER_*_TIMEOUT_MS` | 见代码默认 | HTTP 服务器超时 |
| `ADMIN_SESSION_TTL_HOURS` | `24` | 管理 Session 时长 |
| `ADMIN_LOGIN_MAX_ATTEMPTS` 等 | 5 / 5min / 15min | 登录限速与锁定 |
| `VITE_API_BASE` | 空 | 前后端分离开发时指向后端 |
| `ONE_SEARCH_HTTP(S)_PROXY` | 空 | 容器访问上游时的代理 |

公网请在前面加 HTTPS 反代，转发 `/`、`/api/`、`/v1/`、`/healthz`（以及 `/mcp`）。

## 本地开发

```bash
# DB
docker run -d --name one-search-postgres \
  -e POSTGRES_DB=one_search -e POSTGRES_USER=one_search -e POSTGRES_PASSWORD=one_search \
  -p 15432:5432 postgres:16-alpine

# 后端
cd backend
export APP_ENV=development HTTP_ADDR=:18080 \
  DATABASE_URL='postgres://one_search:one_search@localhost:15432/one_search?sslmode=disable' \
  ADMIN_PASSWORD=admin123456 \
  ENCRYPTION_KEY=local-test-encryption-key-for-runtime \
  RUN_MIGRATIONS=true MIGRATIONS_DIR=migrations
go run ./cmd/server

# 前端
cd frontend && npm install && npm run dev
# 分离运行时可设 VITE_API_BASE=http://localhost:18080
```

## 目录

```text
backend/     Go API + migrations
frontend/    Vue 管理台
deploy/      all-in-one 入口 + nginx
docs/        接口文档
```

## License

[Apache License 2.0](LICENSE) © 2026 CncCbz
