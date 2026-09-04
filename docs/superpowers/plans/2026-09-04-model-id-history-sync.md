# 模型 ID 历史记录一键同步 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 模型编辑弹窗在 modelId 变化时提供「同步更新历史记录」勾选，一次 PATCH 请求将该模型归属用户名下 `model_call_audit` / `session` / `message` 三表历史数据中的旧 model id 批量替换为新值，并返回各表影响行数。

**Architecture:** 复用现有 `PATCH /api/web/v1/model` 更新链路（dto → handler → command → repo），在 command 层追加历史替换调用。替换逻辑为纯 Go 实现（GORM 单事务 + Go 内数组替换，非 PG jsonb SQL），保证 sqlite 测试基建可运行。scope 严格按模型归属 `m.UserID()` 的 API Key 集合限定，不误伤其他用户同名字符串。

**Tech Stack:** Go + GORM + huma + dig/fx DI；Next.js + TypeScript 前端；测试为 sqlite 内存库 + miniredis 内嵌服务器 e2e（模板 `test/e2e/cross_tenant_reference`）。

**Spec:** `docs/superpowers/specs/2026-09-04-model-id-history-sync-design.md`（含 §5.4 实现方式修订、§6 共享行 trade-off、§5.4 已知边界）

## Global Constraints

- 执行环境：git worktree `.worktrees/model-id-history-sync-2026-09-04`，分支 `feature/model-id-history-sync-2026-09-04`。
- **编写 Go 代码前必须**：加载 `use-modern-go` 并先跑 `list`（`sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <目标文件>`，输出不得截断），再加载 `golang-naming`、`golang-code-style`、`golang-samber-lo`、`golang-samber-mo`；改 `internal/dto/**` 或 huma 路由时加载 `huma-dto-conventions`。
- 业务错误统一走 `internal/common/ierr`（禁止 errors.Join / 直接 fmt.Errorf 返回业务错误）；DTO 层 `any`/`interface{}` 均禁用。
- huma DTO 铁律：JSON body 字段必须包在 `Body *XxxReqBody` 里（现有 `UpdateModelReq` 已是 `query:"id"` + `Body` 包装，只加字段不改结构）。
- 项目硬约束优先于任何 skill 通用建议（详见 `docs/agents/go-backend.md`）。
- 验证命令禁止用 rtk 过滤输出（use-modern-go `list`/`explain`、`go test` 全量跑时）。
- 提交信息用中文 conventional commits（如 `feat(model): ...`）。

## 代码事实速查（实现者必读）

- 更新链路：`internal/handler/model.go:78 HandleUpdateModel` → `internal/application/model/command/update_model.go Handle` → `m.Update(...)` → `repo.Update`。
- `UpdateModelReq` 用 `query:"id"`（非 path）；`Model` 聚合有 `ModelID()` / `UserID()` / `SetModelID()` getter/setter（`internal/domain/llmproxy/aggregate/model.go:82-100`）。
- `ModelRepository` 接口在 `internal/domain/llmproxy/repository.go:34`，实现在 `internal/infrastructure/repository/endpoint_repository.go:227`（`modelRepository{dao, db}`）。
- 表结构：`session.model_ids`（`[]string`，`serializer:json`）、`session.api_key_name`、`message.model_id`（无归属字段）、`model_call_audit.model_id` + `api_key_id`、`proxy_api_keys.user_id`/`name`（`deleted_at` 为 int64 自定义软删列，**无 gorm 自动过滤**，查询天然含已删 key）。
- `BaseModel.DeletedAt` 是 int64 非 `gorm.DeletedAt` → 不需要 `Unscoped()`。
- HTTP 响应包装：`dto.HTTPResponse[BodyT]` 序列化为 `{"data": ...}`（`internal/dto/base.go:12`）；handler 用 `apiutil.WrapHTTPResponse(rsp, nil)`。
- e2e 模板：`test/e2e/cross_tenant_reference/cross_tenant_test.go`（生产路由装配 + JWT + sqlite + miniredis，含 userA/userB/admin 双租户种子）。
- 前端：模型管理在 `web/src/app/(dashboard)/upstream/`（`page.tsx` + `model-dialog.tsx` + `shared.tsx`）；`ModelDialogProps` 已有 `modelIdTouched`；`api.updateModel` 在 `web/src/lib/api-client.ts:618`；`t(key, fallback)` **不支持插值**；i18n 文件 `web/src/locales/{zh,en,ja}.json`（注意 ja 也要加）。

---

### Task 1: Repository 历史替换方法 `ReplaceHistoricalModelID`

**Files:**
- Modify: `internal/domain/llmproxy/repository.go`（接口 + 结果类型）
- Modify: `internal/infrastructure/repository/endpoint_repository.go`（`modelRepository` 实现，追加在文件尾部）
- Test: `test/unit/model_repository/replace_history_test.go`（新建）

**Interfaces:**
- Consumes: `dbmodel.ModelCallAudit` / `dbmodel.Session`（`ModelIDs []string`、`MessageIDs []uint`、`APIKeyName`）/ `dbmodel.Message` / `dbmodel.ProxyAPIKey`（`UserID`、`Name`）。
- Produces（Task 2 依赖，签名必须一致）：

```go
// domain/llmproxy/repository.go 追加（ModelRepository 接口同级）：
type ModelIDSyncCounts struct {
	AuditCount   int64
	SessionCount int64
	MessageCount int64
}

// ModelRepository 接口追加方法：
// ReplaceHistoricalModelID 将归属 userID 的历史数据中业务模型 ID oldID 批量替换为 newID。
// 单事务内依次替换 model_call_audit / session / message 三表，返回各表影响行数；
// message 的 scope 为「实际命中的会话引用到的消息」（见 spec §5.4）。
ReplaceHistoricalModelID(ctx context.Context, userID uint, oldID, newID string) (ModelIDSyncCounts, error)
```

- [ ] **Step 1: 跑 use-modern-go `list`**

```sh
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/infrastructure/repository/endpoint_repository.go
```

读完整输出；对本任务涉及的 guideline 按需 `explain`。

- [ ] **Step 2: 写失败的单测**

新建 `test/unit/model_repository/replace_history_test.go`。sqlite 内存库 + AutoMigrate 种数据，直接调 `repository.NewModelRepository(db)`：

```go
package model_repository

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newHistorySyncDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&dbmodel.ProxyAPIKey{}, &dbmodel.ModelCallAudit{}, &dbmodel.Session{}, &dbmodel.Message{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// 种子：userA 有 key（含已删 key）、audit/session/message 历史均为 oldID；
// userB 有同名 modelId 的 audit（隔离对照）。
func seedHistorySyncData(t *testing.T, db *gorm.DB) (keyAID uint) {
	t.Helper()
	keyA := &dbmodel.ProxyAPIKey{UserID: 1, Name: "key-a"}
	keyADel := &dbmodel.ProxyAPIKey{UserID: 1, Name: "key-a-del", DeletedAt: 100}
	keyB := &dbmodel.ProxyAPIKey{UserID: 2, Name: "key-b"}
	if err := db.Create([]*dbmodel.ProxyAPIKey{keyA, keyADel, keyB}).Error; err != nil {
		t.Fatal(err)
	}
	msg := &dbmodel.Message{ModelID: "old-id", CheckSum: "m1"}
	if err := db.Create(msg).Error; err != nil {
		t.Fatal(err)
	}
	msgB := &dbmodel.Message{ModelID: "old-id", CheckSum: "m2"} // B 引用的独立消息
	if err := db.Create(msgB).Error; err != nil {
		t.Fatal(err)
	}
	sess := &dbmodel.Session{
		APIKeyName: "key-a",
		MessageIDs: []uint{msg.ID},
		ModelIDs:   []string{"old-id", "other-id"},
	}
	sessNoHit := &dbmodel.Session{APIKeyName: "key-a", ModelIDs: []string{"other-id"}}
	if err := db.Create([]*dbmodel.Session{sess, sessNoHit}).Error; err != nil {
		t.Fatal(err)
	}
	audits := []*dbmodel.ModelCallAudit{
		{APIKeyID: keyA.ID, ModelID: "old-id"},
		{APIKeyID: keyADel.ID, ModelID: "old-id"}, // 已删 key 的历史也要替换
		{APIKeyID: keyB.ID, ModelID: "old-id"},    // B 的，不能动
		{APIKeyID: keyA.ID, ModelID: "other-id"},  // 非 old 的不动
	}
	if err := db.Create(audits).Error; err != nil {
		t.Fatal(err)
	}
	return keyA.ID
}

func TestReplaceHistoricalModelID(t *testing.T) {
	db := newHistorySyncDB(t)
	seedHistorySyncData(t, db)
	repo := repository.NewModelRepository(db)

	counts, err := repo.ReplaceHistoricalModelID(context.Background(), 1, "old-id", "new-id")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if counts.AuditCount != 2 || counts.SessionCount != 1 || counts.MessageCount != 1 {
		t.Fatalf("counts mismatch: %+v (want audit=2 session=1 message=1)", counts)
	}

	// audit：A 的两条（含已删 key）变 new-id，B 的仍是 old-id
	var auditNew, auditOld int64
	db.Model(&dbmodel.ModelCallAudit{}).Where("model_id = ?", "new-id").Count(&auditNew)
	db.Model(&dbmodel.ModelCallAudit{}).Where("model_id = ? AND api_key_id IN (SELECT id FROM proxy_api_keys WHERE user_id = 2)", "old-id").Count(&auditOld)
	if auditNew != 2 || auditOld != 1 {
		t.Fatalf("audit isolation broken: new=%d old(B)=%d", auditNew, auditOld)
	}

	// session：数组逐元素替换，other-id 保留；未命中会话不动
	var sess dbmodel.Session
	db.Where("api_key_name = ? AND model_ids LIKE ?", "key-a", "%old-id%").First(&sess)
	if len(sess.ModelIDs) != 2 || sess.ModelIDs[0] != "new-id" || sess.ModelIDs[1] != "other-id" {
		t.Fatalf("session model_ids = %v, want [new-id other-id]", sess.ModelIDs)
	}

	// message：A 会话引用的变 new-id；B 的独立消息仍是 old-id
	var msgNew, msgOld int64
	db.Model(&dbmodel.Message{}).Where("model_id = ? AND check_sum = ?", "new-id", "m1").Count(&msgNew)
	db.Model(&dbmodel.Message{}).Where("model_id = ? AND check_sum = ?", "old-id", "m2").Count(&msgOld)
	if msgNew != 1 || msgOld != 1 {
		t.Fatalf("message isolation broken: new=%d old(B)=%d", msgNew, msgOld)
	}
}

func TestReplaceHistoricalModelID_NoHit(t *testing.T) {
	db := newHistorySyncDB(t)
	repo := repository.NewModelRepository(db)

	counts, err := repo.ReplaceHistoricalModelID(context.Background(), 99, "old-id", "new-id")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	var zero llmproxy.ModelIDSyncCounts
	if counts != zero {
		t.Fatalf("counts = %+v, want zero", counts)
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

```sh
go test ./test/unit/model_repository/ -count=1
```

Expected: FAIL（`ReplaceHistoricalModelID` / `ModelIDSyncCounts` 未定义，编译错误）。

- [ ] **Step 4: 实现接口与实现**

`internal/domain/llmproxy/repository.go`：按上方 Interfaces 块追加 `ModelIDSyncCounts` 类型与接口方法（含注释）。

`internal/infrastructure/repository/endpoint_repository.go` 文件尾部追加（需要新 import：`strings` 可省略——LIKE 预过滤直接绑定原始 oldID，误匹配由 `lo.Contains` 精确兜底，无需转义）：

```go
// ReplaceHistoricalModelID 将归属 userID 的历史数据中业务模型 ID oldID 批量替换为 newID。
//
// 单事务三步（spec 2026-09-04-model-id-history-sync §5.4）：
//  1. audit：api_key_id 关联 user 的全部 key（含已删 key）；
//  2. session：api_key_name 关联，model_ids 数组逐元素替换（LIKE 预过滤 + lo.Contains 精确确认）；
//  3. message：scope 为第 2 步实际命中会话引用到的消息，分块更新。
//
// 纯 Go 实现而非 PG jsonb SQL：sqlite 测试基建可运行，且本操作为偶发管理操作。
// 已知边界：modelId 含引号/反斜杠时 LIKE 预过滤可能漏匹配（JSON 转义字节序列不同），spec §5.4 已记录。
func (r *modelRepository) ReplaceHistoricalModelID(ctx context.Context, userID uint, oldID, newID string) (llmproxy.ModelIDSyncCounts, error) {
	var counts llmproxy.ModelIDSyncCounts
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		counts = llmproxy.ModelIDSyncCounts{}

		// ① audit
		res := tx.Model(&dbmodel.ModelCallAudit{}).
			Where("model_id = ? AND api_key_id IN (?)", oldID,
				tx.Model(&dbmodel.ProxyAPIKey{}).Select("id").Where("user_id = ?", userID)).
			Update("model_id", newID)
		if res.Error != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, res.Error, "replace audit model id")
		}
		counts.AuditCount = res.RowsAffected

		// ② session：预过滤后 Go 内精确替换
		var names []string
		if err := tx.Model(&dbmodel.ProxyAPIKey{}).Where("user_id = ?", userID).
			Distinct().Pluck("name", &names).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBQuery, err, "pluck api key names")
		}
		if len(names) == 0 {
			return nil
		}
		var sessions []dbmodel.Session
		if err := tx.Where("api_key_name IN (?) AND model_ids LIKE ?", names, "%"+oldID+"%").
			Find(&sessions).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBQuery, err, "find sessions with old model id")
		}
		var affectedSessionIDs []uint
		for i := range sessions {
			if !lo.Contains(sessions[i].ModelIDs, oldID) {
				continue
			}
			sessions[i].ModelIDs = lo.Map(sessions[i].ModelIDs, func(id string, _ int) string {
				if id == oldID {
					return newID
				}
				return id
			})
			if err := tx.Model(&dbmodel.Session{}).Where("id = ?", sessions[i].ID).
				Update("model_ids", sessions[i].ModelIDs).Error; err != nil {
				return ierr.Wrap(ierr.ErrDBUpdate, err, "update session model_ids")
			}
			counts.SessionCount++
			affectedSessionIDs = append(affectedSessionIDs, sessions[i].ID)
		}

		// ③ message：命中会话引用到的消息，分块更新
		if len(affectedSessionIDs) == 0 {
			return nil
		}
		var msgIDs []uint
		for i := range sessions {
			msgIDs = append(msgIDs, sessions[i].MessageIDs...)
		}
		for _, chunk := range lo.Chunk(lo.Uniq(msgIDs), 500) {
			res := tx.Model(&dbmodel.Message{}).
				Where("model_id = ? AND id IN (?)", oldID, chunk).
				Update("model_id", newID)
			if res.Error != nil {
				return ierr.Wrap(ierr.ErrDBUpdate, res.Error, "replace message model id")
			}
			counts.MessageCount += res.RowsAffected
		}
		return nil
	})
	if err != nil {
		return llmproxy.ModelIDSyncCounts{}, err
	}
	return counts, nil
}
```

注意：`endpoint_repository.go` 已 import `lo`（samber/lo）与 `dbmodel`/`ierr`/`gorm`，若缺则补。

- [ ] **Step 5: 跑测试确认通过**

```sh
go test ./test/unit/model_repository/ -count=1 -v
```

Expected: 全部 PASS。若 `Update("model_ids", []string{...})` 序列化异常（serializer 未生效），改为整行 `tx.Save(&sessions[i])` 并重跑。

- [ ] **Step 6: Commit**

```sh
git add internal/domain/llmproxy/repository.go internal/infrastructure/repository/endpoint_repository.go test/unit/model_repository/replace_history_test.go
git commit -m "feat(model): Repository 新增 ReplaceHistoricalModelID 历史模型ID批量替换"
```

---

### Task 2: 更新链路串联（DTO / Port / Command / Handler）

**Files:**
- Modify: `internal/dto/model.go`（ReqBody 加字段 + 新响应 Rsp）
- Modify: `internal/application/model/port/handler.go`（Command 加字段 + 结果类型 + 接口签名）
- Modify: `internal/application/model/command/update_model.go`（同步逻辑 + 日志）
- Modify: `internal/handler/model.go`（`HandleUpdateModel` 签名与透传）

**Interfaces:**
- Consumes: Task 1 的 `llmproxy.ModelRepository.ReplaceHistoricalModelID(ctx, userID, old, new) (llmproxy.ModelIDSyncCounts, error)`。
- Produces（Task 3/4 依赖）：
  - HTTP `PATCH /api/web/v1/model?id=`：body 可带 `"syncHistory": true`；响应 `data` 为 `{"auditCount","sessionCount","messageCount"}`（int64）。
  - `port.UpdateModelHandler.Handle(ctx, UpdateModelCommand) (port.UpdateModelResult, error)`（**签名从 `error` 变为 `(UpdateModelResult, error)`**）。

- [ ] **Step 1: 跑 use-modern-go `list` + 加载 huma-dto-conventions**

```sh
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/dto/model.go
```

并读 `.agents/skills/internal/huma-dto-conventions/SKILL.md`。

- [ ] **Step 2: 修改 DTO**

`internal/dto/model.go`：

```go
// UpdateModelReqBody 的 ModelID 字段后追加：
	SyncHistory      *bool                 `json:"syncHistory,omitempty" doc:"modelId 变化时是否同步更新历史记录（audit/session/message）"`

// 文件内追加（DeleteModelReq 之前）：
// ModelUpdateRsp 更新 Model 响应
type ModelUpdateRsp struct {
	AuditCount   int64 `json:"auditCount" doc:"审计记录替换行数"`
	SessionCount int64 `json:"sessionCount" doc:"会话替换行数"`
	MessageCount int64 `json:"messageCount" doc:"消息替换行数"`
}
```

- [ ] **Step 3: 修改 Port**

`internal/application/model/port/handler.go`：

```go
// UpdateModelCommand 追加字段（ModelID 之后）：
	SyncHistory     *bool
// 注释补一句：SyncHistory 为 true 且 ModelID 实际变化时，同步替换归属 user 的历史数据

// 接口同级追加：
// UpdateModelResult 更新结果（含历史同步影响行数；未同步时全 0）
type UpdateModelResult struct {
	AuditCount   int64
	SessionCount int64
	MessageCount int64
}

// UpdateModelHandler 接口签名改为：
type UpdateModelHandler interface {
	Handle(ctx context.Context, cmd UpdateModelCommand) (UpdateModelResult, error)
}
```

- [ ] **Step 4: 修改 Command**

`internal/application/model/command/update_model.go` 的 `Handle`（先写实现骨架再编译）：

```go
func (h *updateModelHandler) Handle(ctx context.Context, cmd port.UpdateModelCommand) (port.UpdateModelResult, error) {
	// ... 前半段（FindByID、endpoint 归属校验）不变 ...

	oldModelID := m.ModelID() // 在 m.Update(...) 之前记录
	if uerr := m.Update(...); uerr != nil { ... } // 不变
	if err := h.repo.Update(ctx, m); err != nil { ... } // 不变

	result := port.UpdateModelResult{}
	if cmd.SyncHistory != nil && *cmd.SyncHistory && cmd.ModelID != nil && *cmd.ModelID != oldModelID {
		counts, serr := h.repo.ReplaceHistoricalModelID(ctx, m.UserID(), oldModelID, *cmd.ModelID)
		if serr != nil {
			// 模型本体已改名、历史未同步：按 spec §7 返回错误，前端提示可重试
			log.Error("[ModelCommand] Replace historical model id failed",
				zap.Uint("id", cmd.ID), zap.Error(serr))
			return port.UpdateModelResult{}, serr
		}
		result = port.UpdateModelResult{
			AuditCount:   counts.AuditCount,
			SessionCount: counts.SessionCount,
			MessageCount: counts.MessageCount,
		}
		log.Info("[ModelCommand] Historical model id synced",
			zap.Uint("id", cmd.ID),
			zap.String("old", oldModelID), zap.String("new", *cmd.ModelID),
			zap.Int64("auditCount", result.AuditCount),
			zap.Int64("sessionCount", result.SessionCount),
			zap.Int64("messageCount", result.MessageCount))
	}

	log.Info("[ModelCommand] Update model success", zap.Uint("id", cmd.ID))
	return result, nil
}
```

注意：`m.UserID()`（模型归属 user，非操作者）作为替换 scope，admin 代管时只作用于模型 owner 数据（spec §9）。

- [ ] **Step 5: 修改 Handler**

`internal/handler/model.go` 的 `HandleUpdateModel`：

```go
func (h *modelHandler) HandleUpdateModel(ctx context.Context, req *dto.UpdateModelReq) (*dto.HTTPResponse[*dto.ModelUpdateRsp], error) {
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	result, err := h.update.Handle(ctx, port.UpdateModelCommand{
		ScopeUserID:     scope,
		ID:              req.ID,
		Alias:           req.Body.Alias,
		UpstreamModel:   req.Body.UpstreamModel,
		EndpointID:      req.Body.EndpointID,
		Enabled:         req.Body.Enabled,
		ContextLength:   req.Body.ContextLength,
		MaxOutputTokens: req.Body.MaxOutputTokens,
		Capabilities:    req.Body.Capabilities,
		ModelID:         req.Body.ModelID,
		SyncHistory:     req.Body.SyncHistory,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Update model failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.ModelUpdateRsp{
		AuditCount:   result.AuditCount,
		SessionCount: result.SessionCount,
		MessageCount: result.MessageCount,
	}, nil)
}
```

同时检查 handler 的 `ModelHandler` 接口声明（同文件顶部）中 `HandleUpdateModel` 的签名同步更新。Router（`internal/router/model.go`）通过 handler 方法签名推断响应类型，无需改动，但编译器会兜底。

- [ ] **Step 6: 编译 + 既有测试回归**

```sh
go build ./internal/... && go vet ./internal/...
go test ./test/unit/... -count=1
```

Expected: build 通过（`internal/web/static.go` 的 dist 错误为已知环境问题可忽略）；unit 测试绿。若 `test/` 下有引用 `UpdateModelHandler` 旧签名的测试，一并修复（改为断言返回的 result）。

- [ ] **Step 7: Commit**

```sh
git add internal/dto/model.go internal/application/model/port/handler.go internal/application/model/command/update_model.go internal/handler/model.go
git commit -m "feat(model): 更新模型支持 syncHistory 一键同步历史 modelId"
```

---

### Task 3: E2E 集成测试（内嵌服务器 + sqlite）

**Files:**
- Test: `test/e2e/model_id/sync_history_test.go`（新建，包名 `model_id`，与现有 `model_id_test.go` 同包但独立 fixture）

**Interfaces:**
- Consumes: Task 1/2 的完整链路（真实路由装配）。
- Produces: 回归守护用例，后续部署验证复用。

- [ ] **Step 1: 通读模板**

完整阅读 `test/e2e/cross_tenant_reference/cross_tenant_test.go`：fixture 装配方式（`RegisterAPIRouter` + JWT 中间件 + sqlite + miniredis）、用户/endpoint/model 种子、HTTP 请求 helper。

- [ ] **Step 2: 写 e2e 测试**

`test/e2e/model_id/sync_history_test.go`。fixture 复刻 cross_tenant 模式（fiber app + humafiber + sqlite + miniredis + 真实 router；userA/userB/admin 三用户，userA/userB 各持 api key）。种子直接用 gorm db 插 `dbmodel`：

```go
// 种子（示意，实际以 fixture 里真实创建的 key/model 为准）：
//   userA: key "key-a"（userID=A）、audit{api_key_id=keyA.ID, model_id="old-id"}、
//          session{api_key_name="key-a", message_ids=[msg.ID], model_ids=["old-id"]}、
//          message{model_id="old-id", check_sum="m1"}
//   userB: key "key-b"、audit{api_key_id=keyB.ID, model_id="old-id"}（隔离对照）
```

三个用例：

1. **`TestSyncHistory_ReplacesAndIsolates`**：userA 的模型（modelId=old-id）PATCH `{"modelId":"new-id","syncHistory":true}` → 断言响应 `data.auditCount==1 && data.sessionCount==1 && data.messageCount==1`；查库断言 A 的 audit/session/message 全变 `new-id`，**B 的 audit 仍为 `old-id`**。
2. **`TestSyncHistory_NoChangeReturnsZero`**：modelId 不变 + `syncHistory:true` → 三计数为 0，历史数据无变化。
3. **`TestSyncHistory_UncheckedKeepsOld`**：PATCH 只改 modelId 不带 syncHistory → 历史数据保持 `old-id`，响应计数 0。

PATCH 请求体（huma 两段式）：

```json
{"body": {"modelId": "new-id", "syncHistory": true}}
```

断言响应 JSON 路径：`{"data": {"auditCount": 1, "sessionCount": 1, "messageCount": 1}}`。

- [ ] **Step 3: 跑测试确认通过**

```sh
go test ./test/e2e/model_id/ -count=1 -v
```

Expected: 新增 3 个用例 PASS，既有 `model_id_test.go` 用例不回归（其 `mustE2EEnv` 缺环境变量会 Skip，属预期）。若断言失败，按失败点修 repo/command 实现（回到 Task 1/2 对应步骤），禁止改断言迁就实现。

- [ ] **Step 4: Commit**

```sh
git add test/e2e/model_id/sync_history_test.go
git commit -m "test(e2e): modelId 历史同步替换、幂等与租户隔离用例"
```

---

### Task 4: 前端类型与 API 层

**Files:**
- Modify: `web/src/lib/types.ts:462`（`UpdateModelReqBody`）
- Modify: `web/src/lib/api-client.ts:618`（`updateModel`）

**Interfaces:**
- Produces（Task 5 依赖）：`api.updateModel(id, body)` 返回 `Promise<ModelUpdateRsp>`。

- [ ] **Step 1: types.ts**

```ts
export interface UpdateModelReqBody {
  alias?: string;
  modelId?: string;
  syncHistory?: boolean; // modelId 变化时同步更新历史记录
  upstreamModel?: string;
  endpointID?: number;
  enabled?: boolean;
  contextLength?: number;
  maxOutputTokens?: number;
  capabilities?: ModelCapability[];
}

// ─── Model update（PATCH /api/web/v1/model） ───────────────────
/** 更新模型响应：历史同步影响行数（未同步时全 0） */
export interface ModelUpdateRsp {
  auditCount: number;
  sessionCount: number;
  messageCount: number;
}
```

- [ ] **Step 2: api-client.ts**

```ts
  async updateModel(id: number, body: UpdateModelReqBody): Promise<ModelUpdateRsp> {
    return this.request<ModelUpdateRsp>(`${API_PREFIX}/model?id=${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  }
```

并在文件顶部 import 处补 `ModelUpdateRsp`（与现有 dto 类型同一 import 语句）。

- [ ] **Step 3: 类型检查**

```sh
cd web && npx tsc --noEmit
```

Expected: 无错误（`updateModel` 旧调用方 `await api.updateModel(...)` 不消费返回值，兼容）。

- [ ] **Step 4: Commit**

```sh
git add web/src/lib/types.ts web/src/lib/api-client.ts
git commit -m "feat(web): updateModel 返回历史同步影响行数类型"
```

---

### Task 5: 前端 UI（checkbox + toast）与 i18n

**Files:**
- Modify: `web/src/app/(dashboard)/upstream/model-dialog.tsx`
- Modify: `web/src/app/(dashboard)/upstream/page.tsx`
- Modify: `web/src/locales/zh.json`、`web/src/locales/en.json`、`web/src/locales/ja.json`

**Interfaces:**
- Consumes: Task 4 的 `api.updateModel` 返回 `ModelUpdateRsp`。

- [ ] **Step 1: page.tsx 状态**

```tsx
// 83 行附近，modelIdTouched 旁新增：
const [originalModelId, setOriginalModelId] = useState("");
const [syncHistory, setSyncHistory] = useState(false);

// 编辑时同步是否可选：仅编辑模式且 modelId 相对打开时原值变化
const showSyncHistory = editingModel !== null && modelForm.modelId !== originalModelId;

// openCreateModel 里追加：
setOriginalModelId("");
setSyncHistory(false);

// openEditModel 里 setEditingModel({ id: model.id }) 之后追加：
setOriginalModelId(model.modelId ?? "");
setSyncHistory(false);
```

- [ ] **Step 2: handleSaveModel 更新分支**

```tsx
      if (editingModel) {
        const rsp = await api.updateModel(editingModel.id, {
          alias: modelForm.alias,
          ...(modelForm.modelId.trim() ? { modelId: modelForm.modelId.trim() } : {}),
          ...(showSyncHistory && syncHistory ? { syncHistory: true } : {}),
          upstreamModel: modelForm.upstreamModel,
          contextLength: modelForm.contextLength,
          maxOutputTokens: modelForm.maxOutputTokens,
          capabilities,
        });
        if (showSyncHistory && syncHistory) {
          toast.success(
            `${t("models.history_synced")}: ${rsp.auditCount} / ${rsp.sessionCount} / ${rsp.messageCount}`,
          );
        } else {
          toast.success(t("models.updated_success"));
        }
      } else {
        // 创建分支不变
      }
```

- [ ] **Step 3: ModelDialog 渲染调用（page.tsx ~645 行）补 props**

```tsx
            showSyncHistory={showSyncHistory}
            syncHistory={syncHistory}
            onSyncHistoryChange={setSyncHistory}
```

- [ ] **Step 4: model-dialog.tsx checkbox**

`ModelDialogProps` 追加：

```tsx
  showSyncHistory: boolean;
  syncHistory: boolean;
  onSyncHistoryChange: (v: boolean) => void;
```

组件函数解构追加上述三个 props。在 Model ID 输入框所在表单区块之后（仅当 `showSyncHistory` 为 true 时渲染）、复用项目现有 Checkbox 组件（先 `rg -n "Checkbox" web/src/components/ui/` 确认组件名与用法；若无 Checkbox 组件则用 `Switch`）：

```tsx
          {showSyncHistory && (
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={syncHistory}
                onCheckedChange={(v) => onSyncHistoryChange(v === true)}
              />
              <span>{t("models.sync_history")}</span>
            </label>
          )}
```

- [ ] **Step 5: i18n 三语种各加 3 个 key**

`zh.json`：
```json
"models.sync_history": "同步更新历史记录",
"models.sync_history_desc": "将历史审计、会话与消息中的旧模型 ID 替换为新值",
"models.history_synced": "历史同步完成（审计/会话/消息）"
```
`en.json`：
```json
"models.sync_history": "Sync historical records",
"models.sync_history_desc": "Replace the old model ID in historical audits, sessions and messages",
"models.history_synced": "History synced (audit/session/message)"
```
`ja.json`：
```json
"models.sync_history": "履歴レコードを同期更新",
"models.sync_history_desc": "過去の監査・セッション・メッセージ内の旧モデルIDを新値に置き換えます",
"models.history_synced": "履歴同期完了（監査/セッション/メッセージ）"
```

（若 `models.*` key 集中在嵌套对象而非平铺 key，按文件内现有 `models.` 相邻 key 的实际结构放置。）

- [ ] **Step 6: 前端三件套**

```sh
cd web && npx tsc --noEmit && npm run lint && npm run build
```

Expected: 全部通过（Turbopack 字体缓存报错时先 `rm -rf web/.next`；worktree 无 node_modules 时 `ln -s` 主工作区 `web/node_modules`）。

- [ ] **Step 7: 运行时验证（next-dev-loop）**

按 `.agents/skills/external/next-dev-loop/SKILL.md` 启动 `next dev`，验证：编辑模型改 modelId → checkbox 出现；勾选保存 → toast 展示计数；不改 modelId → checkbox 不出现。

- [ ] **Step 8: Commit**

```sh
git add web/src/app/\(dashboard\)/upstream/model-dialog.tsx web/src/app/\(dashboard\)/upstream/page.tsx web/src/locales/zh.json web/src/locales/en.json web/src/locales/ja.json
git commit -m "feat(web): 模型编辑弹窗支持一键同步历史模型ID"
```

---

### Task 6: 全量验证与过度工程审查

**Files:** 无新增（验证 + 审查）

- [ ] **Step 1: 全量后端验证**

```sh
go test -count=1 ./cmd/... ./internal/... ./test/...
make lint
```

Expected: 全绿；lint 0 errors（`internal/web/static.go` dist 编译错误为已知环境问题，不阻塞）。

- [ ] **Step 2: ponytail-review 审查 diff**

加载 `.agents/skills/external/ponytail-review/SKILL.md`，对 `git diff master...HEAD` 审查：投机抽象、重复造轮子、死代码。发现项逐条处理或确认保留理由。

- [ ] **Step 3: 沉淀 Serena 工程经验**

`serena_write_memory`：记录本任务可复用决策——「e2e 基建为 sqlite，跨方言 SQL 不可测 → 历史数据批量替换用纯 Go 单事务实现；message 无归属列，scope 经命中会话的 message_ids 收紧；UpdateModelHandler 签名变更为返回 UpdateModelResult」。

- [ ] **Step 4: 汇报**

向用户汇总：改动文件清单、验证证据（测试输出摘要）、剩余未验证项（生产部署后需跑 `test/e2e/model_id/`）。
