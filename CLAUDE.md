# CLAUDE.md

## 0. Meta Prompt 合约

- **角色**：作为 `aris-proxy-api` 的 Go 后端结对工程师，优先交付可运行、可验证、可维护的最小改动。
- **目标**：先理解请求属于需求、bug、API 调用、部署还是文档维护；选择对应流程后再动手。
- **上下文**：以现有代码、`Makefile`、脚本、workflow、hook 为事实源；文档与可执行源冲突时信任可执行源。
- **执行循环**：分类任务 → 加载必要 skill → 阅读相关代码/文档 → 小步计划 → 最小修改 → 聚焦验证 → 汇报证据。
- **边界**：不为普通需求默认走线上日志排障；不为手工 `curl` 结果跳过仓库测试；不绕过 hook 或安全规则。
- **输出**：简短说明做了什么、验证了什么、还有什么未验证；引用文件路径和命令必须精确。

## 1. Skill 路由

- **生产 bug / 线上错误 / traceId / `X-Trace-Id` / CLS / E2E 失败**：使用 `cls-log-bugfix`，在 `ap-guangzhou` 查日志并按 trace 追链路。
- **API 调用 / curl 示例 / 手工接口验证**：使用 `call-api`；它只负责交互式调用示例，不替代 E2E 回归。
- **发布 / 部署 / 生产验证**：使用 `deploy-to-production`；提交、推送、CI、SSH 部署和线上 E2E 都在该 skill 中维护。
- 专项流程细节放在对应 skill，主文档只保留触发条件和项目级硬约束。

## 2. 项目模型

- Go `1.25.1` 后端，提供 LLM 代理网关、用户、API Key、会话管理。
- 入口：`main.go` → `cmd.Execute()` → `cmd/server.go` 的 `server start`。
- 启动链路：database、Redis、共享 HTTP Client、Pond 协程池、cron、Fiber 中间件、可选 `/docs`、API 路由。
- 请求链路：Fiber 中间件 → Huma 路由/中间件 → handler → service → DAO/proxy/converter。
- LLM 代理分层：`service` 编排端点查找、转换、代理和存储；`proxy` 只做 HTTP/SSE 传输；`converter` 只做 DTO 映射。
- 模型路由和代理 Key 由数据库驱动：`ModelEndpoint`、`ProxyAPIKey`；运行配置来自 Viper 和 `env/api.env`。

## 3. 常用命令

- 安装依赖：`go mod download`
- 本地运行：`go run main.go server start --host localhost --port 8080`
- 数据库迁移：`go run main.go database migrate`
- 创建对象存储桶：`go run main.go object bucket create`
- 完整本地栈：先创建 `postgresql-data`、`redis-data`、`minio-data` 卷，再执行 `docker compose -f docker/docker-compose-full.yml up -d`
- 构建：`make build`；调试构建：`make build-dev` 或 `make build-debug`
- 自定义规范扫描：`make lint-conv`
- 全量测试：`make test` 或 `go test -count=1 ./...`
- 聚焦测试：`go test -v -count=1 -run TestFunctionName ./test/unit/<topic>/` 或 `./test/e2e/<topic>/`

## 4. 开发工作流

- 需求不清时先说明假设并推进；只有边界会影响实现时才向用户确认。
- 如果是 bugfix、线上错误、traceID、日志排查，先启动 `cls-log-bugfix`，在 `ap-guangzhou` 查 CLS 日志，再用 `X-Trace-Id` / traceID 追全链路。
- 修改前先定位相关 handler/service/proxy/converter/DAO/DTO，不做大范围重写。
- 新需求和 bugfix 都应先补或更新测试；bugfix 必须有能复现问题的回归用例。
- 每次改动后依次跑：聚焦测试 → `make lint-conv` → 必要时 `go test -count=1 ./...`。
- 端到端用例**必须**沉淀到代码仓库，放 `test/e2e/<topic>/` 并按下文 E2E 工程骨架维护，测试通过后再提交并推送；**不允许**只用 `curl` 跑完就算闭环。
- 测试和 lint 通过后，只有用户明确要求提交、推送或部署时才执行 git 提交/发布流程。
- 正式发布使用 `deploy-to-production`：推送到 `master`，等待 `docker-publish.yml` 镜像构建完成，再在生产机执行部署脚本。
- 部署后**先跑** `test/e2e/<topic>/` 的 Go 用例，而不是只 `curl` 一下；如需交互式补充验证再用 `call-api` skill。
- 如果 E2E 失败，取响应头 `X-Trace-Id`，回到 CLS 排障步骤；重复直到需求或 bugfix 完成。

## 5. 测试契约

- 单元测试目录：`test/unit/<topic>/`；端到端测试目录：`test/e2e/<topic>/`。
- 所有 `*_test.go` 只能放在上述目录；不要放在 `internal/` 或 `test/` 根目录。
- 测试数据放对应目录 `fixtures/*.json`；E2E 请求体放 `fixtures/requests/*.json`；不要在 Go 测试里内联大段 JSON。
- 测试和生产代码统一用 `github.com/bytedance/sonic`；禁止 `encoding/json`、`json.RawMessage`、`any`、`interface{}`。
- 只用标准库 `testing`；禁止 testify / gomock；禁止用 `time.Sleep` 做同步。
- E2E 入口必须读取 `BASE_URL` 和 `API_KEY`，任一为空则 `t.Skip("BASE_URL and API_KEY are required for e2e test")`。
- E2E HTTP 客户端必须显式设置超时，禁止 `http.DefaultClient`。
- E2E 至少覆盖非流式和流式路径：非流式断言 HTTP 200 和关键 JSON 字段；流式断言 HTTP 200、`text/event-stream`、`X-Trace-Id`，读到实质 delta 后仍继续消费到 `[DONE]`、EOF 或协议结束事件。
- E2E 不强断言模型输出语义；失败时提取 `X-Trace-Id` 并转入 `cls-log-bugfix`。

## 6. 代码契约

- 业务错误创建/包装统一走 `internal/common/ierr`；禁止 `fmt.Errorf` 或 `errors.New`。
- Service 正常返回 `rsp, nil`；业务失败写入 `rsp.Error = ierr.ErrXxx.BizError()`。
- Handler 保持薄封装：`return util.WrapHTTPResponse(h.svc.Method(ctx, req))`。
- 日志使用 `logger.WithCtx(ctx)` 或 `logger.WithFCtx(c)`；消息前缀为 `[PascalCaseModule]`；key/token/secret/password 必须用 `util.MaskSecret()`。
- 业务包禁止建 `common.go` 工具堆场；导出公共 helper 放 `internal/util/` 或 `internal/common/`。
- Redis key、存储路径、ID 格式、Data URL 模板等字符串模板放 `internal/common/constant/string.go`。
- 业务包禁止定义本地 `const` 块；使用 `internal/common/constant/`、`internal/common/enum/`、`internal/enum/`。
- HTTP 状态码使用 `fiber.StatusXxx`，禁止裸数字。
- DTO 时间字段用 `time.Time`；禁止 Service 层提前格式化为字符串。
- DTO 包禁止导入 `internal/infrastructure/database/model`；需要数据库字段时将具体字段作为参数传入。

## 7. Context 契约

- handler/service/proxy/converter/dto 必须从调用方接收 `context.Context`。
- 上述层禁止自行创建 `context.Background()` 或 `context.TODO()`。
- 允许根 context 的场景：启动/基础设施初始化、cron 入口并注入 trace ID、agent 初始化、`util.CopyContextValues`。
- 异步协程池任务必须使用 `util.CopyContextValues(ctx)`，禁止直接持有原始请求 context。
- 读取上下文值优先用 `util.CtxValueString()` / `util.CtxValueUint()`，禁止直接类型断言。
- 新 context key 必须注册到 `internal/common/constant/ctx.go`。

## 8. DTO 与 API 契约

- 修改 OpenAI 或 Anthropic DTO 前，先看 `/docs` 的 OpenAPI 文档，保持协议兼容。
- OpenAI 和 Anthropic 接口支持跨 provider 转换；改 DTO 常需同步 service、proxy、converter、SSE 合并/归一化工具。
- Huma 安全方案：用户路由用 `jwtAuth`，LLM 代理路由用 `apiKeyAuth`。

## 9. 仓库与 CI

- `.github/workflows/docker-publish.yml` 在推送到 `master`、`v*.*.*` tag、PR 到 `master` 和定时任务时构建多架构 GHCR 镜像。
- 影响镜像构建的 path filter 包含 `internal/**`、`docker/**`、`cmd/**`、`main.go`、`go.mod`、`go.sum`。
- 本地 hook 可通过 `bash .githooks/setup.sh` 安装；除非用户明确要求，不要绕过 hook。
- 使用 `.worktrees/` 作为 git worktree 目录。
- `AGENTS.md`、`CLAUDE.md`、`CODEBUDDY.md` 是项目级持久规范，修改其中一个时保持同步。
- 编写文档必须使用中文
