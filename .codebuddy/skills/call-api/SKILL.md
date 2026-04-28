---
name: call-api
description: Use when the user asks how to call aris-proxy-api APIs, requests curl examples, wants to test endpoints manually, or needs interactive post-deploy verification.
---

# aris-proxy-api curl 调用指南

## 适用边界

本 skill 只负责生成准确的 HTTP/curl 调用、排查认证参数和做交互式补充验证。生产 bugfix、新需求和部署闭环必须沉淀并运行仓库 E2E；禁止只用 `curl` 证明完成。若响应头或错误信息包含 `X-Trace-Id` / traceId，转入 `cls-log-bugfix`。

## 项目简介

aris-proxy-api 是一个 LLM 代理网关，提供 OpenAI 和 Anthropic 兼容的代理接口，同时包含用户管理、API Key 管理、Session 管理等功能。

## 基础配置

在 shell 中设置变量，后续所有 curl 示例都基于这些变量：

```bash
BASE_URL="https://api.lvlvko.top"

# API Key：用于调用 LLM 代理接口和 Session 接口
API_KEY="$ANTHROPIC_AUTH_TOKEN"

# JWT Token：通过 OAuth2 登录获取，用于用户管理和 API Key 管理操作
JWT_TOKEN="<your-jwt-token>"
```

## 认证方式

项目有两种认证方式：

| 认证方式 | Header | 适用端点 | 获取方式 |
|---------|--------|---------|---------|
| **API Key** | `Authorization: Bearer <key>` | LLM 代理、Session | 通过 `$ANTHROPIC_AUTH_TOKEN` 环境变量获取，或从 `/api/v1/apikey/` 创建 |
| **JWT Token** | `Authorization: Bearer <token>` | 用户管理、API Key 管理 | 通过 OAuth2 登录获取 |

## 响应格式

所有非流式响应统一包装为：

```json
{
  "data": {
    "error": null,           // 成功时为 null，失败时为错误对象
    "xxx": "..."             // 实际数据
  }
}
```

错误时 `data` 可能为 `null`，外层 `error` 字段包含具体错误信息。

---

## 端点详解与 curl 示例

### 1. 健康检查（无需认证）

```bash
# 基础健康检查
curl -s "$BASE_URL/health"

# SSE 健康检查（持续推送心跳）
curl -sN "$BASE_URL/ssehealth"
```

### 2. OAuth2 登录（无需认证，获取 JWT Token）

```bash
# 第一步：获取 OAuth2 授权跳转地址
curl -s "$BASE_URL/api/v1/oauth2/login?platform=github"
# 返回: {"data":{"error":null,"redirectURL":"https://github.com/login/oauth/authorize?..."}}

# 第二步：浏览器中打开 redirectURL，用户授权后得到 code 和 state
# 用 code 和 state 换取 Token
curl -s -X POST "$BASE_URL/api/v1/oauth2/callback" \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "github",
    "code": "<从回调获取的 code>",
    "state": "<从回调获取的 state>"
  }'
# 返回: {"data":{"error":null,"accessToken":"jwt...","refreshToken":"jwt..."}}
```

### 3. Token 刷新（无需认证）

```bash
curl -s -X POST "$BASE_URL/api/v1/token/refresh" \
  -H "Content-Type: application/json" \
  -d '{
    "refreshToken": "<refresh-token>"
  }'
```

### 4. 当前用户信息（JWT 认证）

```bash
curl -s "$BASE_URL/api/v1/user/current" \
  -H "Authorization: Bearer $JWT_TOKEN"
```

### 5. 更新用户信息（JWT 认证）

```bash
curl -s -X PATCH "$BASE_URL/api/v1/user/" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": {
      "name": "新昵称"
    }
  }'
```

### 6. API Key 管理（JWT 认证）

#### 创建 API Key

```bash
curl -s -X POST "$BASE_URL/api/v1/apikey/" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-api-key"
  }'
# 注意：返回的 key 是完整的（仅创建时可见），如 "sk-aris-xxxx"
```

限流：每分钟 20 次。

#### 列出 API Key

```bash
curl -s "$BASE_URL/api/v1/apikey/" \
  -H "Authorization: Bearer $JWT_TOKEN"
# 返回的 key 会被掩码，如 "sk-aris-****"
```

#### 删除 API Key

```bash
curl -s -X DELETE "$BASE_URL/api/v1/apikey/1" \
  -H "Authorization: Bearer $JWT_TOKEN"
# 将 1 替换为实际的 key ID
```

### 7. Session 管理（API Key 认证）

#### 列出 Session

```bash
curl -s "$BASE_URL/api/v1/session/list" \
  -H "Authorization: Bearer $API_KEY"
# 可选分页参数：?page=1&pageSize=20
```

#### 查看 Session 详情

```bash
curl -s "$BASE_URL/api/v1/session/?sessionId=1" \
  -H "Authorization: Bearer $API_KEY"
# 将 1 替换为实际的 Session ID
```

### 8. LLM 代理 — OpenAI 兼容接口（API Key 认证）

#### 列出可用模型

```bash
curl -s "$BASE_URL/api/openai/v1/models" \
  -H "Authorization: Bearer $API_KEY"
```

#### Chat Completions（非流式）

```bash
curl -s -X POST "$BASE_URL/api/openai/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

#### Chat Completions（流式）

用 `curl -N` 禁用输出缓冲，实时查看 SSE 事件：

```bash
curl -sN -X POST "$BASE_URL/api/openai/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
    "stream": true
  }'
```

流式响应格式（Server-Sent Events）：

```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"你好"},"index":0}]}
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"！"},"index":0}]}
data: [DONE]
```

`data: [DONE]` 表示流结束。

#### Responses API（OpenAI 新版接口）

```bash
curl -sN -X POST "$BASE_URL/api/openai/v1/responses" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "input": "Hello!",
    "stream": true
  }'
```

限流：每秒 100 次。

### 9. LLM 代理 — Anthropic 兼容接口（API Key 认证）

#### 列出可用模型

```bash
curl -s "$BASE_URL/api/anthropic/v1/models" \
  -H "Authorization: Bearer $API_KEY"
```

#### Messages（非流式）

```bash
curl -s -X POST "$BASE_URL/api/anthropic/v1/messages" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-opus",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

#### Messages（流式）

```bash
curl -sN -X POST "$BASE_URL/api/anthropic/v1/messages" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-opus",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "stream": true
  }'
```

Anthropic 流式响应事件类型：`message_start`、`content_block_start`、`content_block_delta`、`message_delta`、`message_stop`。每个事件格式为 `event: <type>\ndata: <json>\n\n`。

#### Count Tokens

```bash
curl -s -X POST "$BASE_URL/api/anthropic/v1/messages/count_tokens" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
# 返回: {"data":{"error":null,"input_tokens":8}}
```

限流：`/messages` 每秒 100 次；`/count_tokens` 不限流。

---

## 常见错误处理

### 401 未认证

```json
{
  "data": null,
  "error": {
    "message": "invalid or expired api key"
  }
}
```

**解决**：确认 `$API_KEY` 或 `$JWT_TOKEN` 是否正确设置。JWT Token 过期需通过 `/api/v1/token/refresh` 刷新。

### 429 限流

```json
{
  "error": {
    "message": "rate limit exceeded"
  }
}
```

**解决**：等待一段时间后重试。响应头 `Retry-After` 指示等待秒数。

### 404 模型不存在

```json
{
  "error": {
    "message": "model xxx not found"
  }
}
```

**解决**：先调用 `/models` 接口查看可用模型列表。

### 500 上游错误

```json
{
  "error": {
    "message": "upstream request failed"
  }
}
```

**解决**：检查上游 LLM 服务是否正常，或稍后重试。

---

## 注意事项

1. **API Key 与 JWT Token 的区别**：API Key（`$ANTHROPIC_AUTH_TOKEN`）用于 LLM 代理和 Session 接口；JWT Token 用于用户管理和 API Key 管理。两者不能混用。
2. **流式请求必须用 `curl -N`**：否则 curl 会缓冲输出，无法实时看到 SSE 事件流。
3. **API Key 创建后只在返回中展示一次**：创建 API Key 的响应中包含完整 Key（如 `sk-aris-xxx`），后续列表接口会掩码为 `sk-aris-****`。如果丢失需要重新创建。
4. **JWT Token 有过期时间**：Access Token 默认 12 小时有效，Refresh Token 默认 168 小时。过期后需用 Refresh Token 刷新。
5. **跨协议代理**：OpenAI 接口的 `model` 字段如果对应的是 `provider=anthropic` 的模型，请求会被自动转换为 Anthropic 格式发给上游，再转回 OpenAI 响应格式。反之亦然。所以不需要用户关心后端实际协议。
6. **`curl -s` 建议始终加上**：静默模式，不显示进度信息，只输出响应体。
