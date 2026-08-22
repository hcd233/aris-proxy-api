# Demo 访问事件审计

> 日期：2026-08-22
> 状态：已评审通过（设计对话确认）

## 1. 背景与目标

Demo 演示账户免 OAuth 登录（仅 IP 限流），多访客共用同一账户，模块白名单中间件在路由级拦截。当前 admin 完全看不到「谁在什么时候用 demo 访问了什么」，也无法发现有人通过直接调 API 探测锁定模块。

本次改动目标：

1. 记录 **demo 登录事件**（成功 + 被拒及原因）。
2. 记录 demo 用户对开放模块的 **每次 API 访问**（含被白名单拒绝的探测尝试）。
3. admin 在 Audit 模块新增子页查看；demo 用户不可见自己的访问日志。
4. 写入异步化不阻塞请求，遵循现有审计写入模式。

## 2. 需求决策记录

| 决策点 | 结论 | 理由 |
|--------|------|------|
| 审计范围 | 登录 + 模块级 API 访问 | 只记登录无法回答「demo 干了什么」；全量请求级噪音高收益低 |
| 记录粒度 | 每次 API 调用一条，不去重 | 量级小（demo 限流 30 次/5s，估每天几百~几千行）；去重丢失真实调用量信息 |
| 查看入口 | Audit 下新增 `audit/demo` 子页 | 与 model/cron 审计信息架构统一 |
| 可见性 | 仅 admin 可见 | 监控访客的管理功能；demo 权限本身只读受限 |
| 被拒访问 | 同时记录，带 reason 字段 | 「试探边界」正是审计最有价值的信号；拦截点上下文齐全 |

## 3. 数据模型

新表 `demo_access_audits`，GORM model `internal/infrastructure/database/model/demo_access_audit.go`，注册进 `model/base.go` AutoMigrate：

| 字段 | 类型 | 说明 |
|------|------|------|
| action | string, index | `login` / `login_denied` / `module_access` / `module_denied` |
| module | string | demo 模块名（如 `sessions`）；login 类动作为空串 |
| path | string | 请求路径（如 `/api/v1/session/list`） |
| ip | string | 客户端 IP |
| user_agent | string | User-Agent |
| reason | string | 拒绝原因：`login_disabled` / `no_demo_user` / `module_closed`；成功时为空串 |

继承 BaseModel。不做保留期清理任务（与 `model_call_audits` / `cron_call_audits` 口径一致）。仓储为轻量 Save/List，跟随 cron 审计模式，不建 domain 聚合。

## 4. 写入路径

照抄 `PoolManager.SubmitModelCallAuditTask`（`internal/infrastructure/pool/store_pool.go:187`）模式：

- 新增 `dto.DemoAccessAuditTask`（携带 ctx + 上表字段）。
- `PoolManager.SubmitDemoAccessAuditTask(task)` 在协程池内落库，best-effort：失败只打日志不影响业务。
- 消费方通过窄接口 `DemoAccessAuditSubmitter` 依赖（fx 注入），不直接依赖 PoolManager。

## 5. 模块访问中间件

实现机制说明：本项目管理 API 的错误一律返回 HTTP 200、错误语义由响应体 `error.code` 承载（见 `internal/api/util/error.go`），外层中间件无法凭状态码判断放行/拒绝。因此审计埋点直接内嵌在 `limitUserPermission`（`internal/middleware/permission.go`）的 demo 分支中：

- 仅当 ctx 中 `permission == enum.PermissionDemo` 时产生记录；admin/user 正常使用这些路由不产生任何记录。
- demo 分支放行（模块开放）→ 记 `module_access`；demo 分支拒绝（模块关闭/accessor 缺失）→ 记 `module_denied`（reason=`module_closed`）。埋点位于权限判断结果处，天然准确，无需外层包裹。
- `LimitUserPermissionWithDemoMiddleware` 增加 audit submitter 参数（nil 时跳过审计），沿现有 demoAccessor 的传递链路注入到各路由注册函数。
- IP 经 `humafiber.Unwrap(ctx).IP()` 获取（同 `middleware/rate.go:140` 现有做法），UA 取请求 header，path 取 fiber `Path()`。

## 6. 登录事件埋点

`internal/application/demo/command/login.go` 注入 submitter，三个埋点：

- 成功签发 token pair → `login`。
- 入口开关关闭 → `login_denied`，reason=`login_disabled`。
- 无 demo 用户 → `login_denied`，reason=`no_demo_user`。
- 配置读取失败属系统故障，不记录。

IP/UA 由 handler 层（`internal/handler/demo.go`）取 fiber `ctx.IP()` 与 UA header 放入 command 结构传递到用例。

已知不记录的边界：IP 限流拦截（发生在中间件层、未到用例）、`GET /demo/status` 轮询、token refresh。

## 7. 查询接口

`GET /api/v1/audit/demo/log/list`（与 cron 审计 `/api/v1/audit/cron/log/list` 命名惯例一致），**admin only**（普通 `LimitUserPermissionMiddleware(PermissionAdmin)`，不挂 demo 白名单）：

- 参数：page / pageSize / 时间范围 / action / module 筛选。
- 挂现有 audit 路由组与 AuditHandler，应用层扩展 audit query service。

## 8. 前端

新增 `web/src/app/(dashboard)/audit/demo/page.tsx`（`PermissionGuard adminOnly`）：

- 时间倒序分页表格；动作四态徽章着色；筛选栏（时间范围 + 动作 + 模块下拉）。
- Audit 导航下加入口，仅 admin 渲染。
- i18n 补 zh/en 文案。

## 9. 测试与验收

1. 后端单测：中间件分类逻辑（demo/admin 身份 × 成功/拒绝）、login 三分支埋点。
2. e2e 沉淀到 `test/e2e/demo-access-audit/`：demo 登录 → 访问开放模块 → 探测锁定模块 → admin 查列表验证 `login` / `module_access` / `module_denied` 三类记录齐全。
3. 前端 `npm run lint && npm run build`；浏览器验证子页渲染与筛选。
4. 部署后跑 e2e 回归。

## 10. 不做的事（YAGNI）

- 不做窗口去重（同一 IP 同模块 N 分钟只记一次）。
- 不做通用操作审计表（现在只有 demo 一个场景，将来做 admin 操作审计时再迁移）。
- 不做审计数据保留期清理任务。
- 不让 demo 用户看到自己的访问日志。
- 不记录 `demo/status`、token refresh、IP 限流拦截。
