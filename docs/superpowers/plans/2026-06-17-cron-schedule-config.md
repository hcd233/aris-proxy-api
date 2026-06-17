# Cron 调度时间配置 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持 cron 任务调度时间（spec）运行时修改，区分核心/功能任务类型，前端提供交互式调度编辑器。

**Architecture:** 新增 `CronManager` 管理运行中 cron 实例的热重载；`Cron` 接口新增 `StopGracefully()`；DB model / DTO / Repository 新增 `type` 字段区分核心与功能任务；`UpdateCronJobParams` 改为指针字段支持部分更新；前端用交互式控件替代 cron 表达式输入。

**Tech Stack:** Go 1.25 / robfig/cron v3 / GORM / Huma / React 19 / Next.js 16 / Tailwind v4 / shadcn/ui / cronstrue

---

## File Structure

### 新增文件
| 文件 | 职责 |
|---|---|
| `internal/cron/manager.go` | CronManager 热重载管理器 |
| `web/src/components/cron/schedule-editor.tsx` | 交互式调度编辑器 Dialog 组件 |

### 修改文件
| 文件 | 改动 |
|---|---|
| `internal/common/constant/cron.go` | 新增 CronType 常量 |
| `internal/infrastructure/database/model/cron.go` | CronJob 新增 Type 列 |
| `internal/application/cronmgmt/port/handler.go` | CronJobView 新增 Type，UpdateCronJobParams 改为结构体 |
| `internal/dto/cron.go` | CronJobItem 新增 Type，UpdateCronJobReqBody 改指针字段 |
| `internal/cron/cron.go` | CronRegistryEntry 新增 Type，InitCronJobs 从 DB 读 spec，注册到 CronManager |
| `internal/cron/session_dedup.go` | 新增 StopGracefully() |
| `internal/cron/soft_delete_purge.go` | 新增 StopGracefully() |
| `internal/cron/think_extract.go` | 新增 StopGracefully() |
| `internal/cron/blocked_hit_sync.go` | 新增 StopGracefully() |
| `internal/infrastructure/repository/cron_repository.go` | Sync 写入 type，Update 改为部分更新 |
| `internal/application/cronmgmt/command/update_cron_job.go` | Handle 接受 UpdateCronJobParams，校验 spec，热重载 |
| `internal/handler/cron.go` | HandleUpdateCronJob 适配新参数 |
| `internal/bootstrap/modules/cron.go` | 初始化 CronManager，注入依赖 |
| `internal/bootstrap/lifecycle.go` | 使用 CronManager.StopAll() |
| `web/src/lib/types.ts` | CronJobItem 新增 type，UpdateCronJobReqBody 新增 spec |
| `web/src/lib/api-client.ts` | updateCronJob 请求体新增 spec |
| `web/src/app/(dashboard)/cron/page.tsx` | 表格显示 type/spec 人类可读描述，编辑 Dialog，核心任务开关锁定 |

---

### Task 1: 常量与数据模型 — 新增 type 字段

**Files:**
- Modify: `internal/common/constant/cron.go`
- Modify: `internal/infrastructure/database/model/cron.go`

- [ ] **Step 1: 在 constant/cron.go 新增 CronType 常量和 BlockedHitSync 常量**

在 `internal/common/constant/cron.go` 的 const 块中新增：

```go
CronTypeFunctional = "functional"
CronTypeCore       = "core"

CronModuleSessionDeduplicate = "SessionDeduplicateCron"
CronSpecSessionDeduplicate   = "0 * * * *"
```

注意：`CronModuleSessionDeduplicate` 和 `CronSpecSessionDeduplicate` 目前定义在 `internal/common/constant/session.go`，需确认是否已存在，若已存在则不重复添加，只新增 `CronType*` 常量。

- [ ] **Step 2: 在 CronJob DB model 新增 Type 字段**

修改 `internal/infrastructure/database/model/cron.go`，在 `CronJob` 结构体中 `Name` 和 `Spec` 之间新增：

```go
Type        string    `gorm:"column:type;not null;default:functional;comment:任务类型:functional/core" json:"type"`
```

---

### Task 2: Port 层 — 更新接口和视图类型

**Files:**
- Modify: `internal/application/cronmgmt/port/handler.go`

- [ ] **Step 1: 更新 CronJobView 和接口**

修改 `internal/application/cronmgmt/port/handler.go`：

1. `CronJobView` 新增 `Type string` 字段（在 `Name` 之后）

2. 新增 `UpdateCronJobParams` 结构体：

```go
// UpdateCronJobParams 更新 CronJob 参数（部分更新，非 nil 字段才更新）
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type UpdateCronJobParams struct {
	Enabled *bool
	Spec    *string
}
```

3. 修改 `UpdateCronJobHandler` 接口：

```go
type UpdateCronJobHandler interface {
	Handle(ctx context.Context, name string, params UpdateCronJobParams) error
}
```

4. 修改 `CronJobRepository` 接口的 `Update` 方法签名：

```go
Update(ctx context.Context, name string, params UpdateCronJobParams) error
```

---

### Task 3: Repository 层 — Sync 写入 type，Update 改为部分更新

**Files:**
- Modify: `internal/infrastructure/repository/cron_repository.go`

- [ ] **Step 1: 更新 Sync 方法写入 type**

在 `Sync` 方法中，创建新记录时加入 `Type: job.Type`，更新已有记录时也同步 type：

```go
// 创建时
if err := tx.Create(&dbmodel.CronJob{
    Name:        job.Name,
    Type:        job.Type,
    Spec:        job.Spec,
    Description: job.Description,
    Enabled:     true,
}).Error; err != nil {
```

```go
// 更新时，如果 type/spec/description 有变化
if existing.Type != job.Type || existing.Spec != job.Spec || existing.Description != job.Description {
    if err := tx.Model(&existing).Updates(map[string]any{
        constant.FieldCronType: job.Type,
        constant.FieldSpec:     job.Spec,
        constant.FieldDescription: job.Description,
    }).Error; err != nil {
```

2. 在 `List` 方法的 `lo.Map` 中新增 `Type: row.Type`

3. 在 `Get` 方法的返回值中新增 `Type: row.Type`

- [ ] **Step 2: 重写 Update 方法为部分更新**

```go
func (r *cronRepository) Update(ctx context.Context, name string, params port.UpdateCronJobParams) error {
	updates := map[string]any{}
	if params.Enabled != nil {
		updates[constant.FieldEnabled] = *params.Enabled
	}
	if params.Spec != nil {
		updates[constant.FieldSpec] = *params.Spec
	}
	if len(updates) == 0 {
		return nil
	}

	result := r.db.WithContext(ctx).Model(&dbmodel.CronJob{}).
		Where(constant.CronJobWhereNameEquals, name).
		Updates(updates)
	if result.Error != nil {
		return ierr.Wrap(ierr.ErrDBQuery, result.Error, "update cron job")
	}
	if result.RowsAffected == 0 {
		return ierr.New(ierr.ErrDataNotExists, constant.CronJobNotFoundMessage+name)
	}
	return nil
}
```

- [ ] **Step 3: 在 constant/cron.go 新增缺少的字段常量**

```go
FieldCronType = "type"
```

---

### Task 4: Cron 接口扩展 — StopGracefully

**Files:**
- Modify: `internal/cron/cron.go`
- Modify: `internal/cron/session_dedup.go`
- Modify: `internal/cron/soft_delete_purge.go`
- Modify: `internal/cron/think_extract.go`
- Modify: `internal/cron/blocked_hit_sync.go`

- [ ] **Step 1: 扩展 Cron 接口**

在 `internal/cron/cron.go` 中修改 `Cron` 接口：

```go
type Cron interface {
	Start() error
	Stop()
	StopGracefully()
}
```

- [ ] **Step 2: 为每个 cron 实现新增 StopGracefully()**

以 `session_dedup.go` 为例（其余 3 个结构相同）：

```go
func (c *SessionDeduplicateCron) StopGracefully() {
	if c.cron != nil {
		c.cron.Stop()
	}
}
```

注意：`Stop()` 保持原样（等待运行中任务结束），`StopGracefully()` 只调 `c.cron.Stop()` 不等 `<-ctx.Done()`。

- [ ] **Step 3: CronRegistryEntry 新增 Type 字段**

```go
type CronRegistryEntry struct {
	Name              string
	Type              string  // 新增
	Spec              string
	Description       string
	Enabled           func() bool
	Factory           func(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron
	LockTTL           time.Duration
	LockRenewInterval time.Duration
}
```

- [ ] **Step 4: 更新 buildRegistryEntries 为每个 entry 指定 Type**

```go
func buildRegistryEntries() []CronRegistryEntry {
	return []CronRegistryEntry{
		{
			Name:        constant.CronModuleSessionDeduplicate,
			Type:        constant.CronTypeFunctional,
			Spec:        constant.CronSpecSessionDeduplicate,
			Description: constant.CronDescriptionSessionDeduplicate,
			Enabled:     func() bool { return config.CronSessionDeduplicateEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
				return NewSessionDeduplicateCron(db, cache)
			},
		},
		{
			Name:        constant.CronModuleSoftDeletePurge,
			Type:        constant.CronTypeFunctional,
			Spec:        constant.CronSpecSoftDeletePurge,
			Description: constant.CronDescriptionSoftDeletePurge,
			Enabled:     func() bool { return config.CronSoftDeletePurgeEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
				return NewSoftDeletePurgeCron(db, cache)
			},
		},
		{
			Name:        constant.CronModuleThinkExtract,
			Type:        constant.CronTypeFunctional,
			Spec:        constant.CronSpecThinkExtract,
			Description: constant.CronDescriptionThinkExtract,
			Enabled:     func() bool { return config.CronThinkExtractEnabled },
			Factory: func(_ *gorm.DB, _ *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron {
				return NewThinkExtractCron(thinkRepo, cache)
			},
		},
		{
			Name:        constant.CronModuleBlockedHitSync,
			Type:        constant.CronTypeCore,
			Spec:        constant.CronSpecBlockedHitSync,
			Description: constant.CronDescriptionBlockedHitSync,
			Enabled:     func() bool { return true },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
				blockedRepo := repository.NewBlockedRepository(db)
				hitCache := cachepkg.NewBlockedHitCache(cache)
				return NewBlockedHitSyncCron(db, blockedRepo, hitCache, cache)
			},
		},
	}
}
```

- [ ] **Step 5: 更新 InitCronJobs 中的 Sync 调用，写入 Type**

在 `lo.Map` 中新增 `Type: entry.Type`：

```go
jobs := lo.Map(entries, func(entry CronRegistryEntry, _ int) *cronmgmtport.CronJobView {
    return &cronmgmtport.CronJobView{
        Name:        entry.Name,
        Type:        entry.Type,
        Spec:        entry.Spec,
        Description: entry.Description,
    }
})
```

---

### Task 5: CronManager 热重载管理器

**Files:**
- Create: `internal/cron/manager.go`

- [ ] **Step 1: 创建 CronManager**

```go
package cron

import (
	"context"
	"fmt"
	"sync"

	cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// CronDeps 创建 cron 实例所需的依赖
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronDeps struct {
	DB          *gorm.DB
	PoolManager *pool.PoolManager
	Cache       *redis.Client
	ThinkRepo   conversation.ThinkExtractRepository
}

// managedEntry 管理中的 cron 实例
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type managedEntry struct {
	cron    Cron
	spec    string
	factory func(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron
}

// CronManager cron 实例热重载管理器
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronManager struct {
	mu      sync.RWMutex
	entries map[string]*managedEntry
	deps    CronDeps
}

// NewCronManager 构造 CronManager
//
//	@param deps CronDeps
//	@return *CronManager
func NewCronManager(deps CronDeps) *CronManager {
	return &CronManager{
		entries: make(map[string]*managedEntry),
		deps:    deps,
	}
}

// Register 注册运行中的 cron 实例
//
//	@receiver m *CronManager
//	@param name string
//	@param c Cron
//	@param spec string
//	@param factory func(...) Cron
func (m *CronManager) Register(name string, c Cron, spec string, factory func(db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository) Cron) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[name] = &managedEntry{
		cron:    c,
		spec:    spec,
		factory: factory,
	}
}

// Restart 停旧启新（热重载）
//
//	@receiver m *CronManager
//	@param name string
//	@param newSpec string
//	@return error
func (m *CronManager) Restart(name string, newSpec string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[name]
	if !ok {
		return fmt.Errorf("cron job %s not found in manager", name)
	}

	// 停旧实例（仅停止调度，不等待运行中任务）
	entry.cron.StopGracefully()
	logger.Logger().Info("[CronManager] Stopped old cron instance", zap.String("name", name))

	// 用新 spec 创建新实例
	newCron := entry.factory(m.deps.DB, m.deps.PoolManager, m.deps.Cache, m.deps.ThinkRepo)
	if err := newCron.Start(); err != nil {
		logger.Logger().Error("[CronManager] Failed to start new cron instance",
			zap.String("name", name), zap.Error(err))
		return err
	}

	m.entries[name] = &managedEntry{
		cron:    newCron,
		spec:    newSpec,
		factory: entry.factory,
	}

	logger.Logger().Info("[CronManager] Restarted cron instance with new spec",
		zap.String("name", name), zap.String("spec", newSpec))
	return nil
}

// Disable 停止指定任务（只停不启）
//
//	@receiver m *CronManager
//	@param name string
//	@return error
func (m *CronManager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[name]
	if !ok {
		return fmt.Errorf("cron job %s not found in manager", name)
	}

	entry.cron.StopGracefully()
	logger.Logger().Info("[CronManager] Disabled cron instance", zap.String("name", name))

	return nil
}

// Enable 启用指定任务（从停用状态恢复）
//
//	@receiver m *CronManager
//	@param name string
//	@param spec string
//	@return error
func (m *CronManager) Enable(name string, spec string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[name]
	if !ok {
		return fmt.Errorf("cron job %s not found in manager", name)
	}

	newCron := entry.factory(m.deps.DB, m.deps.PoolManager, m.deps.Cache, m.deps.ThinkRepo)
	if err := newCron.Start(); err != nil {
		logger.Logger().Error("[CronManager] Failed to enable cron instance",
			zap.String("name", name), zap.Error(err))
		return err
	}

	m.entries[name] = &managedEntry{
		cron:    newCron,
		spec:    spec,
		factory: entry.factory,
	}

	logger.Logger().Info("[CronManager] Enabled cron instance", zap.String("name", name))
	return nil
}

// StopAll 优雅关闭所有 cron 实例
//
//	@receiver m *CronManager
func (m *CronManager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, entry := range m.entries {
		entry.cron.Stop()
		logger.Logger().Info("[CronManager] Stopped cron instance", zap.String("name", name))
	}
}
```

---

### Task 6: 修改 InitCronJobs — 从 DB 读 spec，注册到 CronManager

**Files:**
- Modify: `internal/cron/cron.go`

- [ ] **Step 1: 修改 InitCronJobs 签名和逻辑**

`InitCronJobs` 接收 `CronManager`，启动后从 DB 读 spec 并注册：

```go
func InitCronJobs(parentCtx context.Context, db *gorm.DB, poolManager *pool.PoolManager, cache *redis.Client, thinkRepo conversation.ThinkExtractRepository, jobStore CronJobStore, auditStore CronCallAuditStore, manager *CronManager) []Cron {
	SetBootstrapContext(parentCtx)
	SetCronStores(jobStore, auditStore)
	var entries []CronRegistryEntry
	if len(DefaultCronRegistry) > 0 {
		entries = DefaultCronRegistry
	} else {
		entries = buildRegistryEntries()
	}

	// 构建默认 entry 的 name→entry 映射
	entryMap := lo.SliceToMap(entries, func(e CronRegistryEntry) (string, CronRegistryEntry) {
		return e.Name, e
	})

	if cronJobStore != nil {
		jobs := lo.Map(entries, func(entry CronRegistryEntry, _ int) *cronmgmtport.CronJobView {
			return &cronmgmtport.CronJobView{
				Name:        entry.Name,
				Type:        entry.Type,
				Spec:        entry.Spec,
				Description: entry.Description,
			}
		})
		if err := cronJobStore.Sync(parentCtx, jobs); err != nil {
			logger.Logger().Error("[Cron] Sync cron jobs failed", zap.Error(err))
		}
	}

	var crons []Cron
	for _, entry := range entries {
		if !entry.Enabled() {
			logger.Logger().Info("[Cron] Cron job is disabled by configuration", zap.String("name", entry.Name))
			continue
		}

		// 从 DB 读取实际 spec（允许与常量不同）
		actualSpec := entry.Spec
		if cronJobStore != nil {
			job, err := cronJobStore.Get(parentCtx, entry.Name)
			if err == nil && job != nil && job.Spec != "" {
				actualSpec = job.Spec
			}
		}

		c := entry.Factory(db, poolManager, cache, thinkRepo)
		lo.Must0(c.Start())
		crons = append(crons, c)

		// 注册到 CronManager
		if manager != nil {
			manager.Register(entry.Name, c, actualSpec, entry.Factory)
		}

		logger.Logger().Info("[Cron] Cron job started", zap.String("name", entry.Name), zap.String("spec", actualSpec))
	}

	logger.Logger().Info("[Cron] Init cron jobs", zap.Int("count", len(crons)))
	return crons
}
```

---

### Task 7: Usecase 层 — UpdateCronJobHandler 支持 spec 和热重载

**Files:**
- Modify: `internal/application/cronmgmt/command/update_cron_job.go`

- [ ] **Step 1: 重写 UpdateCronJobHandler**

```go
package command

import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/robfig/cron/v3"
)

// updateCronJobHandler 更新 CronJob 处理器
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type updateCronJobHandler struct {
	repo    port.CronJobRepository
	manager port.CronManager
}

// NewUpdateCronJobHandler 构造更新 CronJob 处理器
//
//	@param repo port.CronJobRepository
//	@param manager port.CronManager
//	@return port.UpdateCronJobHandler
func NewUpdateCronJobHandler(repo port.CronJobRepository, manager port.CronManager) port.UpdateCronJobHandler {
	return &updateCronJobHandler{repo: repo, manager: manager}
}

// Handle 处理更新 CronJob 请求
//
//	@receiver h *updateCronJobHandler
//	@param ctx context.Context
//	@param name string
//	@param params port.UpdateCronJobParams
//	@return error
func (h *updateCronJobHandler) Handle(ctx context.Context, name string, params port.UpdateCronJobParams) error {
	// 校验 spec 合法性
	if params.Spec != nil {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(*params.Spec); err != nil {
			return ierr.New(ierr.ErrValidation, fmt.Sprintf("invalid cron spec: %s", *params.Spec))
		}
	}

	// 查询当前任务信息，校验核心任务不允许关闭
	if params.Enabled != nil && !*params.Enabled {
		job, err := h.repo.Get(ctx, name)
		if err != nil {
			return err
		}
		if job.Type == "core" {
			return ierr.New(ierr.ErrValidation, "core cron job cannot be disabled")
		}
	}

	// DB 更新
	if err := h.repo.Update(ctx, name, params); err != nil {
		return err
	}

	// 运行时热重载
	if h.manager != nil {
		job, err := h.repo.Get(ctx, name)
		if err != nil {
			return err
		}

		if !job.Enabled {
			return h.manager.Disable(name)
		}

		specChanged := params.Spec != nil
		enabledFromFalse := params.Enabled != nil && *params.Enabled

		if specChanged {
			return h.manager.Restart(name, job.Spec)
		}
		if enabledFromFalse {
			return h.manager.Enable(name, job.Spec)
		}
	}

	return nil
}
```

- [ ] **Step 2: 在 port/handler.go 新增 CronManager 接口**

```go
// CronManager cron 实例热重载管理器接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronManager interface {
	Restart(name string, newSpec string) error
	Disable(name string) error
	Enable(name string, spec string) error
}
```

---

### Task 8: DTO 层更新

**Files:**
- Modify: `internal/dto/cron.go`

- [ ] **Step 1: CronJobItem 新增 Type，UpdateCronJobReqBody 改指针字段**

```go
type CronJobItem struct {
	Name        string    `json:"name" doc:"任务名"`
	Type        string    `json:"type" doc:"任务类型: functional/core"`
	Spec        string    `json:"spec" doc:"cron 表达式"`
	Description string    `json:"description" doc:"任务描述"`
	Enabled     bool      `json:"enabled" doc:"是否启用"`
	CreatedAt   time.Time `json:"createdAt" doc:"创建时间"`
	UpdatedAt   time.Time `json:"updatedAt" doc:"更新时间"`
}

type UpdateCronJobReqBody struct {
	Enabled *bool   `json:"enabled,omitempty" doc:"是否启用"`
	Spec    *string `json:"spec,omitempty" doc:"cron 表达式，如 */5 * * * *"`
}
```

---

### Task 9: Handler 层适配

**Files:**
- Modify: `internal/handler/cron.go`

- [ ] **Step 1: HandleListCronJobs 传递 Type**

在 `lo.Map` 中新增 `Type: job.Type`

- [ ] **Step 2: HandleUpdateCronJob 适配新参数**

```go
func (h *cronHandler) HandleUpdateCronJob(ctx context.Context, req *dto.UpdateCronJobReq) (*dto.HTTPResponse[*dto.UpdateCronJobRsp], error) {
	rsp := &dto.UpdateCronJobRsp{}
	if req.Body == nil {
		rsp.Error = ierr.ErrValidation.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	// 至少传一个字段
	if req.Body.Enabled == nil && req.Body.Spec == nil {
		rsp.Error = ierr.ErrValidation.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	params := port.UpdateCronJobParams{
		Enabled: req.Body.Enabled,
		Spec:    req.Body.Spec,
	}
	if err := h.updateCronJob.Handle(ctx, req.Name, params); err != nil {
		logger.WithCtx(ctx).Error("[CronHandler] Update cron job failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

需在 handler 文件中 import `cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"`。

---

### Task 10: Bootstrap 层 — 初始化 CronManager 和 DI

**Files:**
- Modify: `internal/bootstrap/modules/cron.go`
- Modify: `internal/bootstrap/lifecycle.go`

- [ ] **Step 1: 在 cron module 中初始化 CronManager 并注入**

在 `internal/bootstrap/modules/cron.go` 中：
- 新增 `CronManager` 的 `fx.Provide`
- 修改 `NewUpdateCronJobHandler` 的 DI 调用，传入 `CronManager`
- `InitCronJobs` 调用时传入 `manager`

- [ ] **Step 2: 使用 CronManager.StopAll() 替代现有停止逻辑**

在 `internal/bootstrap/lifecycle.go` 的关闭流程中，将手动 stop crons 改为调用 `manager.StopAll()`。

---

### Task 11: 前端 — 安装 cronstrue 依赖

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: 安装 cronstrue**

```bash
cd web && npm install cronstrue && npm install -D @types/cronstrue
```

---

### Task 12: 前端 — 更新 TS 类型

**Files:**
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: 更新 CronJobItem 和 UpdateCronJobReqBody**

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

---

### Task 13: 前端 — 更新 api-client

**Files:**
- Modify: `web/src/lib/api-client.ts`

- [ ] **Step 1: updateCronJob 请求体支持 spec**

```typescript
async updateCronJob(body: UpdateCronJobReqBody): Promise<CommonRsp> {
  const payload: Record<string, unknown> = {};
  if (body.enabled !== undefined) payload.enabled = body.enabled;
  if (body.spec !== undefined) payload.spec = body.spec;
  return this.request<CommonRsp>(`/api/v1/cron?name=${encodeURIComponent(body.name)}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}
```

注意路由是 `PATCH /api/v1/cron?name=xxx`（name 在 query 参数），对应 DTO 中 `UpdateCronJobReq.Name` 的 `query:"name"` tag。

---

### Task 14: 前端 — 交互式调度编辑器组件

**Files:**
- Create: `web/src/components/cron/schedule-editor.tsx`

- [ ] **Step 1: 创建 ScheduleEditorDialog 组件**

组件功能：
- 接收 `open`, `onOpenChange`, `job: CronJobItem`, `onSave` props
- Repeat 模式选择：Every Minute / Every Hour / Every Day / Every Week / Every Month / Advanced
- 根据模式动态渲染控件：
  - Every Minute：无额外控件
  - Every Hour：分钟输入 (0-59)
  - Every Day：时 + 分
  - Every Week：星期 + 时 + 分
  - Every Month：日期 + 时 + 分
  - Advanced：cron 表达式输入框 + cronstrue 人类可读预览
- 初始化时根据 job.spec 解析当前模式
- 保存时将控件值转为 cron 表达式，调用 `onSave(spec)`

核心转换逻辑：

```typescript
import cronstrue from "cronstrue";

type RepeatMode = "minute" | "hour" | "day" | "week" | "month" | "advanced";

function specToMode(spec: string): { mode: RepeatMode; minute: number; hour: number; dayOfMonth: number; dayOfWeek: number } {
  const parts = spec.split(" ");
  // 5 字段: minute hour dom month dow
  if (parts[0] === "*" && parts[1] === "*" && parts[2] === "*" && parts[3] === "*" && parts[4] === "*") {
    return { mode: "minute", minute: 0, hour: 0, dayOfMonth: 1, dayOfWeek: 0 };
  }
  if (parts[1] === "*" && parts[2] === "*" && parts[3] === "*" && parts[4] === "*") {
    return { mode: "hour", minute: parseInt(parts[0]), hour: 0, dayOfMonth: 1, dayOfWeek: 0 };
  }
  if (parts[2] === "*" && parts[3] === "*" && parts[4] === "*") {
    return { mode: "day", minute: parseInt(parts[0]), hour: parseInt(parts[1]), dayOfMonth: 1, dayOfWeek: 0 };
  }
  if (parts[2] === "*" && parts[3] === "*" && parts[4] !== "*") {
    return { mode: "week", minute: parseInt(parts[0]), hour: parseInt(parts[1]), dayOfMonth: 1, dayOfWeek: parseInt(parts[4]) };
  }
  if (parts[3] === "*" && parts[4] === "*") {
    return { mode: "month", minute: parseInt(parts[0]), hour: parseInt(parts[1]), dayOfMonth: parseInt(parts[2]), dayOfWeek: 0 };
  }
  return { mode: "advanced", minute: 0, hour: 0, dayOfMonth: 1, dayOfWeek: 0 };
}

function modeToSpec(mode: RepeatMode, minute: number, hour: number, dayOfMonth: number, dayOfWeek: number): string {
  switch (mode) {
    case "minute": return "* * * * *";
    case "hour": return `${minute} * * * *`;
    case "day": return `${minute} ${hour} * * *`;
    case "week": return `${minute} ${hour} * * ${dayOfWeek}`;
    case "month": return `${minute} ${hour} ${dayOfMonth} * *`;
    case "advanced": return ""; // 由用户手填
  }
}
```

UI 使用 shadcn/ui 的 `Dialog`, `Select`, `Input`, `Button` 组件。

---

### Task 15: 前端 — 更新 Cron 页面

**Files:**
- Modify: `web/src/app/(dashboard)/cron/page.tsx`

- [ ] **Step 1: 表格显示增强**

1. 新增 `Type` 列，显示为 badge（`core` 用醒目色，`functional` 用默认色）
2. Spec 列改为人类可读描述（用 cronstrue）+ 下方小字显示原始 cron 表达式
3. Spec 列旁新增编辑图标按钮，点击打开 ScheduleEditorDialog
4. Enabled 列：核心 cron 的 Switch 设为 disabled + 锁定图标

- [ ] **Step 2: 集成 ScheduleEditorDialog**

```typescript
const [editingJob, setEditingJob] = useState<CronJobItem | null>(null);
```

点击编辑图标时 `setEditingJob(job)`，Dialog 的 `onSave` 回调调用 `api.updateCronJob({ name: job.name, spec })` 并刷新列表。

- [ ] **Step 3: handleToggle 区分核心任务**

在 `handleToggle` 中，如果 `job.type === "core"` 则不允许关闭，显示 toast 提示。

---

### Task 16: 编译验证与 Lint

- [ ] **Step 1: 后端编译**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api && make build
```

- [ ] **Step 2: 后端 lint**

```bash
make lint
```

- [ ] **Step 3: 前端构建**

```bash
make web-build
```

- [ ] **Step 4: 前端 lint**

```bash
cd web && npm run lint
```

---

### Task 16b: CronManager 多 Pod 同步（Redis Pub/Sub）

**问题**：K8s 部署有 2 个副本，每个 Pod 独立运行 cron 调度器。`PATCH /api/v1/cron` 请求只命中一个 Pod，`CronManager.Restart()` 仅重启本地实例，另一 Pod 继续使用旧 schedule。

**Files:**
- Modify: `internal/common/constant/string.go`
- Modify: `internal/cron/manager.go`
- Modify: `internal/bootstrap/modules/cron.go`

- [ ] **Step 1: 在 string.go 新增 Redis 频道常量**

```go
CronReloadChannel = "cron:reload"
```

- [ ] **Step 2: CronManager 新增 podID、pubSub 字段和广播逻辑**

```go
type CronManager struct {
    mu      sync.RWMutex
    entries map[string]*managedEntry
    deps    CronDeps
    podID   string
    pubSub  *redis.PubSub
}
```

- `NewCronManager`: 从 `os.Hostname()` 获取 `podID`
- `StartListener(ctx)`: 订阅 `cron:reload`，goroutine 处理消息
- `publish(action, name)`: 向 `cron:reload` 发布 JSON 消息 `{"action":"restart","name":"x","pod":"y"}`
- `handleMessage(msg)`: 解析消息，跳过自身发出的，从 `cronJobStore.Get()` 读最新状态，调用对应的 `restartLocked`/`disableLocked`/`enableLocked`
- `Restart` → 调用 `restartLocked` → 成功后 `publish("restart", name)`
- `Disable` → 调用 `disableLocked` → 成功后 `publish("disable", name)`
- `Enable` → 调用 `enableLocked` → 成功后 `publish("enable", name)`

- [ ] **Step 3: bootstrap 中启动 listener**

```go
func NewCronManager(...) *cron.CronManager {
    m := cron.NewCronManager(cron.CronDeps{...})
    m.StartListener(context.Background())
    return m
}
```

---

### Task 17: 端到端验证

- [ ] **Step 1: 启动服务**

```bash
go run main.go server start
```

- [ ] **Step 2: 验证 API**

1. `GET /api/v1/cron/list` — 确认返回 `type` 字段
2. `PATCH /api/v1/cron?name=SessionDeduplicateCron` body `{"spec": "*/30 * * * *"}` — 修改 spec
3. `GET /api/v1/cron/list` — 确认 spec 已更新
4. `PATCH /api/v1/cron?name=BlockedHitSyncCron` body `{"enabled": false}` — 确认返回验证错误
5. `PATCH /api/v1/cron?name=BlockedHitSyncCron` body `{"spec": "*/10 * * * *"}` — 核心 cron 改 spec 成功

- [ ] **Step 3: 前端验证**

打开 `/web/cron/` 页面，确认：
- Type badge 正确显示
- 点击编辑图标弹出 Dialog
- 修改重复模式后 Save 成功
- 核心 cron 的 Switch 禁用且带锁定图标
