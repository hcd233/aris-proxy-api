# 支持手动触发 Cron Job 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 管理后台支持手动触发 cron job：绕过调度器立即执行一次，审计记录区分 `scheduled`/`manual`，任务正在执行（拿不到锁）时忽略并提示。

**Architecture:** 复用现有 `wrapCronFunc` 的执行骨架（traceID + panic 恢复 + Redis 分布式锁 + 审计）。`Cron` 接口新增 `Trigger() bool`，各任务实现保存锁 key 并委托给新函数 `TriggerWithLock`（同步拿锁、异步执行、拿不到锁零记录）。`CronManager.Trigger(name)` 暴露给 HTTP 层：任务不存在返回 `ErrDataNotExists`，锁被占用返回 `ErrResourceLocked`。审计表新增 `trigger_source` 列。

**Tech Stack:** Go + robfig/cron v3 + Redis 锁 + huma v2 + fx DI；前端 Next.js + TypeScript + shadcn/ui。

**Spec:** `docs/superpowers/specs/2026-08-05-cron-manual-trigger-design.md`

## Global Constraints

- 所有 Go 代码遵循项目风格：samber/lo、samber/mo 可用；函数注释带 `@author`/`@update` 头
- 手动触发**跳过** DB enabled 检查（disabled 任务也可触发）；定时触发行为不变
- 拿不到锁：忽略执行、前端提示、后端**零记录**（不写 skipped 审计）
- 手动触发**不**走 pub/sub 跨 Pod 广播
- 全部测试/命令前缀 `rtk`（RTK 过滤）
- 提交前必须先 `serena_write_memory` 沉淀工程经验

---

### Task 1: 触发来源常量与审计链路字段打通

**Files:**
- Modify: `internal/common/constant/cron.go`
- Modify: `internal/common/constant/sql.go`
- Modify: `internal/infrastructure/database/model/cron.go`
- Modify: `internal/application/cronaudit/port/handler.go`
- Modify: `internal/infrastructure/repository/cron_audit_repository.go`
- Modify: `internal/dto/cron.go`
- Modify: `internal/handler/cron.go`

**Interfaces:**
- Produces: 常量 `constant.CronTriggerSourceScheduled = "scheduled"` / `constant.CronTriggerSourceManual = "manual"`；`CronCallAuditView.TriggerSource string`；`CronCallAuditItem.TriggerSource string`（json `triggerSource`）；DB 列 `trigger_source`（Task 2 的 `saveCronCallAudit` 会写入）

- [ ] **Step 1: 常量新增**

`internal/common/constant/cron.go` 的 const 块内追加：

```go
	// CronTriggerSourceScheduled 定时触发
	CronTriggerSourceScheduled = "scheduled"
	// CronTriggerSourceManual 手动触发
	CronTriggerSourceManual = "manual"
```

`internal/common/constant/sql.go` 在 `FieldCronName`（第 37 行）附近加：

```go
	FieldTriggerSource = "trigger_source"
```

并把 `CronCallAuditRepoFields`（约 270 行）中 `FieldStatus` 之后插入 `FieldTriggerSource`：

```go
	CronCallAuditRepoFields = []string{
		CronCallAuditRepoFieldIDQualified,
		FieldCronName,
		FieldTraceID,
		FieldStartedAt,
		FieldEndedAt,
		FieldDurationMs,
		FieldStatus,
		FieldTriggerSource,
		FieldMessage,
		FieldMetadata,
		CronCallAuditRepoFieldCreatedAtQualified,
	}
```

- [ ] **Step 2: DB 模型加列**

`internal/infrastructure/database/model/cron.go` 的 `CronCallAudit` struct，`Status` 字段后新增：

```go
	TriggerSource string                       `json:"trigger_source" gorm:"column:trigger_source;not null;default:scheduled;comment:触发来源:scheduled/manual"`
```

（GORM AutoMigrate 自动加列，存量行默认 `scheduled`，无需迁移脚本。）

- [ ] **Step 3: 视图与仓储透传**

`internal/application/cronaudit/port/handler.go` 的 `CronCallAuditView` 加：

```go
	TriggerSource string
```

`internal/infrastructure/repository/cron_audit_repository.go`：
- `Save` 的 record 构造加 `TriggerSource: audit.TriggerSource,`
- `List` 的 views 映射加 `TriggerSource: rec.TriggerSource,`

- [ ] **Step 4: DTO 与 handler 映射**

`internal/dto/cron.go` 的 `CronCallAuditItem` 加：

```go
	TriggerSource string `json:"triggerSource" doc:"触发来源:scheduled/manual"`
```

`internal/handler/cron.go` 的 `HandleListCronCallAudits` 映射加：

```go
			TriggerSource: log.TriggerSource,
```

- [ ] **Step 5: 编译验证**

Run: `rtk go build ./...`
Expected: 无错误

- [ ] **Step 6: Commit**

```bash
rtk git add internal/common/constant/cron.go internal/common/constant/sql.go internal/infrastructure/database/model/cron.go internal/application/cronaudit/port/handler.go internal/infrastructure/repository/cron_audit_repository.go internal/dto/cron.go internal/handler/cron.go
rtk git commit -m "feat(cron): add trigger_source column to cron call audit chain"
```

---

### Task 2: TriggerWithLock + Cron 接口 Trigger() + 4 个任务实现

**Files:**
- Modify: `internal/cron/lock_runner.go`
- Modify: `internal/cron/cron.go`（Cron 接口）
- Modify: `internal/cron/session_dedup.go`
- Modify: `internal/cron/soft_delete_purge.go`
- Modify: `internal/cron/think_extract.go`
- Modify: `internal/cron/blocked_hit_sync.go`
- Test: `test/unit/cron/lock_runner_test.go`（追加）
- Test: `test/unit/cron/cron_test.go`（mockCron 补 Trigger）

**Interfaces:**
- Consumes: Task 1 的 `constant.CronTriggerSourceScheduled` / `CronTriggerSourceManual`
- Produces: `Cron.Trigger() bool`；`TriggerWithLock(name string, locker lock.Locker, key string, opts LockOptions, fn func(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error)) bool`；`wrapCronFunc` 增加第 6 参 `source string`

- [ ] **Step 1: lock_runner.go — 加 source 参数与 TriggerWithLock**

`internal/cron/lock_runner.go`：
1. `saveCronCallAudit` 签名加 `source string`，构造 `CronCallAuditView` 时填 `TriggerSource: source`：
2. `cronPanicHandler` 签名加 `source string`，调用 `saveCronCallAudit` 时传它
3. `wrapCronFunc` 签名加 `source string` 参数，内部两处 `saveCronCallAudit(...)` 与 `cronPanicHandler(ctx, name, r)` 调用补传 `source`
4. 文件末尾新增：

```go
// TriggerWithLock 手动触发：同步获取分布式锁，拿到锁后在后台 goroutine 执行 fn（含锁续期、
// panic 恢复与审计，source=manual），立即返回 true；拿不到锁或加锁失败返回 false，不产生任何记录。
//
//	@author centonhuang
//	@update 2026-08-05 10:00:00
func TriggerWithLock(name string, locker lock.Locker, key string, opts LockOptions,
	fn func(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error)) bool {

	ctx := context.WithValue(getBootstrapContext(), constant.CtxKeyTraceID, uuid.New().String())
	log := logger.WithCtx(ctx)

	ttl := opts.TTL
	if ttl <= 0 {
		ttl = constant.CronLockDefaultTTL
	}
	renew := opts.RenewInterval
	if renew <= 0 {
		renew = ttl / constant.CronLockDefaultRenewDivisor
	}

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	value := uuid.New().String()
	locked, err := locker.Lock(childCtx, key, value, ttl)
	if err != nil {
		log.Error("[CronTrigger] Lock acquire error", zap.String("key", key), zap.Error(err))
		return false
	}
	if !locked {
		log.Info("[CronTrigger] Lock held by another instance, skip manual trigger", zap.String("key", key))
		return false
	}

	go func() {
		start := time.Now()
		var (
			metadata *commonmodel.CronCallAuditMetadata
			fnErr    error
		)
		defer func() {
			if r := recover(); r != nil {
				cronPanicHandler(ctx, name, r, constant.CronTriggerSourceManual)
				return
			}
			durationMs := time.Since(start).Milliseconds()
			if fnErr != nil {
				saveCronCallAudit(ctx, name, constant.CronCallAuditStatusFailed, durationMs, fnErr.Error(), nil, constant.CronTriggerSourceManual)
				return
			}
			saveCronCallAudit(ctx, name, constant.CronCallAuditStatusSuccess, durationMs, "", metadata, constant.CronTriggerSourceManual)
		}()
		go renewLoop(childCtx, locker, key, value, ttl, renew)
		metadata, fnErr = fn(childCtx)
	}()

	return true
}
```

- [ ] **Step 2: Cron 接口加 Trigger()**

`internal/cron/cron.go` 的 `Cron` 接口新增：

```go
	// Trigger 手动触发一次；返回是否成功获取锁并启动执行
	Trigger() bool
```

- [ ] **Step 3: 4 个任务实现加 lockKey 字段与 Trigger()**

每个文件两处改动（以 `session_dedup.go` 为例，其余 3 个同构替换模块名/业务 fn）：

`session_dedup.go` struct 加字段（`locker` 之后）：

```go
	lockKey  string
```

`Start` 改为：

```go
func (c *SessionDeduplicateCron) Start(spec string) error {
	c.lockKey = fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionDeduplicate)
	entryID, err := c.cron.AddFunc(spec, wrapCronFunc(constant.CronModuleSessionDeduplicate, c.locker, c.lockKey, LockOptions{}, c.deduplicate, constant.CronTriggerSourceScheduled))
	if err != nil {
		logger.Logger().Error("[SessionDeduplicateCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionDeduplicateCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// Trigger 手动触发一次 Session 去重
//
//	@receiver c *SessionDeduplicateCron
//	@return bool
func (c *SessionDeduplicateCron) Trigger() bool {
	return TriggerWithLock(constant.CronModuleSessionDeduplicate, c.locker, c.lockKey, LockOptions{}, c.deduplicate)
}
```

对应替换映射：
- `soft_delete_purge.go`：struct 名 `SoftDeletePurgeCron`，模块 `CronModuleSoftDeletePurge`，业务 fn `c.purge`
- `think_extract.go`：struct 名 `ThinkExtractCron`，模块 `CronModuleThinkExtract`，业务 fn `c.extract`
- `blocked_hit_sync.go`：struct 名 `blockedHitSyncCron`，模块 `CronModuleBlockedHitSync`，业务 fn `c.sync`（该文件无 logger 错误分支，保持原样风格）

- [ ] **Step 4: 更新 mockCron**

`test/unit/cron/cron_test.go` 的 `mockCron` 加字段与方法：

```go
type mockCron struct {
	started       bool
	stopped       bool
	triggerResult bool
}

func (m *mockCron) Trigger() bool {
	return m.triggerResult
}
```

- [ ] **Step 5: 追加 TriggerWithLock 单测**

`test/unit/cron/lock_runner_test.go` 追加（复用文件内已有的 `newRealLocker`）：

```go
func TestTriggerWithLock_LockHeld_ReturnsFalse(t *testing.T) {
	t.Parallel()
	locker, mr := newRealLocker(t)
	key := "test:trigger-held"
	mr.Set(key, "other-instance")

	got := cron.TriggerWithLock("TestCron", locker, key, cron.LockOptions{}, func(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
		t.Fatal("fn must not run when lock held")
		return nil, nil
	})
	if got {
		t.Fatal("TriggerWithLock must return false when lock held")
	}
}

func TestTriggerWithLock_Acquired_RunsAsync(t *testing.T) {
	t.Parallel()
	locker, _ := newRealLocker(t)
	key := "test:trigger-async"

	ran := make(chan struct{})
	got := cron.TriggerWithLock("TestCron", locker, key, cron.LockOptions{
		TTL:           500 * time.Millisecond,
		RenewInterval: 100 * time.Millisecond,
	}, func(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
		close(ran)
		return &commonmodel.CronCallAuditMetadata{}, nil
	})
	if !got {
		t.Fatal("TriggerWithLock must return true when lock acquired")
	}

	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("fn must run asynchronously after lock acquired")
	}
}
```

补充 import `commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"`。

- [ ] **Step 6: 运行测试**

Run: `rtk go test ./test/unit/cron/...`
Expected: 全部 PASS（含既有 RunWithLock 测试回归）

- [ ] **Step 7: 编译验证**

Run: `rtk go build ./...`
Expected: 无错误

- [ ] **Step 8: Commit**

```bash
rtk git add internal/cron/ test/unit/cron/
rtk git commit -m "feat(cron): support manual trigger via TriggerWithLock and Cron.Trigger"
```

---

### Task 3: CronManager.Trigger + 应用层 handler + DTO + 路由 + DI

**Files:**
- Modify: `internal/cron/manager.go`
- Modify: `internal/application/cronmgmt/port/handler.go`
- Create: `internal/application/cronmgmt/command/trigger_cron_job.go`
- Modify: `internal/dto/cron.go`
- Modify: `internal/handler/cron.go`
- Modify: `internal/router/cron.go`
- Modify: `internal/bootstrap/modules/application.go`
- Modify: `internal/bootstrap/modules/handler.go`
- Test: `test/unit/cron/cron_handler_test.go`（追加）

**Interfaces:**
- Consumes: Task 2 的 `Cron.Trigger() bool`
- Produces: `CronManager.Trigger(name string) error`；`cronmgmtport.TriggerCronJobHandler`；`dto.TriggerCronJobReq{Name string query:"name"}`；HTTP `POST /api/v1/cron/trigger?name=xxx`

- [ ] **Step 1: CronManager.Trigger**

`internal/cron/manager.go` 新增（放在 `Enable` 之后）：

```go
// Trigger 手动触发指定 cron 任务。
// 任务未注册返回 ErrDataNotExists；目标正在执行（拿不到锁）返回 ErrResourceLocked。
// 触发不是配置变更，不做 pub/sub 广播；锁保证单实例执行。
//
//	@receiver m *CronManager
//	@param name string
//	@return error
func (m *CronManager) Trigger(name string) error {
	m.mu.RLock()
	entry, ok := m.entries[name]
	m.mu.RUnlock()
	if !ok {
		return ierr.New(ierr.ErrDataNotExists, "cron job "+name+" not found in manager")
	}
	if !entry.cron.Trigger() {
		return ierr.New(ierr.ErrResourceLocked, "cron job "+name+" is already running")
	}
	logger.Logger().Info("[CronManager] Manually triggered cron job", zap.String("name", name))
	return nil
}
```

- [ ] **Step 2: cronmgmt port 接口**

`internal/application/cronmgmt/port/handler.go`：
1. `CronManager` 接口加 `Trigger(name string) error`
2. 新增接口：

```go
// TriggerCronJobHandler 手动触发 CronJob 处理器接口
//
//	@author centonhuang
//	@update 2026-08-05 10:00:00
type TriggerCronJobHandler interface {
	Handle(ctx context.Context, name string) error
}
```

- [ ] **Step 3: 新建 command**

`internal/application/cronmgmt/command/trigger_cron_job.go`：

```go
package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// triggerCronJobHandler 手动触发 CronJob 处理器
//
//	@author centonhuang
//	@update 2026-08-05 10:00:00
type triggerCronJobHandler struct {
	manager port.CronManager
}

// NewTriggerCronJobHandler 构造手动触发 CronJob 处理器
//
//	@param manager port.CronManager
//	@return port.TriggerCronJobHandler
func NewTriggerCronJobHandler(manager port.CronManager) port.TriggerCronJobHandler {
	return &triggerCronJobHandler{manager: manager}
}

// Handle 处理手动触发 CronJob 请求
//
//	@receiver h *triggerCronJobHandler
//	@param ctx context.Context
//	@param name string
//	@return error
func (h *triggerCronJobHandler) Handle(ctx context.Context, name string) error {
	if h.manager == nil {
		return ierr.New(ierr.ErrInternal, "cron manager not initialized")
	}
	return h.manager.Trigger(name)
}
```

- [ ] **Step 4: DTO**

`internal/dto/cron.go` 的 `UpdateCronJobRsp` 后新增：

```go
// TriggerCronJobReq 手动触发 CronJob 请求
//
//	@author centonhuang
//	@update 2026-08-05 10:00:00
type TriggerCronJobReq struct {
	Name string `query:"name" required:"true" maxLength:"100" doc:"任务名"`
}

// TriggerCronJobRsp 手动触发 CronJob 响应
//
//	@author centonhuang
//	@update 2026-08-05 10:00:00
type TriggerCronJobRsp struct {
	CommonRsp
}
```

- [ ] **Step 5: handler**

`internal/handler/cron.go`：
1. `CronHandler` 接口加 `HandleTriggerCronJob(ctx context.Context, req *dto.TriggerCronJobReq) (*dto.HTTPResponse[*dto.TriggerCronJobRsp], error)`
2. `CronDependencies` 加 `TriggerCronJob cronmgmtport.TriggerCronJobHandler`
3. `cronHandler` struct 加 `triggerCronJob cronmgmtport.TriggerCronJobHandler`，`NewCronHandler` 赋值
4. 新增方法：

```go
func (h *cronHandler) HandleTriggerCronJob(ctx context.Context, req *dto.TriggerCronJobReq) (*dto.HTTPResponse[*dto.TriggerCronJobRsp], error) {
	rsp := &dto.TriggerCronJobRsp{}
	if err := h.triggerCronJob.Handle(ctx, req.Name); err != nil {
		logger.WithCtx(ctx).Error("[CronHandler] Trigger cron job failed",
			zap.String("name", req.Name), zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 6: 路由**

`internal/router/cron.go` 的 `UpdateCronJob` 注册后新增：

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

- [ ] **Step 7: DI 注册**

`internal/bootstrap/modules/application.go`：
1. Provide 列表（`NewUpdateCronJobHandler` 附近）加 `NewTriggerCronJobHandler,`
2. 函数定义（`NewUpdateCronJobHandler` 之后）：

```go
func NewTriggerCronJobHandler(manager *cronpkg.CronManager) cronmgmtport.TriggerCronJobHandler {
	return cronmgmtcommand.NewTriggerCronJobHandler(manager)
}
```

`internal/bootstrap/modules/handler.go` 的 `NewCronDependencies` 改为：

```go
func NewCronDependencies(
	listJobs cronmgmtport.ListCronJobsHandler,
	updateJob cronmgmtport.UpdateCronJobHandler,
	triggerJob cronmgmtport.TriggerCronJobHandler,
	listAudits cronauditport.ListCronCallAuditsHandler,
	listAuditOpts cronauditport.ListCronCallAuditOptionsHandler,
) handler.CronDependencies {
	return handler.CronDependencies{
		ListCronJobs:             listJobs,
		UpdateCronJob:            updateJob,
		TriggerCronJob:           triggerJob,
		ListCronCallAudits:       listAudits,
		ListCronCallAuditOptions: listAuditOpts,
	}
}
```

- [ ] **Step 8: handler 单测**

`test/unit/cron/cron_handler_test.go` 追加：

```go
type fakeTriggerCronJobHandler struct {
	handleErr error
}

func (f *fakeTriggerCronJobHandler) Handle(ctx context.Context, name string) error {
	return f.handleErr
}

func TestCronHandler_TriggerCronJob_Success(t *testing.T) {
	h := handler.NewCronHandler(handler.CronDependencies{
		TriggerCronJob: &fakeTriggerCronJobHandler{},
	})
	rsp, err := h.HandleTriggerCronJob(context.Background(), &dto.TriggerCronJobReq{Name: "TestCron"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rsp == nil {
		t.Fatal("response must not be nil")
	}
}

func TestCronHandler_TriggerCronJob_Error(t *testing.T) {
	h := handler.NewCronHandler(handler.CronDependencies{
		TriggerCronJob: &fakeTriggerCronJobHandler{
			handleErr: ierr.New(ierr.ErrResourceLocked, "already running"),
		},
	})
	_, err := h.HandleTriggerCronJob(context.Background(), &dto.TriggerCronJobReq{Name: "TestCron"})
	if err == nil {
		t.Fatal("expected error when trigger handler fails")
	}
}
```

补 import：`"github.com/hcd233/aris-proxy-api/internal/common/ierr"`。

- [ ] **Step 9: 编译 + 测试**

Run: `rtk go build ./... && rtk go test ./test/unit/cron/...`
Expected: 无错误，全部 PASS

- [ ] **Step 10: Commit**

```bash
rtk git add internal/cron/manager.go internal/application/cronmgmt/ internal/dto/cron.go internal/handler/cron.go internal/router/cron.go internal/bootstrap/modules/ test/unit/cron/cron_handler_test.go
rtk git commit -m "feat(cron): add POST /cron/trigger endpoint for manual trigger"
```

---

### Task 4: E2E 测试

**Files:**
- Create: `test/e2e/cron_trigger/cron_trigger_test.go`

**Interfaces:**
- Consumes: Task 3 的 `POST /api/v1/cron/trigger?name=xxx` 与既有 `GET /api/v1/cron/list`、`GET /api/v1/cron/log/list`

- [ ] **Step 1: 编写 E2E**

参照 `test/e2e/model_capabilities/model_capabilities_test.go` 的 HTTP 客户端骨架（`mustE2EEnv` 读 `BASE_URL`/`JWT_TOKEN`），新建：

```go
// Package cron_trigger 验证 cron 手动触发的全链路行为。
//
// 环境变量：
//   - BASE_URL   API 根地址（必填）
//   - JWT_TOKEN  管理员 JWT（必填）
package cron_trigger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const e2eHTTPTimeout = 30 * time.Second

func mustE2EEnv(t *testing.T) (baseURL, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || jwtToken == "" {
		t.Skip("BASE_URL and JWT_TOKEN are required")
	}
	return baseURL, jwtToken
}

func doJSON(t *testing.T, method, url, jwtToken string, reqBody map[string]any) (int, string, []byte) {
	t.Helper()
	var body io.Reader
	if reqBody != nil {
		b, _ := sonic.Marshal(reqBody)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: e2eHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("X-Trace-Id"), b
}

type cronJobItem struct {
	Name string `json:"name"`
}
type listCronJobsRsp struct {
	Jobs []cronJobItem `json:"jobs"`
}
type cronAuditItem struct {
	CronName      string `json:"cronName"`
	TriggerSource string `json:"triggerSource"`
	CreatedAt     string `json:"createdAt"`
}
type listCronAuditsRsp struct {
	Logs []cronAuditItem `json:"logs"`
}

func TestE2E_CronManualTrigger_ProducesManualAudit(t *testing.T) {
	baseURL, jwtToken := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 1. 取第一个任务名
	status, _, body := doJSON(t, http.MethodGet, baseURL+"/api/v1/cron/list?page=1&pageSize=20", jwtToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list cron jobs: status=%d body=%s", status, body)
	}
	var listRsp listCronJobsRsp
	if err := sonic.Unmarshal(body, &listRsp); err != nil {
		t.Fatalf("unmarshal list rsp: %v", err)
	}
	if len(listRsp.Jobs) == 0 {
		t.Fatal("no cron jobs found")
	}
	name := listRsp.Jobs[0].Name
	t.Logf("triggering cron job: %s", name)

	// 2. 触发
	status, _, body = doJSON(t, http.MethodPost, baseURL+"/api/v1/cron/trigger?name="+name, jwtToken, nil)
	if status != http.StatusOK {
		t.Fatalf("trigger cron job: status=%d body=%s", status, body)
	}

	// 3. 轮询执行日志，等待出现 triggerSource=manual 的新记录（最长 30s）
	deadline := time.Now().Add(e2eHTTPTimeout)
	start := time.Now().Add(-2 * time.Minute).Format("2006-01-02T15:04:05Z")
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		status, _, body = doJSON(t, http.MethodGet,
			fmt.Sprintf("%s/api/v1/cron/log/list?page=1&pageSize=20&sort=desc&sortField=created_at&startTime=%s", baseURL, start),
			jwtToken, nil)
		if status != http.StatusOK {
			t.Fatalf("list cron audits: status=%d body=%s", status, body)
		}
		var auditRsp listCronAuditsRsp
		if err := sonic.Unmarshal(body, &auditRsp); err != nil {
			t.Fatalf("unmarshal audit rsp: %v", err)
		}
		for _, log := range auditRsp.Logs {
			if log.CronName == name && log.TriggerSource == "manual" {
				t.Logf("found manual audit record: %s", log.CreatedAt)
				return
			}
		}
	}
	t.Fatal("manual trigger audit record not found within timeout")
}

func TestE2E_CronTrigger_NotFound(t *testing.T) {
	baseURL, jwtToken := mustE2EEnv(t)
	status, _, body := doJSON(t, http.MethodPost, baseURL+"/api/v1/cron/trigger?name=non-existent-job", jwtToken, nil)
	if status == http.StatusOK {
		t.Fatalf("expected non-200 for unknown cron job, got status=%d body=%s", status, body)
	}
}
```

- [ ] **Step 2: 编译测试包**

Run: `rtk go vet ./test/e2e/cron_trigger/...`
Expected: 无错误（`go build` 通过；运行需 BASE_URL/JWT_TOKEN 环境变量，接入既有 E2E 运行方式）

- [ ] **Step 3: Commit**

```bash
rtk git add test/e2e/cron_trigger/
rtk git commit -m "test(e2e): cron manual trigger flow"
```

---

### Task 5: 前端

**Files:**
- Modify: `web/src/lib/api-client.ts`
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/app/(dashboard)/cron/page.tsx`
- Modify: `web/src/app/(dashboard)/audit/cron/page.tsx`
- Modify: `web/src/locales/zh.json` / `web/src/locales/en.json` / `web/src/locales/ja.json`

**Interfaces:**
- Consumes: Task 3 的 `POST /api/v1/cron/trigger?name=xxx`（返回 200 或业务错误）；`CronCallAuditItem.triggerSource`
- Produces: `api.triggerCronJob(name: string)`；cron 页「立即执行」按钮 + 确认弹窗；执行日志页「触发来源」列

- [ ] **Step 1: api-client + types**

`web/src/lib/api-client.ts` 的 `updateCronJob` 后新增：

```ts
  async triggerCronJob(name: string): Promise<CommonRsp> {
    return this.request<CommonRsp>(`/api/v1/cron/trigger?name=${encodeURIComponent(name)}`, {
      method: "POST",
    });
  }
```

`web/src/lib/types.ts` 的 `CronCallAuditItem` 加：

```ts
  triggerSource?: "scheduled" | "manual";
```

- [ ] **Step 2: cron 管理页「立即执行」**

`web/src/app/(dashboard)/cron/page.tsx`：
1. import 增加 `Play`（lucide-react）、`DeleteConfirmDialog`（`@/components/delete-confirm-dialog`）、`parseError`（`@/lib/api-errors`）
2. state 增加：

```tsx
  const [triggeringJob, setTriggeringJob] = useState<CronJobItem | null>(null);
  const [triggering, setTriggering] = useState(false);
```

3. 增加处理函数：

```tsx
  const handleTrigger = async () => {
    if (!triggeringJob) return;
    setTriggering(true);
    try {
      const rsp = await api.triggerCronJob(triggeringJob.name);
      if (rsp.error) {
        showErrorToast(rsp.error, { title: t("cron.trigger_error") });
        return;
      }
      toast.success(t("cron.triggered"));
    } catch (err) {
      const parsed = parseError(err);
      if (parsed.code === BusinessErrorCode.ResourceLocked) {
        toast.error(t("cron.running"));
      } else {
        showErrorToast(err, { title: t("cron.trigger_error") });
      }
    } finally {
      setTriggering(false);
      setTriggeringJob(null);
    }
  };
```

4. spec 单元格内 Pencil 按钮旁加 Play 按钮：

```tsx
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7 shrink-0"
                            onClick={() => setTriggeringJob(job)}
                            aria-label={t("cron.trigger")}
                          >
                            <Play className="size-3.5" />
                          </Button>
```

5. `ScheduleEditorDialog` 旁加确认弹窗：

```tsx
        <DeleteConfirmDialog
          open={triggeringJob !== null}
          onOpenChange={(open) => { if (!open) setTriggeringJob(null); }}
          title={t("cron.trigger_confirm_title")}
          description={t("cron.trigger_confirm_desc")}
          confirmLabel={t("cron.trigger")}
          loadingLabel={t("cron.triggering")}
          loading={triggering}
          onConfirm={handleTrigger}
        />
```

import 增补 `BusinessErrorCode`（来自 `@/lib/api-errors`）。

- [ ] **Step 3: 执行日志页「触发来源」列**

`web/src/app/(dashboard)/audit/cron/page.tsx`：
1. 表格 `TableHead` 在「cron_name」列后加 `<TableHead>{t("cron_audit.trigger_source")}</TableHead>`
2. 行内「cronName」单元格后加：

```tsx
                      <TableCell>
                        <Badge variant={log.triggerSource === "manual" ? "default" : "secondary"} className="text-xs">
                          {log.triggerSource === "manual" ? t("cron_audit.trigger_manual") : t("cron_audit.trigger_scheduled")}
                        </Badge>
                      </TableCell>
```

- [ ] **Step 4: i18n 文案（三语）**

`web/src/locales/zh.json` 的 `cron` 块追加：

```json
  "cron.trigger": "立即执行",
  "cron.triggering": "执行中",
  "cron.triggered": "已触发，可在执行日志中查看结果",
  "cron.trigger_error": "触发失败",
  "cron.running": "任务正在执行中，请稍后再试",
  "cron.trigger_confirm_title": "确认立即执行",
  "cron.trigger_confirm_desc": "该任务将立即执行一次，结果可在执行日志中查看"
```

`web/src/locales/en.json` 的 `cron` 块追加：

```json
  "cron.trigger": "Run now",
  "cron.triggering": "Running",
  "cron.triggered": "Triggered. Check the execution logs for the result.",
  "cron.trigger_error": "Failed to trigger",
  "cron.running": "The job is already running, please try again later",
  "cron.trigger_confirm_title": "Run now?",
  "cron.trigger_confirm_desc": "This job will run once immediately. Check the execution logs for the result."
```

`web/src/locales/ja.json` 的 `cron` 块追加：

```json
  "cron.trigger": "今すぐ実行",
  "cron.triggering": "実行中",
  "cron.triggered": "実行を開始しました。実行ログで結果を確認してください。",
  "cron.trigger_error": "実行に失敗しました",
  "cron.running": "タスクが実行中のため、しばらくしてからお試しください",
  "cron.trigger_confirm_title": "今すぐ実行しますか？",
  "cron.trigger_confirm_desc": "このタスクが直ちに1回実行されます。実行ログで結果を確認できます。"
```

`web/src/locales/zh.json` / `en.json` / `ja.json` 的 `cron_audit` 块分别追加：

```json
  "cron_audit.trigger_source": "触发来源",
  "cron_audit.trigger_scheduled": "定时",
  "cron_audit.trigger_manual": "手动"
```

```json
  "cron_audit.trigger_source": "Trigger Source",
  "cron_audit.trigger_scheduled": "Scheduled",
  "cron_audit.trigger_manual": "Manual"
```

```json
  "cron_audit.trigger_source": "トリガー元",
  "cron_audit.trigger_scheduled": "定期",
  "cron_audit.trigger_manual": "手動"
```

- [ ] **Step 5: 前端验证**

Run（在 `web/` 目录）: `rtk pnpm lint && rtk pnpm build`
Expected: 无 lint 错误，构建成功

- [ ] **Step 6: Commit**

```bash
rtk git add web/src/lib/api-client.ts web/src/lib/types.ts "web/src/app/(dashboard)/cron/page.tsx" "web/src/app/(dashboard)/audit/cron/page.tsx" web/src/locales/
rtk git commit -m "feat(web): cron manual trigger button and trigger source column"
```

---

### Task 6: 全量验证与文档回写

**Files:**
- Modify: `CONTEXT.md`

- [ ] **Step 1: 全量测试**

Run: `rtk go build ./... && rtk go test ./...`
Expected: 全部 PASS

- [ ] **Step 2: E2E 运行**

Run（对接既有 E2E 运行环境，提供 `BASE_URL`/`JWT_TOKEN`）:
`rtk go test ./test/e2e/cron_trigger/... -v`
Expected: `TestE2E_CronManualTrigger_ProducesManualAudit` 找到 `triggerSource=manual` 记录；`TestE2E_CronTrigger_NotFound` 返回非 200

- [ ] **Step 3: 回写 CONTEXT.md**

`CONTEXT.md` 的 `Cron & Maintenance` 节补充：

```markdown
**CronTriggerSource（触发来源）**:
cron 任务执行的触发来源枚举：`scheduled`（定时调度触发）与 `manual`（管理后台手动触发，可对 disabled 任务执行，拿不到分布式锁时忽略执行且不产生审计）。写入 `cron_call_audits.trigger_source` 列。
_Avoid_: trigger type, execution source, run source
```

- [ ] **Step 4: 沉淀工程经验**

Run: `serena_write_memory`，主题 `cron/manual-trigger`，记录：手动触发=复用 wrapCronFunc 骨架、同步拿锁异步执行、拿锁失败零记录、无需 pub/sub、`ErrResourceLocked` 语义、审计 trigger_source 列。

- [ ] **Step 5: Commit**

```bash
rtk git add CONTEXT.md
rtk git commit -m "docs: add CronTriggerSource glossary and engineering notes"
```

---

## Self-Review

- **Spec coverage**：disabled 可触发（Task 2 的 TriggerWithLock 无 enabled 检查）✓；异步执行（TriggerWithLock goroutine）✓；审计区分来源（Task 1 + Task 2 source 参数）✓；拿不到锁忽略+提示+零记录（TriggerWithLock 返回 false → ErrResourceLocked → 前端 toast；无审计写入）✓；API/前端入口（Task 3/5）✓；测试（Task 2/3/4/6）✓；YAGNI 排除项均未实现 ✓
- **Placeholder scan**：所有步骤含完整代码/命令，无 TBD ✓
- **Type consistency**：`Trigger() bool`（Task 2）→ `CronManager.Trigger`（Task 3）→ `port.CronManager.Trigger(name string) error`（Task 3 Step 2）；`TriggerWithLock` 签名在 Task 2 定义并在 4 个实现复用；`saveCronCallAudit`/`cronPanicHandler` 加 source 参数在 Task 2 统一 ✓；`TriggerSource` 贯穿 Task 1 各层 ✓
