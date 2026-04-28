# Header 透传设计

## 目标

客户端请求到达 aris-proxy-api 时携带的 HTTP 头（除代理自身覆盖的头外），在转发到上游 LLM 提供者时透传过去。

## 现状

`internal/infrastructure/transport/openai.go` 和 `internal/infrastructure/transport/anthropic.go` 的 `sendRequest` 创建上游 HTTP 请求时只设置了 `Content-Type`、`Authorization`/`x-api-key`、`anthropic-version`，**完全不透传客户端请求头**。

## 方案

### 数据流

```
客户端请求 → Fiber → Huma Middleware (apikey.go)
  → ctx.EachHeader() 枚举所有请求头
  → 过滤排除列表，存入 context (CtxKeyPassthroughHeaders)
  → Handler → UseCase → Transport (openai.go / anthropic.go)
  → 从 context 读取头，设置到上游 *http.Request
  → 再覆盖代理自身管理的头（Content-Type、Authorization 等）
```

### 排除的 Header（不透传）

| 原因 | Header |
|---|---|
| 代理自身覆盖 | `Content-Type`、`Authorization`、`x-api-key`、`anthropic-version` |
| Go HTTP 客户端自动处理 | `Content-Length`、`Host` |
| HTTP 逐跳头标准 | `Connection`、`Transfer-Encoding`、`Upgrade`、`Proxy-Authorization`、`Proxy-Authenticate`、`TE`、`Trailer` |
| 内部链路，不与上游混淆 | `X-Trace-Id` |

### 存储格式

`context.WithValue(ctx, CtxKeyPassthroughHeaders, map[string]string)` — 单值 map。同名的多个头只取第一个值，因为上游代理场景下罕见且单值简化处理。

### 变更清单

1. **`internal/common/constant/ctx.go`** — 新增 `CtxKeyPassthroughHeaders`
2. **`internal/middleware/apikey.go`** — 在认证成功后，用 `ctx.EachHeader()` 枚举、过滤、存入 context
3. **`internal/infrastructure/transport/openai.go`** — `sendRequest` 和 `sendResponseRequest` 读取 context 设置到 `req.Header`，然后覆盖代理自有头
4. **`internal/infrastructure/transport/anthropic.go`** — 同上
5. **单元测试** — `test/unit/header_passthrough/` 测试过滤逻辑
6. **E2E 测试** — 在 `test/e2e/` 下新增，验证自定义头透传

### 不涉及

- 不修改 `UpstreamEndpoint` 结构体
- 不修改接口签名（OpenAIProxy / AnthropicProxy 接口不变）
- 不引入数据库字段或配置文件
- 不作逐头值校验或转换（原样透传）
