# Go 后端编码契约

> **使用场景**：编写或修改 Go 后端代码时加载。涵盖测试、代码风格、Context、DTO/API、路由命名全部硬约束。

## 测试契约

- 单元测试目录：`test/unit/<topic>/`；端到端测试目录：`test/e2e/<topic>/`。
- 所有 `*_test.go` 只能放在上述目录；不要放在 `internal/` 或 `test/` 根目录。
- 测试数据放对应目录 `fixtures/*.json`；E2E 请求体放 `fixtures/requests/*.json`；不要在 Go 测试里内联大段 JSON。
- 测试和生产代码统一用 `github.com/bytedance/sonic`；禁止 `encoding/json`、`json.RawMessage`、`any`、`interface{}`。
- 只用标准库 `testing`；禁止 testify / gomock；禁止用 `time.Sleep` 做同步。

## 代码契约

- 业务错误创建/包装统一走 `internal/common/ierr`；禁止 `fmt.Errorf` 或 `errors.New`。
- 内部链路（usecase/domain service）使用 `ierr.Wrap(sentinel, cause, msg)` 或 `ierr.New(sentinel, msg)` 传递错误。
- Handler 从 error 中提取业务错误：`rsp.Error = ierr.ToBizError(err, ierr.ErrXxx.BizError())`，然后 `return util.WrapHTTPResponse(rsp, nil)`。
- Handler 保持薄封装：`return util.WrapHTTPResponse(h.uc.Method(ctx, req))` 或流式直接透传 `*huma.StreamResponse`。
- 日志使用 `logger.WithCtx(ctx)` 或 `logger.WithFCtx(c)`；消息前缀为 `[PascalCaseModule]`；key/token/secret/password 必须用 `util.MaskSecret()`。
- 业务包禁止建 `common.go` 工具堆场；导出公共 helper 放 `internal/util/` 或 `internal/common/`。
- Redis key、存储路径、ID 格式、Data URL 模板等字符串模板放 `internal/common/constant/string.go`。
- 业务包禁止定义本地 `const` 块；使用 `internal/common/constant/`、`internal/common/enum/`。
- HTTP 状态码使用 `fiber.StatusXxx`，禁止裸数字。
- DTO 时间字段用 `time.Time`；禁止 Service 层提前格式化为字符串。
- DTO 包禁止导入 `internal/infrastructure/database/model`；需要数据库字段时将具体字段作为参数传入。

## Context 契约

- handler/service/proxy/converter/dto 必须从调用方接收 `context.Context`。
- 上述层禁止自行创建 `context.Background()` 或 `context.TODO()`。
- 允许根 context 的场景：启动/基础设施初始化、cron 入口并注入 trace ID、agent 初始化、`util.CopyContextValues`。
- 异步协程池任务必须使用 `util.CopyContextValues(ctx)`，禁止直接持有原始请求 context。
- 读取上下文值优先用 `util.CtxValueString()` / `util.CtxValueUint()`，禁止直接类型断言。
- 新 context key 必须注册到 `internal/common/constant/ctx.go`。

## DTO 与 API 契约

- 修改 OpenAI 或 Anthropic DTO 前，先看 `/docs` 的 OpenAPI 文档，保持协议兼容。
- OpenAI 和 Anthropic 接口支持跨 provider 转换；改 DTO 常需同步 usecase、proxy、converter、SSE 合并/归一化工具。
- Huma 安全方案：用户路由用 `jwtAuth`，LLM 代理路由用 `apiKeyAuth`。

## API 路由命名规范

### 分层结构

```
/api/v1/{resource}[/{action}]
/api/{provider}/v1/{action}      # LLM 代理路由 (openai/anthropic)
```

### 通用规则

- **资源名使用单数小写**：`/user`、`/session`、`/apikey`、`/endpoint`、`/model`、`/audit`
- **禁止裸尾斜杠**：`POST /endpoint` ✅，`POST /endpoint/` ❌；所有路径必须有明确路径段
- **操作通过 Path 段表达，不依赖 HTTP Method 表达语义差异**（即不使用 `GET /endpoint` + `POST /endpoint` 作为唯一区分）

### 操作映射

| 操作 | Method | Path | 示例 |
|------|--------|------|------|
| 创建 | POST | `/{resource}` | `POST /endpoint` |
| 列表 | GET | `/{resource}/list` | `GET /endpoint/list` |
| 查询详情 | GET | `/{resource}` | `GET /session?sessionId=1` |
| 获取当前用户 | GET | `/user/current` | `GET /user/current` |
| 按 ID 更新 | PATCH | `/{resource}/{id}` | `PATCH /endpoint/{id}` |
| 按 ID 删除 | DELETE | `/{resource}/{id}` | `DELETE /endpoint/{id}` |
| 特殊动作 | POST/GET | `/{resource}/{action}` | `POST /token`、`GET /audit/logs` |

### 已注册资源路由一览

| 资源 | 分组路径 | 操作路径 |
|------|---------|---------|
| Health | `/health`, `/ssehealth` | `GET /health`, `GET /ssehealth` |
| Token | `/api/v1/token` | `POST /` |
| OAuth2 | `/api/v1/oauth2` | `GET /{provider}/login`, `POST /{provider}/callback` |
| User | `/api/v1/user` | `GET /current`, `PATCH /`, `GET /{userID}` |
| APIKey | `/api/v1/apikey` | `POST /`, `GET /list`, `DELETE /{id}` |
| Session | `/api/v1/session` | `GET /list`, `GET /` |
| Endpoint | `/api/v1/endpoint` | `POST /`, `GET /list`, `PATCH /{id}`, `DELETE /{id}` |
| Model | `/api/v1/model` | `POST /`, `GET /list`, `PATCH /{id}`, `DELETE /{id}` |
| Audit | `/api/v1/audit` | `GET /logs` |
| OpenAI | `/api/openai/v1` | `POST /chat/completions` |
| Anthropic | `/api/anthropic/v1` | `POST /messages`, `POST /messages/count_tokens` |
