# Cron 调度时间配置 — 设计文档

> 日期：2026-06-17
> 状态：Draft

## 1. 背景

当前 cron 模块仅支持通过 API/前端开关（enabled）控制任务启停，调度时间（spec）为代码常量硬编码，运行时不可修改。需求：

1. 支持通过 API 和前端修改 cron 任务的调度时间（spec）
2. 修改后立即生效（重启对应 cron 实例）
3. 区分「核心 cron」与「功能 cron」，核心 cron 不允许关闭
4. 前端提供交互式调度编辑器，而非手动填写 cron 表达式

## 2. 需求汇总

| 项目 | 决策 |
|---|---|
| 修改 spec 后何时生效 | 立即（重启对应 cron 实例） |
| 哪些任务可改 spec | 全部 4 个任务 |
| 哪些任务可关闭 | 仅功能 cron；核心 cron 不允许关闭 |
| 前端编辑交互 | Dialog 弹窗 + 交互式控件 |
| 重复模式选项 | 每分钟、每小时、每天、每周、每月 + 高级模式（手填 cron） |
| 是否支持一次性执行 | 暂不支持 |

### 2.1 Cron 分类

| 名称 | 类型 | 可关闭 | 可改 spec |
|---|---|---|---|
| SessionDeduplicateCron | 功能 | ✅ | ✅ |
| SoftDeletePurgeCron | 功能 | ✅ | ✅ |
| ThinkExtractCron | 功能 | ✅ | ✅ |
| BlockedHitSyncCron | 核心 | ❌ | ✅ |

## 3. 后端设计

### 3.1 Cron 分类字段

在 DB model、port view 和 DTO 中新增 `type` 字段：

```go
// 枚举值
const (
    CronTypeFunctional = "functional"  // 功能性任务
    CronTypeCore       = "core"        // 核心任务
)
```

- `CronRegistryEntry` 新增 `Type string` 字段
- `buildRegistryEntries()` 中为每个 entry 指定 type
- `CronJob` DB model 新增 `type` 列（varchar，非空）
- `CronJobView` / `CronJobItem` 新增 `Type` 字段
- `Sync` 时写入 type；更新时不允许修改 type

### 3.2 CronManager 热重载

新增 `internal/cron/manager.go`：

```go
type CronManager struct {
    mu      sync.RWMutex
    entries map[string]*managedEntry
    deps    CronDeps
}

type managedEntry struct {
    cron   Cron
    spec   string
}

type CronDeps struct {
    DB          *gorm.DB
    PoolManager *pool.PoolManager
    Cache       *redis.Client
    ThinkRepo   conversation.ThinkExtractRepository
}
```

核心方法：

- `Register(name, cron, spec)` — 启动时注册
- `Restart(name, newSpec) error` — 停旧启新（热重载）
- `Disable(name) error` — 停止指定任务（enabled=false 时只停不启）
- `StopAll()` — 优雅关闭（替代现有逻辑）
- `GetSpec(name) string` — 获取当前生效的 spec

> 注：`Restart` 和 `Disable` 均调用 `StopGracefully`，区别在于 Restart 会用新 spec 创建并启动新实例，Disable 只停不启。

### 3.3 Cron 接口扩展

```go
type Cron interface {
    Start() error
    Stop()            // 阻塞等待运行中任务结束（用于服务关闭）
    StopGracefully()  // 仅停止调度，不等待运行中任务（用于热重载）
}
```

每个 cron 实现新增 `StopGracefully()`：

```go
func (c *SessionDeduplicateCron) StopGracefully() {
    if c.cron != nil {
        c.cron.Stop()  // 调用 robfig/cron Stop，不等待 <-ctx.Done()
    }
}
```

### 3.4 启动流程变更

`InitCronJobs` 改为：

1. 用 `buildRegistryEntries()` 构建（提供默认 spec、type 和 Factory）
2. `Sync` 到 DB
3. **从 DB 读取实际 spec**（允许与常量不同）
4. 用 DB 中的 spec 创建实例 → start
5. 注册到 `CronManager`

### 3.5 运行中任务被重启的边界处理

**场景**：修改 spec 时，目标 cron 任务正在执行。

**方案**：

1. `oldCron.StopGracefully()` — 仅停止调度器，不等待运行中任务
2. 用新 spec 创建并启动新实例
3. 旧实例的运行中任务**自然完成**，不受中断

**安全保证**：

- Redis 分布式锁：新实例的触发拿不到锁 → 跳过本次执行（日志 "Lock held by another instance"）
- 旧任务完成后释放锁 → 下次调度周期新实例正常运行
- 旧任务的审计记录照常保存
- 最坏情况：修改后第一个调度周期被跳过，但绝不会重复执行或数据损坏

### 3.6 Update 流程

```
Handler → UpdateCronJobHandler.Handle(ctx, name, params) →
  1. 校验 spec 合法性（robfig/cron Parse）
  2. 校验 type=core 时 enabled 不可设为 false
  3. repository.Update(ctx, name, params)  // DB 更新
  4. 运行时生效：
     - spec 变更 + enabled=true → CronManager.Restart(name, newSpec)
     - enabled=false → CronManager.Disable(name)
     - 仅 enabled=true（从 disabled 恢复）→ CronManager.Restart(name, 当前spec)
```

### 3.7 Repository 变更

`CronJobRepository.Update` 签名变更：

```go
// 旧
Update(ctx context.Context, name string, enabled bool) error

// 新 — 支持部分更新
Update(ctx context.Context, name string, params UpdateCronJobParams) error

type UpdateCronJobParams struct {
    Enabled *bool
    Spec    *string
}
```

只更新非 nil 字段。

### 3.8 Spec 校验

在 usecase 层使用 `robfig/cron/v3` 的 `ParseStandard` 校验 spec 合法性，不合法返回 `ierr.ErrValidation`。

## 4. API 变更

### 4.1 DTO

```go
// CronJobItem 新增字段
type CronJobItem struct {
    Name        string    `json:"name" doc:"任务名"`
    Type        string    `json:"type" doc:"任务类型: functional/core"`
    Spec        string    `json:"spec" doc:"cron 表达式"`
    Description string    `json:"description" doc:"任务描述"`
    Enabled     bool      `json:"enabled" doc:"是否启用"`
    CreatedAt   time.Time `json:"createdAt" doc:"创建时间"`
    UpdatedAt   time.Time `json:"updatedAt" doc:"更新时间"`
}

// UpdateCronJobReqBody 改为指针字段（部分更新）
type UpdateCronJobReqBody struct {
    Enabled *bool   `json:"enabled,omitempty" doc:"是否启用"`
    Spec    *string `json:"spec,omitempty" doc:"cron 表达式，如 */5 * * * *"`
}
```

### 4.2 路由

无需新增路由，复用现有 `PATCH /api/v1/cron?name=xxx`。

### 4.3 校验规则

- `enabled` 和 `spec` 至少传一个
- `spec` 必须是合法的标准 5 字段 cron 表达式
- `type=core` 的任务 `enabled` 不允许设为 `false`

## 5. 前端设计

### 5.1 交互式调度编辑器 Dialog

点击 Spec 列旁的编辑图标弹出 Dialog：

```
┌──────────────────────────────────────┐
│  Edit Schedule                       │
│                                      │
│  Name:  SessionDeduplicateCron       │
│  (只读)                               │
│                                      │
│  Repeat                              │
│  ┌──────────────────────────────┐    │
│  │ Every Hour            ▼     │    │
│  └──────────────────────────────┘    │
│                                      │
│  ┌─ Every Hour 展开时 ─────────────┐ │
│  │  At minute: [ 0 ▼ ]            │ │
│  └─────────────────────────────────┘ │
│                                      │
│  ┌─ Every Day 展开时 ──────────────┐ │
│  │  At time:   [ 00 ] : [ 00 ]    │ │
│  └─────────────────────────────────┘ │
│                                      │
│  ┌─ Every Week 展开时 ─────────────┐ │
│  │  On day:  [ Monday ▼ ]         │ │
│  │  At time: [ 00 ] : [ 00 ]      │ │
│  └─────────────────────────────────┘ │
│                                      │
│  ┌─ Every Month 展开时 ────────────┐ │
│  │  On day:  [ 1 ▼ ]              │ │
│  │  At time: [ 00 ] : [ 00 ]      │ │
│  └─────────────────────────────────┘ │
│                                      │
│  ┌─ Advanced ──────────────────────┐ │
│  │  Cron expression:              │ │
│  │  ┌──────────────────────────┐   │ │
│  │  │ 0 * * * *                │   │ │
│  │  └──────────────────────────┘   │ │
│  │  ↳ 每小时整点执行               │ │
│  └─────────────────────────────────┘ │
│                                      │
│  [Cancel]  [Save]                    │
└──────────────────────────────────────┘
```

### 5.2 重复模式选项

| 模式 | 生成的 cron 表达式 | UI 控件 |
|---|---|---|
| Every Minute | `* * * * *` | 无 |
| Every Hour | `{minute} * * * *` | 分钟选择 (0-59) |
| Every Day | `{minute} {hour} * * *` | 时 + 分 |
| Every Week | `{minute} {hour} * * {dow}` | 星期 + 时 + 分 |
| Every Month | `{minute} {hour} {dom} * *` | 日期 + 时 + 分 |
| Advanced | 用户手填 | 输入框 + 人类可读预览 |

### 5.3 Spec 列显示

- 人类可读描述为主，cron 表达式为辅（灰色小字）
- 例：`每小时 :00` 下方小字 `0 * * * *`
- 旁边加编辑图标按钮

### 5.4 Enabled 开关

- 功能 cron：显示 Switch，可切换
- 核心 cron：隐藏 Switch 或显示为 disabled + 锁定图标

### 5.5 TS 类型变更

```typescript
export interface CronJobItem {
  name: string;
  type: "functional" | "core";
  spec: string;
  description: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface UpdateCronJobReqBody {
  name: string;
  enabled?: boolean;
  spec?: string;
}
```

### 5.6 前端校验

- 使用 `cronstrue` 库将 cron 表达式转为人类可读描述
- Advanced 模式下，输入时实时校验并显示预览
- 格式不合法时禁用 Save 按钮

## 6. 不改的部分

- `CronRegistryEntry` 的 Factory 签名不变
- 审计逻辑不变
- Redis 锁逻辑不变
- 已有的 4 个 cron 任务的业务逻辑不变
- `Sync` 机制不变（启动时仍将代码默认值同步到 DB）

## 7. 数据库迁移

`cron_jobs` 表新增 `type` 列：

```sql
ALTER TABLE cron_jobs ADD COLUMN type VARCHAR(20) NOT NULL DEFAULT 'functional';
UPDATE cron_jobs SET type = 'core' WHERE name = 'BlockedHitSyncCron';
```

GORM AutoMigrate 会自动添加列，`Sync` 时会写入 type。

## 8. 测试策略

- 单元测试：`UpdateCronJobParams` 部分更新逻辑、spec 校验、核心 cron 关闭保护
- 集成测试：`CronManager.Restart` 热重载流程、边界场景（运行中任务重启）
- E2E 测试：API 完整流程（修改 spec → 验证 DB → 验证运行时生效）
