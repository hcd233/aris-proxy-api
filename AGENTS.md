# AGENTS.md

## 0. Meta Prompt 合约

- **角色**：作为 `aris-proxy-api` 的 Go 后端结对工程师，优先交付可运行、可验证、可维护的最小改动。
- **目标**：先理解请求属于需求、bug、API 调用、部署还是文档维护；选择对应流程后再动手。
- **上下文**：以现有代码、`Makefile`、脚本、workflow、hook 为事实源；文档与可执行源冲突时信任可执行源。
- **执行循环**：分类任务 → 加载必要 skill → 阅读相关代码/文档 → 小步计划 → 最小修改 → 聚焦验证 → 汇报证据。
- **边界**：不为普通需求默认走线上日志排障；不为手工 `curl` 结果跳过仓库测试；不绕过 hook 或安全规则。
- **输出**：简短说明做了什么、验证了什么、还有什么未验证；引用文件路径和命令必须精确。

## 1. Karpathy 编码原则

源自 [Andrej Karpathy 对 LLM 编码陷阱的观察](https://x.com/karpathy/status/2015883857489522876)，融入四项基本原则：

### 1.1 先思考，再编码

**不要假设。不要隐藏困惑。暴露权衡。**

实现之前：
- 明确陈述你的假设。如果不确定，先问清楚。
- 如果存在多种解读，全部列出来——不要默默选一种。
- 如果有更简单的方案，说出来。必要时反驳。
- 如果某件事不清晰，停下来。指出哪里困惑。提问。

**反例对照：隐藏假设**

需求："加个导出用户数据的功能"

❌ LLM 常见做法——默默假设导出全部用户、假设文件路径、假设 CSV 字段名，写出一大段代码。

✅ 正确做法——列出假设，逐条确认后再动手：
```
1. 导出范围：全部用户还是筛选子集？（隐私问题？）
2. 导出格式：JSON？CSV？还是直接提供 API 端点？
3. 涉及字段：哪些字段可以导出？（是否有敏感字段？）
4. 数据量级：大概多少条记录？（影响实现方案）
```

**反例对照：多种解读**

需求："让搜索更快"

❌ LLM 常见做法——不假思索地加缓存、加索引、加异步等。

✅ 正确做法——列出可能方向，让需求方选择：
```
"更快"可能指：
1. 响应时间更快（<100ms）→ 加数据库索引、缓存
2. 并发吞吐更高 → 加异步、连接池
3. 用户感知更快 → 渐进式加载、即时显示部分结果

当前搜索约 500ms，您更关注哪个维度？
```

### 1.2 简约优先

**用最少代码解决问题。不做投机设计。**

- 不做需求之外的功能。
- 不为一次性代码做抽象。
- 不做未被要求的"灵活性"或"可配置性"。
- 不为不可能的场景写错误处理。
- 如果 200 行能写成 50 行，重写它。

自问："资深工程师会觉得这过于复杂吗？"如果是，简化。

**反例对照：过度抽象**

需求："加个计算折扣的函数"

❌ LLM 常见做法——Strategy 模式、ABC 抽象类、DiscountConfig 配置类、DiscountCalculator 计算器，30 行配置只为算个折扣。

```go
// ❌ 过度工程
type DiscountStrategy interface {
    Calculate(amount float64) float64
}
type PercentageDiscount struct{ Percentage float64 }
func (d PercentageDiscount) Calculate(amount float64) float64 {
    return amount * d.Percentage / 100
}
// ... 还有 FixedDiscount、DiscountConfig、DiscountCalculator ...
```

✅ 正确做法——一个函数搞定：
```go
func CalcDiscount(amount, percent float64) float64 {
    return amount * percent / 100
}
```

等到真的需要多种折扣类型时再重构。

**反例对照：投机功能**

需求："把用户偏好存到数据库"

❌ LLM 常见做法——PreferenceManager 构造器带上缓存、校验器、通知功能；save 方法支持 merge、validate、notify 三个开关参数。

✅ 正确做法——只做需求要求的：
```go
func SavePreferences(db DB, userID int, prefs map[string]any) error {
    _, err := db.Exec("UPDATE users SET preferences = $1 WHERE id = $2", prefs, userID)
    return err
}
```

缓存、校验、通知——等真正需要时再加。

### 1.3 精准修改

**只动必须动的代码。只清理自己的遗留物。**

修改现有代码时：
- 不要"顺手改进"相邻代码、注释或格式。
- 不要重构没坏的东西。
- 遵循现有风格，即使你更偏好另一种写法。
- 如果发现无关的死代码，提一句——不要删掉。

当你的改动产生了孤儿代码：
- 删除你的改动导致不再使用的 import/变量/函数。
- 除非被要求，不要删除已有的死代码。

检验标准：每一行改动的代码都应直接追溯到用户的需求。

**反例对照：顺手重构**

需求："修复空邮箱导致校验器崩溃的 bug"

❌ LLM 常见做法——修复空邮箱的同时"顺便"改了邮箱格式校验规则、补了用户名长度检查、改了注释、加了 docstring。

```diff
- // Check email format
- if not user_data.get('email'):
+ email = user_data.get('email', '').strip()
+ if not email:
      raise ValueError("Email required")
- if '@' not in user_data['email']:
+ if '@' not in email or '.' not in email.split('@')[1]:
      raise ValueError("Invalid email")
- if not user_data.get('username'):
+ username = user_data.get('username', '').strip()
+ if not username:
      raise ValueError("Username required")
+ if len(username) < 3:
+     raise ValueError("Username too short")
```

✅ 正确做法——只改动修复 bug 的最小行：
```diff
- if not user_data.get('email'):
+ email = user_data.get('email', '')
+ if not email or not email.strip():
      raise ValueError("Email required")
```

**反例对照：风格漂移**

需求："给 upload 函数加日志"

❌ LLM 常见做法——加日志的同时改了引号风格、加了类型注解、改了缩进、重写了返回逻辑。

✅ 正确做法——全文保持一致的引号风格、缩进、无类型注解，只加日志相关行。

### 1.4 目标驱动执行

**定义成功标准。循环直到验证通过。**

将任务转化为可验证的目标：
- "加校验" → "先写非法输入的测试，再让它们通过"
- "修 bug" → "先写能复现它的测试，再让测试通过"
- "重构 X" → "确保重构前后测试全部通过"

多步骤任务应给出简要计划：
```
1. [步骤] → 验证: [检查方式]
2. [步骤] → 验证: [检查方式]
3. [步骤] → 验证: [检查方式]
```

强成功标准让你能独立循环。弱标准（"把它搞定"）需要不断澄清。

**反例对照：模糊 vs 可验证**

需求："修复认证系统"

❌ LLM 常见做法——"我来修复认证系统：1. 审查代码 2. 定位问题 3. 改进 4. 测试"——没有可验证的标准。

✅ 正确做法——拆解为可验证步骤：
```
以"修改密码后旧 session 应失效"为例：

1. 写测试：修改密码 → 验证旧 session 被废弃
   验证: 测试失败（成功复现 bug）

2. 修改逻辑：变更密码时废弃 session
   验证: 测试通过

3. 检查：多 device session、并发修改等边界
   验证: 额外测试通过

4. 回归：现有所认证测试仍通过
   验证: 全量测试绿
```

**反例对照：多步骤增量交付**

需求："给 API 加限流"

✅ 正确做法——分步可验证：
```
1. 基本内存限流（单个端点）
   验证: 100 次请求 → 前 10 次成功，后续 429

2. 提取为中间件（应用到所有端点）
   验证: /users 和 /posts 都受限流保护

3. 加 Redis 后端（多机共享）
   验证: 应用重启后限流计数不丢失
```

**反例对照：先复现再修复**

需求："有重复分数时排序会乱"

❌ LLM 常见做法——不改测试直接改排序逻辑。

✅ 正确做法——先写复现测试，再修复，再验证通过。

---

这些原则偏向**谨慎而非速度**。对于琐碎任务（简单打字修正、显而易见的一行改动），自行判断——不是每次改动都需要完整执行上述原则。

## 2. Skill 路由

- **生产 bug / 线上错误 / traceId / `X-Trace-Id` / CLS / E2E 失败**：使用 `cls-log-bugfix`，在 `ap-guangzhou` 查日志并按 trace 追链路。
- **API 调用 / curl 示例 / 生产验证**：使用 `call-api`；它只负责交互式调用示例，不替代 E2E 回归。
- **发布 / 部署**：使用 `deploy-to-production`；提交、推送、CI、SSH 部署和线上 E2E 都在该 skill 中维护。
- **会话开始/初次接触项目 / 需要历史上下文 / 沉淀经验教训**：使用 `agentmemory`；检查并启动 agentmemory 服务器，召回历史经验，保存新的洞察和偏好。
- **写或改 `internal/dto/**` / 新增 huma 路由 / 排查 "field 总是零值" 类问题**：使用 `huma-dto-conventions`；它沉淀了 huma 的 path/query/body 绑定规则、Body 包装模板、响应 unwrap 行为和反模式速查。
- 专项流程细节放在对应 skill，主文档只保留触发条件和项目级硬约束。

## 3. 项目模型

- Go `1.25.1` 后端，提供 LLM 代理网关、用户、API Key、会话管理。
- 入口：`main.go` → `cmd.Execute()` → `cmd/server.go` 的 `server start`。
- 启动链路：database、Redis、共享 HTTP Client、Pond 协程池、cron、Fiber 中间件、可选 `/docs`、API 路由。
- 请求链路：Fiber 中间件 → Huma 路由 → handler → application usecase/command → domain service → infrastructure repository/transport。
- 依赖注入：`go.uber.org/dig`，全部在 `internal/bootstrap/container.go` 中注册。
- LLM 代理分层：`application/llmproxy/usecase` 编排端点查找、转换、代理和存储；`infrastructure/transport` 做 HTTP/SSE 传输；`application/llmproxy/converter` 做 DTO 映射。
- 模型路由和代理 Key 由数据库驱动；运行配置来自 Viper 和 `env/api.env`。

## 4. 常用命令

- 安装依赖：`go mod download`
- 本地运行：`go run main.go server start --host localhost --port 8080`
- 数据库迁移：`go run main.go database migrate`
- 创建对象存储桶：`go run main.go object bucket create`
- 完整本地栈：先创建 `postgresql-data`、`redis-data`、`minio-data` 卷，再执行 `docker compose -f docker/docker-compose-full.yml up -d`
- 构建：`make build`；调试构建：`make build-dev` 或 `make build-debug`
- 规范扫描：`make lint`
- 全量测试：`make test` 或 `go test -count=1 ./...`
- 聚焦测试：`go test -v -count=1 -run TestFunctionName ./test/unit/<topic>/` 或 `./test/e2e/<topic>/`
- 前端开发：`cd web && npm install && npm run dev`（默认 `http://localhost:3000/web`）
- 前端 lint：`cd web && npm run lint`
- 前端构建（同时同步到 `internal/web/dist/`）：`make web-build`；清理产物：`make web-clean`
- 生产构建会自动包含前端：`make build` 在编译 Go 之前先跑 `web-build`

## 5. 开发工作流

- 需求不清时先说明假设并推进；只有边界会影响实现时才向用户确认。
- 如果是 bugfix、线上错误、traceID、日志排查，先启动 `cls-log-bugfix`，在 `ap-guangzhou` 查 CLS 日志，再用 `X-Trace-Id` / traceID 追全链路。
- 修改前先定位相关 handler/usecase/converter/transport/DTO，不做大范围重写。
- 新需求和 bugfix 都应先补或更新测试；bugfix 必须有能复现问题的回归用例。
- 每次改动后依次跑：聚焦测试 → `make lint` → 必要时 `go test -count=1 ./...`。
- 端到端用例**必须**沉淀到代码仓库，放 `test/e2e/<topic>/` 并按下文 E2E 工程骨架维护，测试通过后再提交并推送；**不允许**只用 `curl` 跑完就算闭环。
- 测试和 lint 通过后，只有用户明确要求提交、推送或部署时才执行 git 提交/发布流程。
- 正式发布使用 `deploy-to-production`：推送到 `master`，等待 `docker-publish.yml` 镜像构建完成，再在生产机执行部署脚本。
- 部署后**先跑** `test/e2e/<topic>/` 的 Go 用例，而不是只 `curl` 一下；如需交互式补充验证再用 `call-api` skill。
- 如果 E2E 失败，取响应头 `X-Trace-Id`，回到 CLS 排障步骤；重复直到需求或 bugfix 完成。

## 6. 测试契约

- 单元测试目录：`test/unit/<topic>/`；端到端测试目录：`test/e2e/<topic>/`。
- 所有 `*_test.go` 只能放在上述目录；不要放在 `internal/` 或 `test/` 根目录。
- 测试数据放对应目录 `fixtures/*.json`；E2E 请求体放 `fixtures/requests/*.json`；不要在 Go 测试里内联大段 JSON。
- 测试和生产代码统一用 `github.com/bytedance/sonic`；禁止 `encoding/json`、`json.RawMessage`、`any`、`interface{}`。
- 只用标准库 `testing`；禁止 testify / gomock；禁止用 `time.Sleep` 做同步。
- E2E 入口必须读取 `BASE_URL` 和 `API_KEY`，任一为空则 `t.Skip("BASE_URL and API_KEY are required for e2e test")`。
- E2E HTTP 客户端必须显式设置超时，禁止 `http.DefaultClient`。
- E2E 至少覆盖非流式和流式路径：非流式断言 HTTP 200 和关键 JSON 字段；流式断言 HTTP 200、`text/event-stream`、`X-Trace-Id`，读到实质 delta 后仍继续消费到 `[DONE]`、EOF 或协议结束事件。
- E2E 不强断言模型输出语义；失败时提取 `X-Trace-Id` 并转入 `cls-log-bugfix`。

## 7. 代码契约

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

## 8. Context 契约

- handler/service/proxy/converter/dto 必须从调用方接收 `context.Context`。
- 上述层禁止自行创建 `context.Background()` 或 `context.TODO()`。
- 允许根 context 的场景：启动/基础设施初始化、cron 入口并注入 trace ID、agent 初始化、`util.CopyContextValues`。
- 异步协程池任务必须使用 `util.CopyContextValues(ctx)`，禁止直接持有原始请求 context。
- 读取上下文值优先用 `util.CtxValueString()` / `util.CtxValueUint()`，禁止直接类型断言。
- 新 context key 必须注册到 `internal/common/constant/ctx.go`。

## 9. DTO 与 API 契约

- 修改 OpenAI 或 Anthropic DTO 前，先看 `/docs` 的 OpenAPI 文档，保持协议兼容。
- OpenAI 和 Anthropic 接口支持跨 provider 转换；改 DTO 常需同步 usecase、proxy、converter、SSE 合并/归一化工具。
- Huma 安全方案：用户路由用 `jwtAuth`，LLM 代理路由用 `apiKeyAuth`。

## 10. 仓库与 CI

- `.github/workflows/docker-publish.yml` 在推送到 `master`、`v*.*.*` tag、PR 到 `master` 和定时任务时构建多架构 GHCR 镜像。
- 影响镜像构建的 path filter 包含 `internal/**`、`docker/**`、`cmd/**`、`main.go`、`go.mod`、`go.sum`。
- 本地 hook 可通过 `bash .githooks/setup.sh` 安装；除非用户明确要求，不要绕过 hook。
- 使用 `.worktrees/` 作为 git worktree 目录。
- `AGENTS.md`、`CLAUDE.md`、`CODEBUDDY.md` 是项目级持久规范，修改其中一个时保持同步。
- 编写文档必须使用中文

### Git 分支规范

- 任何开发任务都必须先在 `.worktrees/` 下创建或切换 git worktree，并在该 worktree 上 checkout 分支进行开发，禁止直接在主工作区开发。开发完成后询问用户是否需要提mr或者直接合并到master，禁止擅自操作
- 分支命名规范：`{feature|bugfix|refactor|chore|docs|test|hotfix}/{5个以内小写英文单词描述功能或修复，使用连字符}-{当前 datetime，例如 2026-05-28}`。
- 分支示例：`feature/session-share-2026-05-28`、`bugfix/token-expiry-2026-05-28`、`refactor/split-endpoint-model-2026-05-28`。

## 11. API 路由命名规范

### 11.1 分层结构

```
/api/v1/{resource}[/{action}]
/api/{provider}/v1/{action}      # LLM 代理路由 (openai/anthropic)
```

### 11.2 通用规则

- **资源名使用单数小写**：`/user`、`/session`、`/apikey`、`/endpoint`、`/model`、`/audit`
- **禁止裸尾斜杠**：`POST /endpoint` ✅，`POST /endpoint/` ❌；所有路径必须有明确路径段
- **操作通过 Path 段表达，不依赖 HTTP Method 表达语义差异**（即不使用 `GET /endpoint` + `POST /endpoint` 作为唯一区分）

### 11.3 操作映射

| 操作 | Method | Path | 示例 |
|------|--------|------|------|
| 创建 | POST | `/{resource}` | `POST /endpoint` |
| 列表 | GET | `/{resource}/list` | `GET /endpoint/list` |
| 查询详情 | GET | `/{resource}` | `GET /session?sessionId=1` |
| 获取当前用户 | GET | `/user/current` | `GET /user/current` |
| 按 ID 更新 | PATCH | `/{resource}/{id}` | `PATCH /endpoint/{id}` |
| 按 ID 删除 | DELETE | `/{resource}/{id}` | `DELETE /endpoint/{id}` |
| 特殊动作 | POST/GET | `/{resource}/{action}` | `POST /token`、`GET /audit/logs` |

### 11.4 已注册资源路由一览

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

## 12. Web 前端契约

### 12.1 项目模型

- 位置：仓库根目录 `web/`，独立 npm 工程，不参与 Go module。
- 技术栈：Next.js `16.2.6`（App Router）+ React `19` + TypeScript + Tailwind v4 + shadcn/ui（`base-nova` 风格）+ `@base-ui/react` + `lucide-react` + `sonner`。
- 关键配置（`web/next.config.ts`）：`output: "export"` 静态导出、`basePath: "/web"`、`trailingSlash: true`、`images.unoptimized: true`。前端最终被 Go 后端 embed 服务，**不要改 basePath，也不要引入需要 SSR/Edge 的依赖**。
- 后端 embed 路径：`internal/web/static.go` 里 `//go:embed all:dist`；构建时 `make web-build` 把 `web/out/` 拷贝到 `internal/web/dist/`（已 `.gitignore`）。线上请求由 `internal/router/web.go` 的 `/web/*` 处理，找不到的非 `_next/`/`static/` 路径回落 `index.html`。
- 注意：`web/AGENTS.md` 已声明这是新版 Next.js，与训练数据有差异；动手前优先读 `node_modules/next/dist/docs/` 中的对应章节，并尊重弃用提示。

### 12.2 目录结构

- `src/app/layout.tsx`：全局布局，挂载 `AuthProvider` 和 `Toaster`。
- `src/app/login/`、`src/app/callback/`：OAuth2 登录入口与回调。
- `src/app/(dashboard)/`：登录后的管理后台路由组，包含 `apikeys/`、`endpoints/`、`models/`、`profile/`、`sessions/`、`shares/` 子页面，以及 `layout.tsx`、`page.tsx`。
- `src/app/share/`：会话分享只读页面（公开访问）。
- `src/components/ui/`：shadcn 生成的基础组件，遵循其约定，禁止手改命名/导出方式。
- `src/components/chat/`、`src/components/session-detail/`、`src/components/share/`：业务组件按页面切分。
- `src/components/permission-guard.tsx`：基于 `auth-context` 的角色守卫。
- `src/lib/api-client.ts`：统一 fetch 封装，自动带 `Authorization: Bearer <access_token>` 并处理 401 → `/api/v1/token` 刷新。
- `src/lib/auth-context.tsx`：登录态、`isAdmin/isUser` 判断、`access_token` / `refresh_token` 存储。
- `src/lib/types.ts`：与后端 huma DTO 对应的 TS 类型；后端 DTO 改动时必须同步。
- `src/hooks/use-mobile.ts`：响应式断点 hook。
- `src/lib/utils.ts`：`cn`、通用辅助函数；新增公共 helper 一律放这里，禁止散落业务文件。

### 12.3 开发契约

- 路由别名：使用 `@/components`、`@/lib`、`@/hooks`、`@/components/ui`（见 `components.json`），不要写相对路径回溯。
- 调用后端：所有 HTTP 调用统一走 `src/lib/api-client.ts` 暴露的 `api.*` 方法，**禁止**业务组件里直接 `fetch`；新增接口时同步在 `types.ts` 增补 `XxxReq` / `XxxRsp`，命名与后端 huma DTO 一致。
- 后端开发地址：`API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL`。本地联调先 `go run main.go server start --host localhost --port 8080`，再设置 `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080` 后 `npm run dev`。
- 鉴权：登录态由 `AuthProvider` 维护；需要管理员能力的页面用 `<PermissionGuard role="admin">` 包裹，不要自行重复判断 token。
- UI 组件：优先复用 `src/components/ui/` 里的 shadcn 组件；新增基础组件用 `npx shadcn add <name>` 生成，不要手写散件。
- 样式：仅使用 Tailwind v4 + `cn()` 组合 class，禁止内联 `style` 写定值；颜色走 `globals.css` 中 CSS 变量 + `neutral` baseColor，避免硬编码 hex。
- 图标：统一 `lucide-react`，不要混用其他图标库。
- Toast：用 `sonner` 的 `toast.*`，禁止 `alert/confirm`。
- 路径前缀：所有内部跳转链接必须考虑 `basePath=/web`；用 `next/link`、`next/navigation` 即可，框架自动加前缀，**不要手动拼 `/web` 前缀**。
- 修改前端 DTO 时如发现后端字段缺失，按 `huma-dto-conventions` 流程到 `internal/dto/` 同步更新。

### 12.4 联调与发布

- 本地完整链路：先后端 `go run main.go server start ...`，再 `cd web && npm run dev`，浏览器访问 `http://localhost:3000/web`。
- 生产路径：`make build` → 镜像里 Go 二进制内置 `internal/web/dist/`，浏览器访问 `https://<host>/web/`。
- CI：`.github/workflows/docker-publish.yml` 的 path filter **不包含** `web/**`，所以纯前端改动不会触发镜像重建；纯前端发布需要触发后端文件改动或在 PR 描述中说明，必要时手工触发 workflow。
- 测试：当前没有强制的前端单测/e2e 框架；改动后至少运行 `cd web && npm run lint && npm run build` 验证类型与导出能成功。
- 提交：前端改动同样遵循 `.worktrees/` + `feature|bugfix|refactor|chore|docs|test|hotfix/...-YYYY-MM-DD` 分支规范；与后端联动的功能尽量在同一个 PR 中提交，避免接口前后不一致。
