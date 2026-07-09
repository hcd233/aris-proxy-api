# Skill 路由与开发工作流

> **使用场景**：收到新任务时加载。确定走哪个流程、加载哪个 skill、遵循哪些步骤。

## Skill 路由

- **生产 bug / 线上错误 / traceId / `X-Trace-Id` / CLS / E2E 失败**：使用 `cls-log-bugfix`，在 `ap-guangzhou` 查日志并按 trace 追链路。
- **API 调用 / curl 示例 / 生产验证**：使用 `call-api`；它只负责交互式调用示例，不替代 E2E 回归。
- **生产配置更新 / api.env / K8s ConfigMap**：使用 `update-prod-config`；SSH 到 `api.lvlvko.top` 修改配置，禁止使用裸 IP 地址。
- **发布 / 部署**：推送到 `master` 或合并 PR 到 `master` 自动触发 `docker-publish.yml` 构建镜像并部署到 K8s；不需要额外手动部署步骤。
- **写或改 `internal/dto/**` / 新增 huma 路由 / 排查 "field 总是零值" 类问题**：使用 `huma-dto-conventions`；它沉淀了 huma 的 path/query/body 绑定规则、Body 包装模板、响应 unwrap 行为和反模式速查。
- **编写 Go 代码**：使用 `golang-code-style`（代码风格）、`golang-naming`（命名规范）、`golang-modernize`（现代 Go 特性）、`golang-design-patterns`（设计模式）。
- **修改 `go.uber.org/fx` 相关代码**：使用 `golang-uber-fx`。
- **修改 `github.com/spf13/cobra` 相关代码**：使用 `golang-spf13-cobra`。
- **修改 `github.com/spf13/viper` 相关代码**：使用 `golang-spf13-viper`。
- **使用 `github.com/samber/lo` 函数式编程**：使用 `golang-samber-lo`。
- **使用 `github.com/samber/mo` Monadic 类型**：使用 `golang-samber-mo`。
- **需求 / 设计方案评审**：在动手编码前，使用 `brainstorming`（superpowers）对设计方案进行压力测试——逐步澄清问题、提出 2-3 个方案及权衡、输出设计文档（`docs/superpowers/specs/`），暴露假设、权衡和边界。
- **需求澄清 / 快速决策**：在 `brainstorming` 中按"一次一个问题"的交互式方式厘清需求与设计分支。
- **实现阶段 / 防止过度工程**：使用 `ponytail`（默认 `full` 级别）强制走最简可行阶梯：YAGNI → 复用已有代码 → 标准库 → 原生平台特性 → 已有依赖 → 一行代码 → 最小可行代码。刻意简化处用 `// ponytail: <ceiling>, <upgrade path>` 注释标记。
- **diff 过度工程审查**：实现完成后、提交前，使用 `ponytail-review` 审查 diff 中的过度工程（投机抽象、重复造轮子、死代码），逐行列出可删项。与 lint 互补——lint 查规范，ponytail-review 查复杂度。
- 专项流程细节放在对应 skill，主文档只保留触发条件和项目级硬约束。

## 开发工作流

- **Brainstorming-first 设计先行**：收到需求或 bug 任务后，在着手编码前优先加载 `brainstorming`（superpowers）skill 对设计方案进行压力测试。需求模糊或存在多种方案时先进入 brainstorming 的澄清循环厘清。process skill（设计评审、调试等）优先于 implementation skill。禁止以"这不是正式任务""我先收集信息"等理由跳过设计评审。
- **开发前先读 `CONTEXT.md`**：动手前阅读根 `CONTEXT.md`（涉前端时一并读 `web/CONTEXT.md`），确认相关领域概念与术语；开发中新出现的领域概念、术语或语义边界，及时回写 `CONTEXT.md`。
- **编写或修改 Go 代码时，必须加载 `golang-samber-lo` 和 `golang-samber-mo` skill**，确保正确使用 `github.com/samber/lo` 函数式编程工具和 `github.com/samber/mo` Monadic 类型。
- **实现阶段激活 `ponytail`（默认 `full`）**：编码时强制走最简可行阶梯，不建投机抽象、不造标准库已有的轮子、不做未被要求的"灵活性"。刻意简化处用 `// ponytail: <ceiling>, <upgrade path>` 注释标记，便于后续 `ponytail-debt` 追踪。`ponytail` 不适用于安全、输入校验、数据防损等不可简化的场景。
- 需求不清时先说明假设并推进；只有边界会影响实现时才向用户确认。
- 设计方案不确定时，优先启动 `brainstorming` 做快速方案验证，在设计文档中记录决策。
- 如果是 bugfix、线上错误、traceID、日志排查，先启动 `cls-log-bugfix`，在 `ap-guangzhou` 查 CLS 日志，再用 `X-Trace-Id` / traceID 追全链路。
- 修改前先定位相关 handler/usecase/converter/transport/DTO，不做大范围重写。
- 新需求和 bugfix 都应先补或更新测试；bugfix 必须有能复现问题的回归用例。
- 端到端用例**必须**沉淀到代码仓库，放 `test/e2e/<topic>/` 并按下文 E2E 工程骨架维护，测试通过后再提交并推送；**不允许**只用 `curl` 跑完就算闭环。
- 测试和 lint 通过后，使用 `ponytail-review` 审查本次 diff 的过度工程（投机抽象、重复造轮子、死代码），与 lint 互补——lint 查规范，ponytail-review 查复杂度。审查通过后，只有用户明确要求提交、推送或部署时才执行 git 提交/发布流程。
- 正式发布：推送到 `master` 或合并 PR 到 `master`，`docker-publish.yml` 自动构建镜像并部署到 K8s，无需手动 SSH 执行部署脚本。可以通过gh命令行跟踪工作流执行情况，来判断是否完成部署
- 部署后**先跑** `test/e2e/<topic>/` 的 Go 用例，而不是只 `curl` 一下；如需交互式补充验证再用 `call-api` skill。
- 如果 E2E 失败，取响应头 `X-Trace-Id`，回到 CLS 排障步骤；重复直到需求或 bugfix 完成。
- 如果是Web端相关的修复，还需要使用chrome mcp来验证相关网页交互
