# Demo 数据展示逻辑重设计

> 日期：2026-08-20
> 分支：`feature/demo-redesign-2026-08-20`
> 状态：已评审通过

## 1. 背景与目标

当前 demo（演示账户）通过「取模抽样 + 模块白名单」向访客展示受限数据：`demo_configs.sample_modulus` 对行为数据（sessions / audit 及审计聚合图表）按 `id % K == 0` 抽样，demo 只能看到抽样命中的会话与审计行。该机制存在两个问题：

1. 抽样粒度不可控——demo 看到的会话是"碰巧命中取模"的，admin 无法指定"展示哪几个会话"。
2. 非 session 的配置/审计模块要么全量明文（审计里的用户名/邮箱/密钥名），要么被抽样裁剪，语义不清晰。

本次重设计目标：

1. **demo 独立成 tab**：把散落在 `users` 页的 demo 配置迁移到独立 `/demo` 页。
2. **砍掉取模抽样**：非 session 模块展示**全量**数据，但**关键字段脱敏**。
3. **demo 接口按 IP 限流**：demo 登录后的管理 API 访问按 IP 令牌桶限流。
4. **demo sessions 白名单**：admin 在 demo tab 选取多个 session 供 demo 访问，支持批量增删与列表（替代取模）。

## 2. 现状梳理

- **`demo_configs` 单行表**（`internal/infrastructure/database/model/demo_config.go`）：`id`、`login_enabled`、`sample_modulus`、`modules`（serializer:json）、`updated_at`。
- **Demo 账户**：全局单例只读用户（permission=demo），由 `POST /api/v1/user/demo` / `/demo/restore` 设置/恢复。
- **取模抽样**：`DemoScopeProvider.SampleModulus`（`internal/application/demo/query/get_config.go`）供 session/audit 各 handler 调用。
  - session：`ListSessionsByUserQuery.SampleModulus` → `ListAllSessions(..., sampleModulus)`；详情/meta/messages/tools 校验 `sessionID % K == 0`；选项 `ListSessionOptionQuery.SampleModulus`。
  - audit：`auditService.resolveSampleModulus` → `ListAll(...) / ListAuditOption(...) / 各聚合图表(...)`。
- **模块白名单**：`LimitUserPermissionWithDemoMiddleware` + `DemoModuleAccessor.IsModuleOpen`。
- **IP 限流**：`TokenBucketRateLimiterMiddleware` 仅在 demo login/status、share、oauth2 callback 等入口使用；demo 登录后的管理 API 无 IP 限流。
- **前端**：`DemoConfigCard` 嵌在 `web/src/app/(dashboard)/users/page.tsx`；侧边栏 `nav.demoModule` 控制 demo 模块锁定态。

## 3. 设计决策

| 决策点 | 结论 |
|--------|------|
| 脱敏范围 | **B 方案**：身份类（`UserName`/`UserEmail`/`APIKeyName`）+ 上游/连接类（`Endpoint` 名、`TraceID`、`UpstreamModel`、BaseURL）脱敏；保留统计数字与别名 |
| 白名单 session 内容 | 不脱敏（admin 选取即授权完整展示） |
| 选取交互 | **B 方案**：内嵌 session 选择器（复用列表搜索/分页 + 勾选批量加入）+ 已选列表批量移除 |
| demo 限流实现 | 修改现有 `TokenBucketRateLimiterMiddleware`，增加 functional options（`WithPermissionFilter`） |
| 限流阈值 | `PeriodDemoAccess = 5s`，`LimitDemoAccess = 30`（常量，可调） |

## 4. 后端设计

### 4.1 移除取模抽样

删除 `SampleModulus` 的全部链路：

- `internal/infrastructure/database/model/demo_config.go`：删除 `SampleModulus` 列。
- `internal/application/demo/port/handler.go`：删除 `DemoConfigEntity.SampleModulus`、`DemoConfigView.SampleModulus`、`DemoScopeProvider` 接口。
- `internal/application/demo/query/get_config.go`：删除 `demoScopeProvider` 实现；`toDemoConfigView` 去掉 `SampleModulus`。
- `internal/application/demo/command/update_config.go`：删除 `SampleModulus` 校验（`>=2`）与赋值。
- `internal/application/demo/command/login.go` / `query/get_config.go`：视图构建同步去掉字段。
- `internal/dto/demo.go`：删除 `DemoConfig.SampleModulus`、`DemoConfigBody.SampleModulus`。
- `internal/handler/demo.go`：`toDemoConfigDTO` 去掉 `SampleModulus`。
- `internal/infrastructure/repository/demo_config_repository.go`：`Get`/`Save`/`toDemoConfigEntity` 去掉 `SampleModulus`。
- `internal/common/constant`：删除 `DemoDefaultSampleModulus`（若仅被此处引用）。
- session 侧（`internal/application/session/port/handler.go`、`query/jwt_session_queries.go`、`query/session_meta_query.go`、`query/session_message_list_query.go`、`query/session_tool_list_query.go`、`query/option_list.go`、`handler/session.go`）：删除 `SampleModulus` 字段/参数/`resolveDemoScope`，改走白名单视角（§4.2）。
- audit 侧（`internal/application/audit/query/service.go` 及各 query）：删除 `resolveSampleModulus` 与 `SampleModulus` 参数；demo 与 admin 同走全量查询，差异在脱敏（§4.3）。
- `internal/domain/session/repository.go` / `internal/domain/modelcall/repository.go` 及实现：去掉 `sampleModulus` 参数。

### 4.2 demo sessions 白名单

**表结构**（`internal/infrastructure/database/model/demo_session.go`）：

```go
type DemoSession struct {
    ID        uint      `gorm:"column:id;primary_key;auto_increment"`
    SessionID uint      `gorm:"column:session_id;uniqueIndex;not null"`
    CreatedAt time.Time `gorm:"column:created_at"`
}
func (DemoSession) TableName() string { return constant.DemoSessionTableName }
```

`constant.DemoSessionTableName = "demo_sessions"`（新增常量）。

**仓储** `internal/infrastructure/repository/demo_session_repository.go`：
- `List(ctx) ([]uint, error)` — 返回全部白名单 sessionID。
- `Add(ctx, ids []uint) error` — 去重后批量插入（`ON CONFLICT DO NOTHING` 或先查再插）。
- `Remove(ctx, ids []uint) error` — 批量删除。

**应用层**（`internal/application/demo/`，扩展 port + command/query）：
- `ListDemoSessionsHandler`：返回白名单会话摘要 `[]*sessionport.SessionSummaryView`（复用 `SessionReadRepository.ListSessionsByIDs`）。
- `AddDemoSessionsCommand` / `AddDemoSessionsHandler`：校验 sessionID 存在（复用 `SessionReadRepository`，批量 `FindByIDs` 或 `ListSessionsByIDs`），去重后落库。
- `RemoveDemoSessionsCommand` / `RemoveDemoSessionsHandler`：批量删除。
- `DemoSessionAccessor`：`IsAllowed(ctx, sessionID uint) (bool, error)` + `AllowedIDs(ctx) ([]uint, error)`，读取失败 fail-closed（拒绝/空集）。

**路由** `internal/router/demo.go` 扩展（admin，JWT + `LimitUserPermissionMiddleware(PermissionAdmin)`）：
- `GET /demo/sessions/list` — 列出白名单会话摘要。
- `POST /demo/sessions` — 批量添加（body: `{sessionIds: []uint}`）。
- `DELETE /demo/sessions?ids=1,2,3` — 批量移除。

**session 查询改造**：
- 列表：`ListSessionsByUserQuery` 增加 `SessionIDs []uint`（demo 视角传入白名单，替换 `SampleModulus`）。demo 走 `ListSessionsByIDs(ctx, ids, ...)`；非 demo 逻辑不变。
- 详情/meta/messages/tools：`SampleModulus` 校验替换为 `AllowedIDs` 成员判断；不在白名单返回 `ErrDataNotExists`（防遍历）。
- 选项：`ListSessionOptionQuery.SampleModulus` 替换为 `SessionIDs []uint`，repo 按 ID 集合过滤 distinct score/model/messageCount。

### 4.3 脱敏

新增脱敏工具（`internal/common/util/secret.go` 或新文件）：
- 复用 `MaskSecret`（保留前 4 + 后 4）用于连接类字段：`Endpoint` 名、`TraceID`、`UpstreamModel`、`APIKeyName`、BaseURL。
- 新增 `MaskIdentity(s string) string`：对任意非空字符串返回固定 `"***"`，用于身份类字段 `UserName`、`UserEmail`（姓名/邮箱不保留结构，避免反推）。

脱敏在 query 视图构建处按 `isDemo` 标志触发：

- **audit**（`internal/application/audit/query/list_audit_logs.go` `buildAuditViews`）：传入 `isDemo`，为 true 时对 `UserName`、`UserEmail`、`APIKeyName`、`Endpoint`、`TraceID` 脱敏。`auditService.ListLogs` 对 demo 分支传 `isDemo=true`。
- **models**（`internal/application/model/query/list_models.go`）：`ListModelsQuery` 增加 `isDemo`；true 时 `UpstreamModel`、嵌套 `EndpointView` 的 BaseURL/名称脱敏（APIKey 已 `MaskSecret`）。
- **endpoints**（`internal/application/endpoint/query/list_endpoints.go`）：`ListEndpointsQuery` 增加 `isDemo`；true 时 BaseURL 脱敏（APIKey 已脱敏）。

> 保留字段：`ModelID`/`Alias`（别名）、token/latency/status 数字、协议、UA、错误信息。

### 4.4 demo 接口 IP 限流

**改造 `TokenBucketRateLimiterMiddleware`**（`internal/middleware/rate.go`）增加 functional options：

```go
type RateLimiterOption func(*rateLimiterConfig)

type rateLimiterConfig struct {
    permissionFilter enum.Permission // 空串 = 不过滤（现状行为）
}

func WithPermissionFilter(p enum.Permission) RateLimiterOption
```

签名改为：

```go
func TokenBucketRateLimiterMiddleware(cache *redis.Client, serviceName string, key enum.CtxKey, period time.Duration, capacity int64, opts ...RateLimiterOption) func(ctx huma.Context, next func(huma.Context))
```

中间件入口处：若 `permissionFilter != ""` 且 `util.CtxValuePermission(ctx.Context()) != permissionFilter`，直接 `next(ctx)` 放行（零开销、不改现状）。否则走既有令牌桶逻辑。

**常量**（`internal/common/constant/oauth.go` 或新文件）：

```go
PeriodDemoAccess = 5 * time.Second
LimitDemoAccess  = 30
```

**挂载**：在以下 demo 可访问 group 的 `UseMiddleware(JwtMiddleware)` 之后追加：

```go
group.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(cache, "demoAccess", "", constant.PeriodDemoAccess, constant.LimitDemoAccess, middleware.WithPermissionFilter(enum.PermissionDemo)))
```

覆盖 group：`session`、`endpoint`、`model`、`audit`、`cron`、`trigger`、`metrics`、`demo`（config 读取组）。同一 `serviceName="demoAccess"` + IP 维度 → 全局共享一个桶。

> `demo` login/status 的现有 IP 限流保持不变（登录防刷语义不同）。

## 5. 前端设计

- **新增 `/demo` 页**（`web/src/app/(dashboard)/demo/page.tsx`，`PermissionGuard adminOnly`）：
  - `DemoConfigCard`（迁移自 users 页，去掉 sampleModulus 输入框）。
  - `DemoSessionsManager`：已选列表（勾选批量移除）+ 内嵌选择器（复用 `listSessions` 搜索/分页 API，勾选批量加入）。
- **侧边栏**（`layout.tsx` `getNavItems`）：新增 `{ labelKey: "nav.demo", href: "/demo/", icon: ..., adminOnly: true }`（无 `demoModule`，demo 不可见）。
- **users 页**：移除 `DemoConfigCard` 引用；「设为 Demo/恢复」保留。
- **types.ts / api-client.ts**：`DemoConfig` 去掉 `sampleModulus`；新增 `DemoSession` 类型与 `listDemoSessions` / `addDemoSessions` / `removeDemoSessions`。
- **locales**：`en/zh/ja` 新增 demo 页与 demo sessions 相关文案。

## 6. 测试

- 单测：
  - demo sessions 增删/去重/不存在 session 校验。
  - 脱敏输出（audit/models/endpoints 在 demo 视角字段已掩码）。
  - `WithPermissionFilter`：非 demo 放行、demo 限流。
- E2E（`test/e2e/demo/`）：
  - demo 登录 → 白名单 session 可读，非白名单返回"不存在"。
  - 非 session 模块字段已脱敏。
  - 超阈值请求返回 429 + `Retry-After`。
  - admin 批量增删 demo sessions 并列表校验。

## 7. 风险与权衡

- **全量数据 + 脱敏**取代抽样后，demo 可看到全部审计行数（仅身份/连接字段脱敏）——这是需求明确要求的取舍，统计数字与别名有意保留。
- **白名单 session 内容明文**：admin 选取即授权，需 admin 自觉只选可公开的会话。
- **`demo_sessions` 孤儿条目**：session 软删后白名单条目残留，查询时自然过滤，不做主动清理（ponytail）。
- **迁移**：`demo_configs` 删除 `sample_modulus` 列需 DB 迁移脚本；`demo_sessions` 为新表。
