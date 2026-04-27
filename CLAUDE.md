# CLAUDE.md

## 项目形态

- 这是一个 Go 1.25.1 后端，用于 LLM 代理网关和用户/会话管理。
- 入口是 `main.go` -> `cmd.Execute()` -> `cmd/server.go` 的 `server start`。
- 运行时加载顺序是：database、Redis、共享 HTTP Client、Pond 协程池、cron、Fiber 中间件、可选 `/docs`、API 路由。
- 请求链路是 Fiber 中间件 -> Huma 路由/中间件 -> handler -> service -> DAO/proxy/converter。
- LLM 代理分层严格：`service` 编排端点查找、转换、代理和存储，`proxy` 只负责 HTTP/SSE 传输，`converter` 只负责 DTO 映射。
- 模型路由和代理 API Key 是数据库驱动的，存放在 `ModelEndpoint` 和 `ProxyAPIKey`；环境配置来自 Viper 和 `env/api.env`。

## 常用命令

- 安装依赖：`go mod download`
- 本地运行：`go run main.go server start --host localhost --port 8080`
- 数据库迁移：`go run main.go database migrate`
- 创建对象存储桶：`go run main.go object bucket create`
- 完整本地栈：创建 `postgresql-data`、`redis-data`、`minio-data` 卷后执行 `docker compose -f docker/docker-compose-full.yml up -d`
- 构建：`make build`
- 调试构建：`make build-dev` 或 `make build-debug`
- 自定义规范扫描：`make lint-conv`
- 全量测试：`make test` 或 `go test -count=1 ./...`
- 聚焦测试：`go test -v -count=1 -run TestFunctionName ./test/unit/<topic>/` 或 `./test/e2e/<topic>/`

## 工作流

- 如果是普通需求开发，先和用户确认边界与预期行为，不要直接进入 `cls-log-bugfix`。
- 如果是 bugfix、线上错误、traceID、日志排查，先启动 `cls-log-bugfix`，在 `ap-guangzhou` 查 CLS 日志，再用 `X-Trace-Id` / traceID 追全链路。
- 如果可用，使用 superpowers 流程：和用户头脑风暴、锁定范围、评审测试用例、制定小步计划、逐步修复。
- 每次改动后，先补或更新回归/单测，再跑聚焦测试、`make lint-conv`、`go test -count=1 ./...`。
- 端到端用例**必须**沉淀到代码仓库，放 `test/e2e/<topic>/` 并按下文 E2E 工程骨架维护，测试通过后再提交并推送；**不允许**只用 `curl` 跑完就算闭环。
- 测试和 lint 都通过后，若用户要求完整流程或部署，再提交并推送；本仓库 pre-commit 会执行 `gofmt -w`、`go mod tidy`、`go vet ./...`、`go test -count=1 ./...`、`script/lint-conventions.sh`。
- 正式发布使用 `deploy-to-production`：推送到 `master`，等待 `docker-publish.yml` 镜像构建完成，再在生产机执行部署脚本。
- 部署后**先跑** `test/e2e/<topic>/` 的 Go 用例（`BASE_URL=https://api.lvlvko.top API_KEY=$ANTHROPIC_AUTH_TOKEN go test -v -count=1 ./test/e2e/<topic>/`），而不是只 `curl` 一下；如需交互式补充验证再用 `call-api` skill。
- 如果 E2E 失败，取响应头 `X-Trace-Id`，回到 CLS 排障步骤；重复 1~6 直到需求或 bugfix 完成。

## 测试规则

- 单元测试：`test/unit/<topic>/`
- 端到端测试：`test/e2e/<topic>/`
- 所有 `*_test.go` 只能放在 `test/unit/<topic>/` 或 `test/e2e/<topic>/` 下；不要放在 `internal/`，也不要直接放在 `test/` 根目录。
- 测试数据必须放在对应目录的 `fixtures/*.json`；不要在 Go 测试代码里内联构造数据。
- 测试和生产代码都用 `github.com/bytedance/sonic`；禁止 `encoding/json`、`json.RawMessage`、`any`、`interface{}`。
- 只用标准库 `testing`；禁止 testify / gomock。
- 禁止用 `time.Sleep` 做同步。
- bugfix 必须带回归测试，覆盖触发场景。

## E2E 工程骨架（test/e2e/\<topic\>/）

每一个生产 bugfix / 新需求都要沉淀一条 E2E 用例，按下面的骨架产出：

- **目录**：`test/e2e/<topic>/<topic>_test.go` + `test/e2e/<topic>/fixtures/requests/<case>.json`。一个 topic 下允许多个 case 文件，复用同一 `<topic>_test.go`。
- **Skip 机制（硬性要求）**：测试入口必须读取 `BASE_URL` 和 `API_KEY` 两个环境变量；任一为空直接 `t.Skip("BASE_URL and API_KEY are required for e2e test")`。这样默认 `make test` / `go test ./...` 不会打生产；CI 和 pre-commit 都默认走 skip 路径。
- **触发方式**：手工触发生产回归时用 `BASE_URL=https://api.lvlvko.top API_KEY=$ANTHROPIC_AUTH_TOKEN go test -v -count=1 ./test/e2e/<topic>/`；**禁止**把线上密钥写进代码、`.env` 或 CI 配置。
- **HTTP 客户端**：用标准库 `net/http`，但**禁止使用 `http.DefaultClient`**（默认无超时，流式响应可能永远挂住）。必须显式构造带 `Timeout` 的 `*http.Client`（e2e 常用 60~90s 总超时），参考 `openai_chat_completion_test.go` 的 `newE2EClient`。
- **断言原则**：
  - 非流式接口：断言 HTTP 200 + 响应 JSON 关键字段（`id`、`model`、`choices`、`usage` 等）存在。
  - 流式接口：断言 HTTP 200 + `Content-Type: text/event-stream` + 存在 `X-Trace-Id` 响应头 + **必须**读到至少一条**携带实质内容**的 delta（`choices[].delta.content` 或 `choices[].delta.reasoning_content` 非空），才算证明链路健康；**只读到空壳 role chunk 就退出不算通过**（极端情况下上游可能先发 role 再 500）。读到实质 delta 立即 break，配合流读 deadline（常用 60s）避免走满整段生成。
  - 不要对模型输出的**语义**做强断言（易 flaky），只断言通路和首个实质 token 的结构。
- **回归用例命名**：bugfix 用例文件名和测试函数名要能直接描述 bug 场景，例如 `kimi_thinking_missing_reasoning_stream.json` + `TestChatCompletion_KimiThinking_MissingReasoningContent_Stream`，并在测试注释里记录原始 trace / 错误片段，方便后人回溯。
- **失败处理**：E2E 失败时从响应头拿 `X-Trace-Id`，回到 `cls-log-bugfix` 流程排障，不要盲目重跑。

## 代码约束

- 业务代码错误创建/包装统一走 `internal/common/ierr`；禁止 `fmt.Errorf` 或 `errors.New`。
- Service 正常返回 `rsp, nil`；业务失败写入 `rsp.Error = ierr.ErrXxx.BizError()`。
- Handler 保持薄封装：`return util.WrapHTTPResponse(h.svc.Method(ctx, req))`。
- 日志必须用 `logger.WithCtx(ctx)` 或 `logger.WithFCtx(c)`，消息前缀 `[PascalCaseModule]`，并且对 key/token/secret/password 使用 `util.MaskSecret()`。
- 业务包禁止建 `common.go` 式的工具堆场；导出公共 helper 只放 `internal/util/` 或 `internal/common/`。
- Redis key、存储路径、ID 格式、Data URL 模板等字符串模板，统一放 `internal/common/constant/string.go`。
- 业务包禁止定义本地 `const` 块；应使用 `internal/common/constant/`、`internal/common/enum/`、`internal/enum/`。
- HTTP 状态码使用 `fiber.StatusXxx`，禁止裸数字。
- DTO 时间字段用 `time.Time`；禁止在 Service 层提前格式化为字符串。

## Context 规则

- handler/service/proxy/converter/dto 必须从调用方接收 `context.Context`；禁止在这些层自行创建 `context.Background()` 或 `context.TODO()`。
- 允许的根 context 场景是：启动/基础设施初始化、cron 入口并注入 trace ID、agent 初始化、`util.CopyContextValues`。
- 异步协程池任务必须用 `util.CopyContextValues(ctx)`，禁止直接用原始请求 context。
- 读取上下文值优先用 `util.CtxValueString()` / `util.CtxValueUint()`，禁止直接类型断言。
- 新 context key 必须注册到 `internal/common/constant/ctx.go`。

## DTO 与 API 注意

- 修改 OpenAI 或 Anthropic DTO 前，必须先看 `/docs` 的 OpenAPI 文档，保持协议兼容。
- OpenAI 和 Anthropic 接口支持跨 provider 转换；改 DTO 常需要同步更新 service、proxy、converter、SSE 合并/归一化工具。
- Huma 安全方案是用户路由用 `jwtAuth`，LLM 代理路由用 `apiKeyAuth`。

## CI 与部署

- `.github/workflows/docker-publish.yml` 会在推送到 `master`、`v*.*.*` tag、PR 到 `master`，以及定时触发时构建多架构 GHCR 镜像；path filter 包含 `internal/**`、`docker/**`、`cmd/**`、`main.go`、`go.mod`、`go.sum`。
- 仓库本地 hook 可通过 `bash .githooks/setup.sh` 安装；除非明确指示，不要绕过。
- 使用 `.worktrees/` 作为 git worktree 目录。

## 已有文档

- `AGENTS.md`，`CLAUDE.md` 和 `CODEBUDDY.md` 是本仓库长文规范的源头；当用户新增或修改持久规范时，保持它们同步。
- 若文档与可执行源冲突，优先信任 `Makefile`、脚本、workflow、hook。
