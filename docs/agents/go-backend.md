# Go 后端编码契约

> **使用场景**：编写或修改 Go 后端代码时加载。涵盖测试、代码风格、Context、DTO/API、路由命名全部硬约束。
>
> **Skill 前置**：动手前按 [workflow.md](workflow.md) 的「Go 后端 Skill 加载清单」加载 `use-modern-go` / `golang-naming` / `golang-code-style` / `golang-samber-lo` / `golang-samber-mo`。本文件的硬约束**优先级高于**任何 skill 的通用建议。

## 与 `use-modern-go` 建议冲突时的项目硬约束

`use-modern-go` CLI 返回的指南在通用 Go 场景下具有权威性，但下列条目在本项目被更严的约束覆盖，**以本项目为准**：

| CLI 指南 | 本项目约束 |
|---------|-----------|
| `any`：用 `any` 替代 `interface{}` | DTO 层**两者皆禁**，用 `sonic.NoCopyRawMessage` 或具体结构体；由 `lint conv` 的 DTO 规则强制 |
| `errors_join` / `errors_is` | 业务错误的创建与包装统一走 `internal/common/ierr`；禁止 `errors.New` / `fmt.Errorf`。`errors.Is` 仅用于判定标准库或第三方 sentinel |
| `json_omitzero` 等 `encoding/json` 相关 | 全项目统一 `github.com/bytedance/sonic`；禁止 `encoding/json` |
| `http_servemux_patterns` | HTTP 层是 fiber v3 + huma，不使用标准库 `ServeMux`；路由规范见下文 |
| 各类局部常量/字面量改写 | 业务包禁止本地 `const` 块，常量放 `internal/common/constant/` 或 `internal/common/enum/` |

其余指南（`min_max`、`range_over_int`、`slices_*`、`maps_*`、`sync_once_value`、`sync_waitgroup_go`、`testing_t_context`、`testing_b_loop`、`cmp_or`、`atomic_types`、`time_since` 等）无冲突，应直接采纳；`.golangci.yml` 已启用 `modernize` linter 作为兜底拦截（测试目录放宽）。

## 测试契约

- 单元测试目录：`test/unit/<topic>/`；端到端测试目录：`test/e2e/<topic>/`。
- 所有 `*_test.go` 只能放在上述目录；不要放在 `internal/` 或 `test/` 根目录。
- 测试数据放对应目录 `fixtures/*.json`；E2E 请求体放 `fixtures/requests/*.json`；不要在 Go 测试里内联大段 JSON。
- 测试和生产代码统一用 `github.com/bytedance/sonic`；禁止 `encoding/json`、`json.RawMessage`。
- `any` / `interface{}`：DTO 与生产代码禁止（用 `sonic.NoCopyRawMessage` 或具体结构体，由 `lint conv` 的 DTO 规则强制）；测试代码允许作为**函数内局部临时容器**使用（如解码半结构化 JSON 的 `[]any`、`map[string]any`），不得出现在对外 DTO 契约上。
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
/api/web/v1/{resource}[/{action}]    # Web 管理端（session JWT；公开入口注册在 public 子组）
/api/cli/v1/{action}                 # aris CLI（API Key 鉴权）
/api/{provider}/v1/{action}          # LLM 代理路由 (openai/anthropic，API Key 鉴权)
```

### 通用规则

- **资源名使用单数小写**：`/user`、`/session`、`/apikey`、`/endpoint`、`/model`、`/audit`
- **禁止裸尾斜杠**：`POST /endpoint` ✅，`POST /endpoint/` ❌；所有路径必须有明确路径段
- **操作通过 Path 段表达，不依赖 HTTP Method 表达语义差异**（即不使用 `GET /endpoint` + `POST /endpoint` 作为唯一区分）
- **按 ID 的更新/删除/详情走 `?id=` 查询参数**（huma query 绑定），不用 `/{resource}/{id}` 路径段

### 操作映射

| 操作 | Method | Path | 示例 |
|------|--------|------|------|
| 创建 | POST | `/{resource}` | `POST /endpoint` |
| 列表 | GET | `/{resource}/list` | `GET /model/list` |
| 查询详情 | GET | `/{resource}?id={id}` | `GET /session?id=1` |
| 获取当前用户 | GET | `/user/current` | `GET /user/current` |
| 按 ID 更新 | PATCH | `/{resource}?id={id}` | `PATCH /endpoint?id=1` |
| 按 ID 删除 | DELETE | `/{resource}?id={id}` | `DELETE /endpoint?id=1` |
| 特殊动作 | POST/GET | `/{resource}/{action}` | `POST /cron/trigger`、`GET /audit/model/log/list` |

### 已注册资源路由一览

与 `internal/router/` 逐条对齐（`GET /endpoint/list` 已被 `/upstream/list` + `/model/list` 取代，endpoint 组只保留写操作）。

| 资源 | 分组路径 | 操作路径 |
|------|---------|---------|
| Ops | `/`（无鉴权） | `GET /health`、`GET /ready`、`GET /ssehealth`、`GET /install.sh` |
| Token | `/api/web/v1/token` | `POST /refresh` |
| OAuth2 | `/api/web/v1/oauth2` | `GET /login`、`POST /callback` |
| User | `/api/web/v1/user` | `GET /current`、`PATCH /`、`GET /list`、`POST /approve`、`POST /demote`、`DELETE /delete`、`POST /demo`、`POST /demo/restore` |
| APIKey | `/api/web/v1/apikey` | `POST /`、`GET /list`、`DELETE /` |
| Session | `/api/web/v1/session` | `GET /list`、`GET /`、`GET /metadata`、`GET /message/list`、`GET /tool/list`、`DELETE /`、`POST /score`、`DELETE /score`、`GET /option/list` |
| SessionShare | `/api/web/v1/session` | `POST /share`、`GET /share/list`、`DELETE /share` |
| SessionShare（公开） | `/api/web/v1/session` | `GET /share/metadata`、`GET /share/message/list`、`GET /share/tool/list` |
| Endpoint | `/api/web/v1/endpoint` | `POST /`、`PATCH /`、`DELETE /` |
| Model | `/api/web/v1/model` | `GET /list`、`POST /`、`PATCH /`、`DELETE /` |
| Upstream | `/api/web/v1/upstream` | `GET /list` |
| Audit | `/api/web/v1/audit` | `GET /model/log/list`、`GET /model/option/list`、`GET /stats/model/trend`、`GET /stats/request/rate`、`GET /stats/token/throughput`、`GET /stats/token/rate`、`GET /stats/model/usage`、`GET /stats/token/latency` |
| CronAudit | `/api/web/v1/audit` | `GET /cron/log/list`、`GET /cron/option/list` |
| DemoAudit | `/api/web/v1/audit` | `GET /demo/log/list`、`GET /demo/option/list` |
| Cron | `/api/web/v1/cron` | `GET /list`、`PATCH /`、`POST /trigger` |
| Trigger | `/api/web/v1/trigger` | `POST /`、`GET /list`、`PATCH /`、`DELETE /` |
| Metrics | `/api/web/v1/metrics` | `GET /runtime` |
| Dataset | `/api/web/v1/dataset` | `GET /preview`、`GET /export`（SSE）、`GET /sample` |
| Trace | `/api/web/v1/trace` | `GET /list`、`GET /`、`GET /event/list`、`DELETE /` |
| Demo（公开） | `/api/web/v1/demo` | `POST /login`、`GET /status` |
| Demo | `/api/web/v1/demo` | `GET /config`、`PATCH /config`、`GET /sessions/list`、`POST /sessions`、`DELETE /sessions` |
| Client | `/api/cli/v1` | `GET /model/list`、`POST /trace/event`、`GET /aris/client/check` |
| OpenAI | `/api/openai/v1` | `GET /models`、`POST /chat/completions`、`POST /responses` |
| Anthropic | `/api/anthropic/v1` | `GET /models`、`POST /messages`、`POST /messages/count_tokens` |
