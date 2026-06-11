# One Search Relay

One Search Relay 是一个自托管的 Web Search API 中转与聚合网关。它把多个上游搜索平台统一成一个搜索入口，并提供 Web 管理台用于配置 Provider、管理 API Key、查看日志与用量。

适合以下场景：

- 统一管理多个搜索平台 API Key，避免业务侧直接持有上游 Key。
- 在 Exa、You.com、Jina、Tavily、Firecrawl、Serper、Brave Search 之间做并发聚合、fallback 或单平台调用。
- 给内部服务、Agent、MCP 客户端或自动化流程提供稳定的统一搜索 API。
- 在管理台查看调用日志、Provider 健康度、用量统计、额度/账单信息和审计日志。

## 功能概览

- **内置 Provider**：Exa、You.com、Jina、Tavily、Firecrawl、Serper、Brave Search。
- **统一搜索接口**：`POST /v1/search`，支持 `parallel`、`fallback`、`single` 搜索模式。
- **兼容接口**：Tavily-like、Serper-like、OpenAI-like，方便替换已有调用方。
- **Web 管理台**：Provider 配置、上游 Key 管理、外部 API Token、系统设置、搜索调试、日志和审计。
- **Key 管理**：加密存储、别名、状态、权重、RPM、并发、日/月额度、失败冷却。
- **用量与额度**：搜索日志、Provider 调用明细、日级用量聚合、官方额度查询和 Serper 本地额度估算。
- **安全能力**：生产环境强制密钥校验，敏感 Key 加密存储，Token 明文只显示一次，管理端登录限速。
- **可选 MCP**：提供 HTTP JSON-RPC MCP 接口，可供支持 MCP 的客户端调用搜索工具。

## 快速部署

部署前请先准备：

- Docker 24+。
- Docker Compose v2。
- 至少一个上游搜索平台 API Key。
- 一个用于生产环境的强管理员密码和 32 字符以上的 `ENCRYPTION_KEY`。

推荐使用 Docker Compose。默认部署为一个 all-in-one 容器，内部包含：

- Nginx：对外提供管理台静态资源，并代理 `/api/`、`/v1/`、`/mcp`、`/healthz`。
- Go 后端：提供管理接口、搜索接口、MCP 接口和任务清理。
- PostgreSQL：保存配置、Key 密文、日志、用量和审计数据。

### 1. 克隆项目

```bash
git clone <你的仓库地址>
cd one-search-relay
```

### 2. 准备环境变量

```bash
cp .env.example .env
```

编辑 `.env`，至少设置以下生产必填项：

```dotenv
APP_ENV=production
POSTGRES_PASSWORD=请替换为强密码
ADMIN_USERNAME=admin
ADMIN_PASSWORD=请替换为强密码
ENCRYPTION_KEY=至少32字符的随机字符串
```

`ENCRYPTION_KEY` 用于加密数据库中的上游 Provider Key、外部 API Token 和管理员 API Key。请妥善备份；丢失后，已有密文无法解密。

### 3. 启动服务

```bash
docker compose up --build -d
```

### 4. 检查状态

```bash
docker compose ps
docker compose logs -f app
curl http://localhost:5173/healthz
```

### 5. 打开管理台

浏览器访问：

```text
http://localhost:5173
```

使用 `.env` 中的 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 登录。

## 首次配置

登录管理台后，建议按以下顺序完成初始化：

1. 进入 **平台管理**，确认需要启用的 Provider，并按需禁用暂未配置 Key 的平台。
2. 进入 **Key 管理**，为对应 Provider 添加上游 API Key。
3. 进入 **搜索调试**，用真实查询验证 Provider 是否可用。
4. 进入 **系统设置**，确认默认搜索模式、默认 Provider、缓存、鉴权和兼容接口开关。
5. 进入 **API 令牌**，创建给业务系统调用的 `osr_` 外部 API Token。
6. 如果需要外部系统管理本服务，在 **系统设置 / 管理员 API Key** 中生成 `oak_` 管理员 API Key。

> 新库初始化和默认 fallback 会包含七个内置 Provider。如果你没有配置全部上游 Key，建议在管理台调整默认 Provider 或禁用暂不可用的平台。

## 访问入口

| 入口 | 默认地址 | 说明 |
| --- | --- | --- |
| 管理台 | `http://localhost:5173` | Web 管理界面。 |
| 健康检查 | `http://localhost:5173/healthz` | 容器和后端健康状态。 |
| 搜索 API | `http://localhost:5173/v1/search` | 统一搜索接口。 |
| 兼容 API | `http://localhost:5173/v1/compat/*` | Tavily-like、Serper-like、OpenAI-like。 |
| MCP | `http://localhost:5173/mcp` | 默认关闭，需设置 `MCP_ENABLED=true`。 |

## 关键配置

| 变量 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `APP_ENV` | 推荐 | `production` in Compose | 生产环境请使用 `production`。 |
| `POSTGRES_PASSWORD` | 是 | 无 | all-in-one PostgreSQL 密码。 |
| `ADMIN_USERNAME` | 否 | `admin` | 首次创建管理员时使用。 |
| `ADMIN_PASSWORD` | 生产必填 | 无 | 首次创建管理员时使用，生产环境不要使用弱密码。 |
| `ENCRYPTION_KEY` | 是 | 无 | 至少 32 字符；用于加密敏感 Key。 |
| `API_AUTH_REQUIRED` | 否 | `true` | 是否要求 `/v1/*` 和 MCP 调用携带 Token。 |
| `MCP_ENABLED` | 否 | `false` | 是否启用 MCP HTTP JSON-RPC 接口。 |
| `MCP_PATH` | 否 | `/mcp` | MCP 路径；all-in-one Nginx 默认代理 `/mcp`。 |
| `CORS_ALLOWED_ORIGINS` | 否 | `http://localhost:5173,http://localhost:8080` | 允许跨域的 Origin 列表。 |
| `VITE_API_BASE` | 否 | 空 | 前端 API 基础地址；all-in-one 通常保持为空。 |

完整变量示例见 `.env.example`。

## 数据持久化

Docker Compose 默认使用名为 `postgres_data` 的 volume 保存 PostgreSQL 数据：

```yaml
volumes:
  postgres_data:
```

常用命令：

```bash
# 停止服务但保留数据
docker compose down

# 停止服务并删除数据库数据（谨慎）
docker compose down -v

# 查看日志
docker compose logs -f app
```

升级版本时通常执行：

```bash
git pull
docker compose up --build -d
```

默认 `RUN_MIGRATIONS=true`，后端启动时会自动执行数据库迁移。

## 反向代理与 HTTPS

默认 Compose 将容器内 80 端口映射到宿主机 `5173`：

```yaml
ports:
  - "5173:80"
```

如果部署到公网，建议在外层使用 Caddy、Nginx、Traefik 或云厂商负载均衡配置 HTTPS，并将流量转发到 `http://127.0.0.1:5173`。

需要代理的路径：

- `/`：管理台静态资源。
- `/api/`：管理接口。
- `/v1/`：搜索和兼容接口。
- `/mcp` 或 `/v1/mcp`：MCP 接口。
- `/healthz`：健康检查。

## 本地开发

本地开发推荐只用 Docker 启动 PostgreSQL，后端和前端在本机运行。

### 启动 PostgreSQL

```bash
docker run -d --name one-search-postgres \
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

默认开发后端地址通常为：

```text
http://localhost:18080
```

可通过 `.env.development` 或环境变量设置 `DATABASE_URL`、`HTTP_ADDR` 等。

### 启动前端

```bash
cd frontend
npm install
npm run dev
```

默认前端地址：

```text
http://localhost:5173
```

如果前后端分离运行，请设置：

```dotenv
VITE_API_BASE=http://localhost:18080
```

## 文档

README 只保留部署和上手信息，接口细节请查看：

- [管理员 API Key 与管理接口](docs/admin-api-key.md)
- [MCP 接口文档](docs/mcp.md)

常用接口入口：

| 文档 | 内容 |
| --- | --- |
| `docs/admin-api-key.md` | 管理员 API Key、管理接口、搜索接口、兼容接口、用量接口示例。 |
| `docs/mcp.md` | MCP 开启方式、鉴权、工具 schema、Codex / LobeHub / LobeChat 配置示例。 |

## 常见问题

### 启动时报 `POSTGRES_PASSWORD is required`

`.env` 中没有设置 `POSTGRES_PASSWORD`。请设置强密码后重新启动：

```bash
docker compose up --build -d
```

### 启动时报 `ENCRYPTION_KEY is required`

生产环境必须设置 `ENCRYPTION_KEY`，且长度至少 32 字符。示例生成方式：

```bash
openssl rand -base64 32
```

### 登录后没有搜索结果

请检查：

1. 对应 Provider 是否启用。
2. 是否已添加可用上游 API Key。
3. 默认 Provider 是否包含未配置 Key 的平台。
4. **搜索日志** 中的 Provider 调用明细和上游错误信息。

### 调用 `/v1/search` 返回未授权

默认 `API_AUTH_REQUIRED=true`。请先在管理台创建 `osr_` API Token，然后使用：

```http
Authorization: Bearer osr_xxx
```

或：

```http
X-API-Key: osr_xxx
```

### MCP 无法访问

请确认：

1. `.env` 中设置了 `MCP_ENABLED=true`。
2. 使用的路径与 `MCP_PATH` 一致，默认是 `/mcp`。
3. 反向代理已转发 `/mcp` 或 `/v1/mcp`。
4. 如果 `API_AUTH_REQUIRED=true`，请求携带了 `osr_` 或 `oak_` Token。

## License

请根据你的发布计划补充许可证文件和许可证说明。
