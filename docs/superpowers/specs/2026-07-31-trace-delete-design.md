# Agent Trace 删除功能设计

> 日期：2026-07-31
> 状态：已批准（brainstorming 流程）
> 范围：后端 API + Web 前端

## 背景与目标

Trace 模块（agent 运行观测记录，含 `Trace` 与关联的 `TraceEvent`）目前只有查询类接口（list / get / events / conversation / report），缺少删除能力。用户希望 agent trace 功能支持删除，用于清理不需要保留的运行观测数据。

现状要点：

- `Trace` / `TraceEvent` 模型均继承 `BaseModel`，已含 `DeletedAt int64` 软删除字段（默认 0）
- 已有删除先例：Session 删除 = 软删除 + 逗号分隔批量 + 失败收集 + owner/admin 权限校验（`deleteSession`）
- 上报链路：hook 客户端经 `ensureTrace` 按 `session_id` 幂等 upsert；session 不存在时会**自动重建 trace**
- 事件查询（events / conversation）均先经 trace 存在性 + 权限校验

## 需求澄清结论

| 决策点 | 结论 |
|--------|------|
| Q1 删除粒度 | 单个 + 批量都要（逗号分隔，对齐 `deleteSession`） |
| Q2 删除语义 | **软删除**（`deleted_at` 置位，与 Session 一致），必须处理"删除后同 session 继续上报导致复活" |
| Q3 复活处理 | **上报侧拦截**：上报时感知软删状态，已删 session 的记录全部 `rejected`，不重建 |
| Q4 事件处理 | **级联软删**：删除 trace 时一并软删其 `trace_events` |
| Q5 权限 | Owner + Admin（复用 `LookupOwnerNamesByUserID`，与 session 删除一致） |
| Q6 交付范围 | 后端 API + Web 列表页（单条 + 批量）+ 详情页（删除后跳回列表） |

## 架构设计

### API 形态

单接口逗号分隔批量，与 `deleteSession` 完全同构：

```
DELETE /api/v1/trace?ids=1,2,3
```

- 单条删除 = 传单个 id；批量 = 逗号分隔
- 响应：`{ deletedCount, failures: [{ id, error }] }`

### 分层改动（自底向上）

**Domain** — `internal/domain/trace/repository.go`

- `Trace` 领域结构体新增 `DeletedAt int64`
- `TraceRepository` 接口新增两个方法：
  - `Delete(ctx context.Context, id uint) error` — 软删 trace 并级联软删 events（事务）
  - `FindBySessionIDIncludingDeleted(ctx context.Context, sessionID string) (*Trace, error)` — `Unscoped` 查询（含软删），用于上报拦截

**Infrastructure** — `internal/infrastructure/repository/trace_repository.go` + `internal/infrastructure/database/dao/trace.go`

- `Delete`：事务内
  1. 软删 trace（`UPDATE traces SET deleted_at=? WHERE id=?`）
  2. 级联软删 events（`UPDATE trace_events SET deleted_at=? WHERE trace_id=?`）
- `FindBySessionIDIncludingDeleted`：`Unscoped()` 查询按 `session_id`，`toTraceDomain` 回填 `DeletedAt`
- 对应 DAO 增加方法（对齐现有 DAO 风格）

**Application — Command** — 新建 `internal/application/trace/command/delete_trace.go`

- `deleteTraceHandler`，逻辑对齐 `delete_session.go`：
  - admin 直通；非 admin 用 `apiKeyRepo.LookupOwnerNamesByUserID` 收集 owner
  - 逐 id：`FindByID` → nil → NotFound 失败项；owner 不匹配 → NoPermission 失败项；`repo.Delete` 失败 → DeleteFailed 失败项；成功 → `DeletedCount++`
  - 记录审计日志（`[TraceCommand] Trace deleted`）

**Application — Command 上报拦截** — 修改 `internal/application/trace/command/report_trace_event.go`

- `Handle` 开头：`FindBySessionIDIncludingDeleted(cmd.SessionID)`
  - 命中且 `DeletedAt != 0` → 全部 records 置 `rejected`（message: trace deleted），不插入、不重建、不报错（rejected 是正常业务结果，符合 fail-open 语义，客户端 spool 会移入隔离区）
  - 未命中或未删除 → 复用该查询结果走原 `ensureTrace` 逻辑（避免二次查询，`ensureTrace` 改为接收已查到的 trace）

**Port** — `internal/application/trace/port/handler.go`

- `DeleteTraceCommand{ UserID uint; IsAdmin bool; IDs []uint }`
- `DeleteTraceFailedItem{ ID uint; Error string }`
- `DeleteTraceResult{ DeletedCount int; Failures []DeleteTraceFailedItem }`
- `DeleteTraceHandler interface { Handle(ctx, cmd) (*DeleteTraceResult, error) }`

**DTO** — `internal/dto/trace.go`

- `DeleteTraceReq{ IDs string \`query:"ids" required:"true"\` }`（逗号分隔，doc 对齐 `DeleteSessionReq`）
- `DeleteTraceRsp{ CommonRsp; DeletedCount int; Failures []DeleteFailed }`（复用 `dto.DeleteFailed`）

**Handler** — `internal/handler/trace.go`

- `TraceHandler` 接口新增 `HandleDeleteTraces`
- `TraceDependencies` 新增 `Delete port.DeleteTraceHandler`
- 实现：解析 ids（逗号分隔），构造 `DeleteTraceCommand{UserID, IsAdmin, IDs}`，结果映射到 DTO

**Router** — `internal/router/trace.go`（queryGroup 内，JWT）

- OperationID `deleteTrace`，`Method: DELETE`，`Path: ""`（与 getTrace 同路径、不同方法）
- Security JWT，`LimitUserPermissionMiddleware("deleteTrace", enum.PermissionUser)`

**DI** — `internal/bootstrap/container.go`

- 注册 `deleteTraceHandler`（依赖 `trace.TraceRepository` + `apikey.APIKeyRepository`）

**常量** — `internal/common/constant/`

- 新增 `TraceDeleteErrorFindFailed` / `TraceDeleteErrorNotFound` / `TraceDeleteErrorNoPermission` / `TraceDeleteErrorDeleteFailed`，以及上报拦截的 rejected message 常量

### 数据流

**删除：**

```
前端 DELETE /api/v1/trace?ids=1,2,3
  → JwtMiddleware 鉴权 → LimitUserPermissionMiddleware
  → HandleDeleteTraces
    → deleteTraceHandler.Handle
      → admin 直通 / 非 admin LookupOwnerNamesByUserID
      → 逐 id：FindByID → owner 校验 → repo.Delete（事务：软删 trace + 级联 events）
      → 返回 DeletedCount + Failures
```

**上报拦截：**

```
hook 客户端对已删 session 继续上报
  → reportTraceEventHandler.Handle
    → FindBySessionIDIncludingDeleted 命中软删
      → 全部 records rejected（message: trace deleted），trace 不复活
```

### 错误处理

- 失败收集沿用 `delete_session.go` 风格，错误文案用新增 `TraceDeleteError*` 常量
- 上报拦截不返回 error（rejected 为正常业务结果，与现有 `invalid` / `storage_failed` 的 rejected 语义一致）

### 边界情况

- **删除后同 session 继续上报**：被拦截为 rejected，trace 不复活（Q3 结论）
- **active 状态 trace 可删**：允许，删除后上报拦截兜底（不限制状态，保持最简单语义）
- **并发删除同一 trace**：软删幂等，第二次 FindByID 走正常流程（已删的 trace 查不到，返回 NotFound 失败项，可接受）
- **events 孤儿**：事务级联软删，无孤儿

## 测试

### 单元测试 — `test/unit/trace/`

- `delete_test.go`（新增）：
  - owner 删除自己的 trace 成功
  - 非 owner 删除 → NoPermission 失败项
  - admin 删除任意 trace 成功
  - not found → NotFound 失败项
  - 批量混合成功/失败 → DeletedCount + Failures 正确
- 上报拦截用例（新增或并入 usecase_test.go）：
  - 已软删 session 上报 → 全部 rejected + 不重建 trace + 不插入事件
  - 未删除 session 上报 → 原逻辑不受影响（回归）
- `fake_repository.go`：补 `FindBySessionIDIncludingDeleted` / `Delete`

### E2E — `test/e2e/trace/trace_test.go`

- 建 trace（上报）→ 删除 → 列表不含该 trace → 同 session 再上报 → 全部 rejected 且列表仍不含

## 前端（Web）

- **列表页** `web/src/app/(dashboard)/trace/page.tsx`：
  - 表头勾选 + 批量删除按钮（选中状态、确认对话框）
  - 行内单条删除按钮（confirm）
  - 交互对齐 sessions 页
- **详情页** `web/src/app/(dashboard)/trace/detail/page.tsx` + `web/src/components/trace-detail/trace-detail-client.tsx`：
  - 删除按钮 + 确认，成功后跳回 `/web/trace/`
- **i18n**：`web/src/locales/` 补充删除相关文案（确认、成功、失败、错误提示）
- **API client**：`web/src/lib/api-client.ts` 补 `deleteTraces(ids)` 方法

## 不做的事（YAGNI）

- 不做恢复（un-delete）能力
- 不做事件单独删除（只随 trace 级联）
- 不做硬删除 / 物理清理 cron（软删数据由既有维护节奏处理）
- 不做删除原因 / 审计页面展示（仅日志记录）
