# CODEBUDDY.md

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
- 业务包禁止定义本地 `const` 块；使用 `internal/common/constant/`、`internal/common/enum/`、`internal/enum/`。
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
