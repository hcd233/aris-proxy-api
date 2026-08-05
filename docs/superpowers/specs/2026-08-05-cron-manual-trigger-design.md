# 支持手动触发 Cron Job 设计文档

> 日期：2026-08-05
> 分支：`feature/cron-manual-trigger-2026-08-05`
> 状态：已评审

## 背景

管理后台已支持 cron 任务列表、启停开关、cron 表达式热重载（`GET /cron/list`、`PATCH /cron`），但缺少「立即执行一次」的能力。本功能为 cron 任务新增手动触发入口：管理员在管理后台点击「立即执行」，任务绕过 cron 调度器立即执行一次，结果照常写入执行审计（CronCallAudit），且审计记录区分触发来源（`scheduled` / `manual`）。

## 需求决策（已与用户确认）

1. **disabled 任务也可手动触发**：手动触发是一次性显式操作，不受启停开关影响，无需先启用。
2. **异步执行**：API 立即返回「已触发」，任务在后台 goroutine 执行，用户在「执行日志」页查看结果。与定时执行同构，避免长任务阻塞请求。
3. **审计区分触发来源**：CronCallAudit 表新增 `trigger_source` 列（`scheduled` / `manual`），执行日志可区分来源。
4. **拿不到锁时忽略执行**：若目标任务正在执行中（Redis 分布式锁被占用），本次手动触发被忽略，前端提示「任务正在执行中」，后端**不记录任何审计数据**。

## 核心思路

手动触发 = 绕过 cron 调度器、直接执行该任务被 `wrapCronFunc` 包装的执行函数。锁、panic 恢复、审计等逻辑天然复用。触发请求可能落到任意 Pod，Redis 分布式锁保证同一时刻只有一个实例执行 fn，不会与定时执行并发冲突，因此**无需跨 Pod 广播**。

与定时执行的关键差异（也是本设计的核心约束）：

| 维度 | 定时触发（scheduled） | 手动触发（manual） |
|------|---------------------|-------------------|
| 启用检查 | 检查 DB `enabled`，disabled 跳过并记 `skipped` 审计 | **不检查**（disabled 也可触发） |
| 拿锁时机 | 后台调度时拿锁，拿不到静默跳过（不记审计） | **同步拿锁**：拿到 → 异步执行；拿不到 → API 返回 `ErrResourceLocked`，零记录 |
| 审计 source | `scheduled` | `manual` |

## 架构与改动清单

### 1. 常量（`internal/common/constant/cron.go`）

新增：

```go
CronTriggerSourceScheduled = "scheduled" // 定时触发
CronTriggerSourceManual    = "manual"    // 手动触发
```

### 2. 触发执行器（`internal/cron/lock_runner.go`）

新增函数 `TriggerWithLock`，与 `wrapCronFunc` 互补：

```go
// TriggerWithLock 手动触发：同步获取分布式锁，拿到锁后在后台 goroutine 执行 fn（含续期、
// panic 恢复与审计，source=manual），立即返回 true；拿不到锁或加锁失败返回 false，不产生任何记录。
func TriggerWithLock(name string, locker lock.Locker, key string, opts LockOptions,
    fn func(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error)) bool
```

实现要点：

- 父 ctx 用 `getBootstrapContext()`（不是请求 ctx，避免请求结束后取消后台执行）；注入新 traceID
- 同步 `locker.Lock(...)`；拿不到锁仅打日志并返回 false（**不记审计**）
- 拿到锁后 goroutine 内：`renewLoop` 续期 + `defer recover`（panic 时记审计）+ 执行 fn + 记审计（`source=manual`）
- 审计写入复用 `saveCronCallAudit`（需增加 source 参数）

同时 `wrapCronFunc` 增加 `source string` 参数，scheduled 路径保持现有启用检查逻辑不变。

### 3. Cron 接口与实现（`internal/cron/`）

- `internal/cron/cron.go`：`Cron` 接口新增 `Trigger() bool`（返回是否成功获取锁并启动执行）
- 4 个任务实现（session_dedup.go / soft_delete_purge.go / think_extract.go / blocked_hit_sync.go）：
  - `Start()` 中已有 `locker`/锁 key 构造逻辑，将锁 key 提取为结构体字段
  - 新增 `Trigger()` 方法：调 `TriggerWithLock(...)` 执行同一条业务 fn

### 4. CronManager（`internal/cron/manager.go`）

```go
// Trigger 手动触发指定 cron 任务。任务不存在返回 ErrDataNotExists；
// 任务正在执行（拿不到锁）返回 ErrResourceLocked。
func (m *CronManager) Trigger(name string) error
```

- 从 `entries` 找到实例，调 `entry.cron.Trigger()`；返回 false 时包装 `ErrResourceLocked`（信息：任务正在执行中）
- 不调用 pub/sub publish（触发不是配置变更，锁已保证单实例）

### 5. 审计链路（表结构 + 仓储 + DTO）

- `internal/infrastructure/database/model/cron.go`：`CronCallAudit` 新增列

  ```go
  TriggerSource string `json:"trigger_source" gorm:"column:trigger_source;not null;default:scheduled;comment:触发来源:scheduled/manual"`
  ```

  GORM AutoMigrate（`internal/infrastructure/database/model/base.go` 已注册）自动加列，存量行默认 `scheduled`
- `internal/application/cronaudit/port/handler.go`：`CronCallAuditView` 新增 `TriggerSource string`
- `internal/infrastructure/repository/cron_audit_repository.go`：`Save` 写入、`List` 读出该字段
- `internal/dto/cron.go`：`CronCallAuditItem` 新增 `TriggerSource`；`constant.CronCallAuditRepoFields` 等 SQL 字段常量补列
- `internal/handler/cron.go`：`HandleListCronCallAudits` 映射该字段

### 6. HTTP API（管理后台，admin 权限）

`internal/router/cron.go` 新增：

```go
huma.Register(cronGroup, huma.Operation{
    OperationID: "triggerCronJob",
    Method:      http.MethodPost,
    Path:        "/trigger",
    Summary:     "TriggerCronJob",
    Description: "Manually trigger a cron job to run once immediately",
    Tags:        []string{constant.TagCron},
    Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
    Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("triggerCronJob", enum.PermissionAdmin)},
}, cronHandler.HandleTriggerCronJob)
```

链路：router → `handler.CronHandler.HandleTriggerCronJob`（新增）→ `cronmgmtport.TriggerCronJobHandler`（新接口）→ `cronmgmtcommand.NewTriggerCronJobHandler`（新 command）→ `CronManager.Trigger`。

- DTO：`TriggerCronJobReq`（`query:"name"` required）/ `TriggerCronJobRsp`（空响应）
- 错误映射：`ErrDataNotExists` → 任务不存在；`ErrResourceLocked` → 任务正在执行中

### 7. DI 注册（`internal/bootstrap/modules/`）

- `application.go`：`NewTriggerCronJobHandler` 注册到 fx module
- `handler.go`：`CronDependencies` 新增 `TriggerCronJob` 字段并注入

### 8. 前端（`web/src/app/(dashboard)/cron/page.tsx`）

- 每行操作区新增「立即执行」按钮（Play 图标），core/functional 均可点
- 点击 → 确认弹窗 → `POST /cron/trigger` → 成功 toast「已触发，可在执行日志中查看结果」；`ErrResourceLocked` 时 toast「任务正在执行中」；其余错误走 `showErrorToast`
- 触发后刷新列表（可选）或提示用户到「执行日志」查看
- i18n 文案补充（`cron.trigger_*` 相关 key）

### 9. 执行日志页（`web/src/app/(dashboard)/audit/cron/page.tsx`）

- 列表新增「触发来源」列展示 `scheduled`/`manual`（Badge）
- 如有既有筛选机制，补充来源筛选（非必需，可用既有 filter 表达式实现）

## 明确不做（YAGNI）

- 不做跨 Pod 广播手动触发（锁已保证单实例）
- 不做手动触发的独立记录表
- 不做同步返回执行结果
- 不为 `skipped`（拿锁失败）写审计——用户明确要求零记录
- 不限制 core/functional 类型可触发

## 测试计划

### Go 单元测试

- `lock_runner`：`TriggerWithLock` 拿不到锁返回 false 且不产生审计；拿到锁异步执行并写入 `source=manual` 审计
- `CronManager.Trigger`：任务不存在 → `ErrDataNotExists`；实例存在 → 调用实例 `Trigger`
- `wrapCronFunc`：scheduled 路径 disabled 检查行为不变（回归）
- 各任务实现的 `Trigger()` 冒烟：调用后 `TriggerWithLock` 被正确接线（锁 key 正确）

### E2E（`test/e2e/cron_trigger/`）

1. admin 登录 → `POST /cron/trigger` 触发一个 functional 任务 → 断言 200
2. 轮询 `GET /cron/log/list`，断言出现 `triggerSource=manual` 且 `cronName` 匹配的新记录
3. 触发不存在的任务 → 断言 4xx
4. （可选）连续触发同一任务，第二次触发若锁占用则断言 `ErrResourceLocked`

### 验证命令

- `go build ./...` / `go test ./...`
- 前端 `pnpm lint` / `pnpm build`（在 web/ 下）
- E2E 按 `test/e2e/<topic>/` 工程骨架运行

## 影响面

- 新增 DB 列 `cron_call_audits.trigger_source`（AutoMigrate，无数据迁移风险）
- 后端改动：`internal/cron/`（5 文件）、`cronmgmt`（port/command）、`cronaudit`（port/view）、handler、dto、router、bootstrap、constant
- 前端改动：cron 管理页、cron 执行日志页、i18n
