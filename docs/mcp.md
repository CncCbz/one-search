# MCP 接口文档

One Search Relay 支持可选 MCP Streamable HTTP / HTTP JSON-RPC 接口，用于让 MCP 客户端通过统一的 `search` 工具调用已配置的搜索 Provider。

## 1. 是否已经内置 MCP

当前项目已补充 MCP 支持。实现位置：

- 路由启用：`backend/internal/api/handlers.go`
- MCP JSON-RPC Handler：`backend/internal/api/mcp.go`
- 环境变量配置：`backend/internal/config/config.go`
- Docker Compose 环境变量：`docker-compose.yml`
- all-in-one 容器启动脚本：`deploy/all-in-one-entrypoint.sh`
- Nginx 代理：`deploy/nginx.conf`

## 2. 开关配置

默认关闭 MCP：

```dotenv
MCP_ENABLED=false
MCP_PATH=/mcp
```

开启：

```dotenv
MCP_ENABLED=true
MCP_PATH=/mcp
```

说明：

- `MCP_ENABLED=false` 时不会挂载 MCP 路由。
- `MCP_ENABLED=true` 时挂载 `GET <MCP_PATH>` 和 `POST <MCP_PATH>`。
- 默认 all-in-one Nginx 已代理 `/mcp`。如果把 `MCP_PATH` 改成其它路径，需要同步修改反向代理配置。

## 3. 传输和鉴权

当前实现为 Streamable HTTP 兼容的 HTTP JSON-RPC MCP 接口。客户端通过同一个端点发送 JSON-RPC 请求；服务端当前直接返回 `application/json`，不主动建立 SSE 流：

```text
POST /mcp
Content-Type: application/json
```

`GET /mcp` 默认返回接口元信息，便于浏览器或 curl 探测当前是否启用；如果客户端带 `Accept: text/event-stream` 试图建立 SSE 流，服务端会返回 `405 Method Not Allowed`。

鉴权复用搜索 API：

- 当 `API_AUTH_REQUIRED=true` 时，必须传入外部 API Token 或管理员 API Key。
- 当 `API_AUTH_REQUIRED=false` 时，MCP 工具调用不要求鉴权。

支持两种请求头：

```http
Authorization: Bearer osr_xxx
```

或：

```http
X-API-Key: osr_xxx
```

管理员 API Key 也可以使用：

```http
Authorization: Bearer oak_xxx
```

普通 `osr_` API Token 的 `allowed_providers`、RPM 限制会继续生效；管理员 API Key 不受 `allowed_providers` 限制。

## 4. 支持的方法

| JSON-RPC 方法 | 说明 |
| --- | --- |
| `initialize` | 返回协议版本、服务信息和能力。 |
| `ping` | 健康探测，返回空对象。 |
| `tools/list` | 返回可用工具列表。 |
| `tools/call` | 调用工具。当前支持 `search`。 |
| `resources/list` | 返回空资源列表。 |
| `prompts/list` | 返回空提示词列表。 |

通知请求，即没有 `id` 的 JSON-RPC 请求，会返回 `204 No Content`。

## 5. 工具：`search`

`search` 工具会调用后端同一套 `search.Orchestrator`，因此 Provider、缓存、日志、用量统计、Token 限制与 `/v1/search` 保持一致。

输入参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `query` | string | 是 | 搜索关键词。 |
| `providers` | string[] | 否 | 限定 Provider：`exa`、`you`、`jina`、`tavily`、`firecrawl`、`serper`、`brave`。为空时使用系统默认配置；新库初始化和默认 fallback 为这七个内置 Provider。 |
| `mode` | string | 否 | `parallel`、`fallback`、`single`。 |
| `limit` | number | 否 | 返回结果数，后端最大限制 50。 |
| `freshness` | string | 否 | 预留给 Provider 或兼容逻辑的时间新鲜度提示。 |
| `dedupe` | boolean | 否 | 是否按 URL 去重。 |
| `cache` | string | 否 | `default`、`bypass`、`refresh`。 |
| `include_raw` | boolean | 否 | 是否在结果中包含上游原始条目。 |

返回结果：

- `content`：MCP 文本内容，包含格式化后的搜索响应 JSON。
- `structuredContent`：结构化搜索响应，格式与 `/v1/search` 的 `SearchResponse` 一致。
- `isError`：工具执行是否失败。

## 6. 调用示例

以下示例假设：

```bash
export BASE_URL=http://localhost:5173
export API_TOKEN=osr_xxx
```

### 6.1 查看 MCP 元信息

```bash
curl "$BASE_URL/mcp"
```

响应示例：

```json
{
  "auth": "Authorization: Bearer <osr_...|oak_...> or X-API-Key",
  "enabled": true,
  "endpoint": "/mcp",
  "protocol_version": "2025-03-26",
  "tools": ["search"],
  "transport": "http-json-rpc"
}
```

### 6.2 初始化

```bash
curl -X POST "$BASE_URL/mcp" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-03-26",
      "capabilities": {},
      "clientInfo": {"name": "curl", "version": "1.0"}
    }
  }'
```

### 6.3 查看工具列表

```bash
curl -X POST "$BASE_URL/mcp" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }'
```

### 6.4 调用搜索工具

```bash
curl -X POST "$BASE_URL/mcp" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "search",
      "arguments": {
        "query": "latest web search APIs",
        "providers": ["exa", "you", "jina", "tavily", "firecrawl", "serper", "brave"],
        "mode": "parallel",
        "limit": 5,
        "cache": "default"
      }
    }
  }'
```

### 6.5 使用管理员 API Key 调用

```bash
curl -X POST "$BASE_URL/mcp" \
  -H "Authorization: Bearer oak_xxx" \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 4,
    "method": "tools/call",
    "params": {
      "name": "search",
      "arguments": {
        "query": "one search relay",
        "limit": 3,
        "include_raw": false
      }
    }
  }'
```

## 7. 错误格式

MCP 接口使用 JSON-RPC 错误格式：

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "query is required"
  }
}
```

常见错误：

| code | 场景 |
| --- | --- |
| `-32700` | 请求体为空或 JSON 解析失败。 |
| `-32600` | JSON-RPC 请求格式错误。 |
| `-32601` | 方法不存在。 |
| `-32602` | 工具参数错误。 |
| `-32001` | 缺少或无效 Token。 |
| `-32003` | 普通 API Token 请求了未授权 Provider。 |

工具执行期间如果搜索链路返回错误，会以工具结果形式返回：

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [{"type": "text", "text": "...错误信息..."}],
    "isError": true
  }
}
```

## 8. 在 Codex 中配置 MCP

Codex CLI / Codex IDE 扩展共用 `config.toml`。常见位置：

- 用户级配置：`~/.codex/config.toml`
- 项目级配置：当前仓库下的 `.codex/config.toml`，仅建议用于可信项目，不要把真实 Token 提交进仓库。

Codex 对 MCP 服务使用 TOML 表：`[mcp_servers.<server-name>]`。One Search Relay 是远程 HTTP MCP 服务，因此推荐直接编辑 `config.toml`，而不是使用主要面向 stdio 服务的 `codex mcp add -- <command>` 形式。

### 8.1 推荐配置：用环境变量保存 Token

先在 shell 中设置 Token：

```bash
export ONE_SEARCH_API_TOKEN=osr_xxx
# 或使用管理员 API Key：
# export ONE_SEARCH_API_TOKEN=oak_xxx
```

编辑 `~/.codex/config.toml`：

```toml
[mcp_servers.one_search]
url = "http://localhost:5173/mcp"
bearer_token_env_var = "ONE_SEARCH_API_TOKEN"
enabled = true
startup_timeout_sec = 10
tool_timeout_sec = 60

# 可选：只暴露 search 工具
enabled_tools = ["search"]
```

说明：

- `url` 指向启用后的 MCP 端点。Docker Compose / all-in-one 默认是 `http://localhost:5173/mcp`。
- `bearer_token_env_var` 让 Codex 从本地环境变量读取 Token，并自动发送 `Authorization: Bearer <token>`。
- `tool_timeout_sec` 建议大于后端 `request_timeout_ms`，否则 Codex 可能先超时。
- 普通 `osr_` Token 会受 `allowed_providers`、RPM、日/月额度限制；`oak_` 管理员 API Key 拥有完整权限。

### 8.2 直接写请求头的配置

如果只是本机临时测试，也可以直接写 HTTP Header：

```toml
[mcp_servers.one_search]
url = "http://localhost:5173/mcp"
http_headers = { "Authorization" = "Bearer osr_xxx" }
enabled = true
tool_timeout_sec = 60
enabled_tools = ["search"]
```

不建议把这种配置提交到项目仓库，因为会泄露 Token。

### 8.3 使用 `X-API-Key` 请求头

如果你的环境更偏向 API Key Header，也可以这样配置：

```toml
[mcp_servers.one_search]
url = "http://localhost:5173/mcp"
http_headers = { "X-API-Key" = "osr_xxx" }
enabled = true
tool_timeout_sec = 60
```

### 8.4 在 Codex 中检查是否生效

启动 Codex：

```bash
codex
```

在 TUI 中输入：

```text
/mcp
```

应能看到 `one_search` MCP 服务和 `search` 工具。随后可以在对话中让 Codex 使用该工具，例如：

```text
使用 one_search 的 search 工具搜索 “latest web search APIs”，返回 5 条结果。
```

如果没有出现：

1. 确认 One Search Relay 已启用 MCP：`MCP_ENABLED=true`。
2. 确认服务可访问：`curl http://localhost:5173/mcp`。
3. 确认 `ONE_SEARCH_API_TOKEN` 在启动 Codex 的同一个 shell 中存在。
4. 确认 `~/.codex/config.toml` 使用的是 `[mcp_servers.one_search]`，不是 Claude Desktop 风格的 `mcpServers`。
5. 重新启动 Codex。

### 8.5 项目级配置示例

如果希望这个仓库自带 Codex MCP 配置模板，可以新建 `.codex/config.toml`：

```toml
[mcp_servers.one_search]
url = "http://localhost:5173/mcp"
bearer_token_env_var = "ONE_SEARCH_API_TOKEN"
enabled = true
tool_timeout_sec = 60
enabled_tools = ["search"]
```

项目级配置只引用环境变量，不要写入真实 `osr_` 或 `oak_` Token。

## 9. 在 LobeHub / LobeChat 中配置 MCP

LobeHub 的自定义 MCP 支持 Streamable HTTP。配置时建议这样填：

| 字段 | 建议值 |
| --- | --- |
| MCP name | `one-search` |
| Connection type | `Streamable HTTP` / `HTTP` |
| Endpoint URL | `http://<One Search 可访问地址>:5173/mcp` |
| Auth type | `API Key`，会作为 Bearer Token 发送 |
| API Key | `osr_xxx` 或 `oak_xxx` |

如果使用高级 HTTP Headers，也可以不选 API Key，而手动填：

```json
{
  "Authorization": "Bearer osr_xxx"
}
```

或：

```json
{
  "X-API-Key": "osr_xxx"
}
```

### 9.1 LobeHub 中最容易踩的地址问题

LobeHub 获取 Manifest 时，通常不是浏览器直接访问 MCP，而是由 LobeHub 后端、Electron 主进程或容器里的服务端去访问你的 MCP 地址。因此：

- 如果 LobeHub 是云端版，`http://localhost:5173/mcp` 一定不可用，因为 localhost 指向 LobeHub 服务器自己。需要给 One Search 暴露公网 HTTPS 地址。
- 如果 LobeHub 是 Docker 部署，`http://127.0.0.1:5173/mcp` / `http://localhost:5173/mcp` 通常指向 LobeHub 容器内部，不是宿主机。可以尝试：
  - Docker Desktop：`http://host.docker.internal:5173/mcp`
  - Linux Docker：宿主机 LAN IP，例如 `http://192.168.1.10:5173/mcp`
  - 同一个 compose 网络：使用服务名，例如 `http://one-search:80/mcp`
- 如果 One Search 在反向代理后面，Endpoint URL 必须使用 LobeHub 服务端能访问到的代理地址。

当前服务同时兼容以下路径，优先推荐 `/mcp`：

```text
/mcp
/mcp/
/v1/mcp
/v1/mcp/
```

### 9.2 排查 “获取 Manifest 失败 / Error POSTing to endpoint”

先在 **运行 LobeHub 的同一台机器或同一个容器网络内** 测试，而不是只在浏览器所在机器测试。

1. 确认 One Search 已启用 MCP：

```dotenv
MCP_ENABLED=true
MCP_PATH=/mcp
```

Docker Compose 修改 `.env` 后需要重建或重启：

```bash
docker compose up -d --build
```

2. 测试 GET 元信息：

```bash
curl -i http://localhost:5173/mcp
```

应返回 `200` 和包含 `tools:["search"]` 的 JSON。

3. 用 Streamable HTTP 客户端常用请求头测试初始化：

```bash
export BASE_URL=http://localhost:5173
export API_TOKEN=osr_xxx

curl -i -X POST "$BASE_URL/mcp" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-03-26",
      "capabilities": {},
      "clientInfo": {"name": "lobehub-test", "version": "1.0"}
    }
  }'
```

应返回 `200`，并包含 `result.capabilities.tools`。

4. 测试 initialized 通知。这个请求没有 `id`，按 Streamable HTTP 规范应返回 `202 Accepted`：

```bash
curl -i -X POST "$BASE_URL/mcp" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{
    "jsonrpc": "2.0",
    "method": "notifications/initialized"
  }'
```

5. 测试工具列表：

```bash
curl -i -X POST "$BASE_URL/mcp" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }'
```

应返回 `search` 工具。

6. 如果返回 `401`：

- LobeHub 的 Auth type 请选择 `API Key`，并填写 `osr_` 或 `oak_` Token。
- 或在高级 Headers 中填写 `Authorization: Bearer osr_xxx`。
- 如果只是临时内网测试，也可以把 One Search 的 `API_AUTH_REQUIRED=false`，但不建议公网这样配置。

7. 如果返回 `404` / `405` / HTML：

- 检查是否访问到了前端 SPA，而不是后端 MCP 路由。
- all-in-one 部署请确认 Nginx 已包含 `/mcp` 代理配置，并重建镜像。
- 不确定时可使用 `/v1/mcp` 作为兼容路径再试。

8. 如果是 `fetch failed` 或只有 `Error POSTing to endpoint` 没有 HTTP 状态：

- 基本是 LobeHub 运行环境无法连到该 URL。
- 把 `localhost` 换成 LobeHub 容器/服务器能访问到的 IP、服务名或公网域名。

## 10. 其它客户端配置示例

不同 MCP 客户端对远程 HTTP MCP 的配置字段略有差异，通用要点是：

```json
{
  "url": "http://localhost:5173/mcp",
  "headers": {
    "Authorization": "Bearer osr_xxx"
  }
}
```

如果客户端只支持 stdio MCP，需要额外使用支持 HTTP/SSE/Streamable HTTP 转发的适配器。
