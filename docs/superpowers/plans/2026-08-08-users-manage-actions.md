# Users 管理页三种操作（升级/降级/删除）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 web 端 users 管理页为管理员提供三种用户操作：pending→user 升级（已有）、user→pending 降级、删除用户（软删除 + 级联撤销 API Keys），并做好移动端适配。

**Architecture:** 后端沿用现有 DDD 分层（dto → port → command → repository）与 huma 路由模式：新增 `demote`（POST）与 `delete`（DELETE）两个 admin-only 端点，复用现有 `approve` 端点不变；删除通过 `UserRepository.DeleteCascade` 事务内软删用户并批量软删其 API Keys。前端将 users 页行操作从单一「通过」按钮改为统一 DropdownMenu（⋮ 更多操作），桌面表格与移动端卡片复用同一 `UserRowActions` 子组件，删除/降级走 `DeleteConfirmDialog` 二次确认。

**Tech Stack:** Go（huma v2 + fx + gorm + samber/lo）、Next.js 16（App Router + Tailwind v4 + shadcn/ui base-nova + lucide-react）、三语言 i18n（en/zh/ja）。

## Global Constraints

- 所有 HTTP 调用走 `web/src/lib/api-client.ts` 的 `api.*`，禁止业务组件直接 fetch。
- 路由别名：`@/components`、`@/lib`、`@/hooks`，不写相对路径回溯。
- 图标统一 `lucide-react`；Toast 用 `sonner`；禁止 `alert/confirm`。
- 样式仅 Tailwind v4 + `cn()`，颜色走 CSS 变量，禁止硬编码 hex。
- 后端代码需加载 `golang-samber-lo`、`golang-samber-mo` skill 合规；本项目后端无单测文件，验证以 `go build`/`go vet` + E2E 为准。
- i18n 布局稳定性契约：操作触发按钮为图标按钮（`size="icon"`），无文字宽度预留问题；菜单项为弹性元素不强制预留。
- 前端改动后必须 `cd web && npm run lint && npm run build` 通过。
- 开发在 worktree `feature/users-manage-actions-2026-08-08` 上进行；每任务结束提交一次。

---

### Task 1: 后端 DTO + Port 接口定义

**Files:**
- Modify: `internal/dto/user.go`（追加 `DemoteUserReq` / `DeleteUserReq`）
- Modify: `internal/application/identity/port/handler.go`（追加 Demote/Delete 命令与处理器接口）

**Interfaces:**
- Produces: `dto.DemoteUserReq` / `dto.DeleteUserReq`（均 `ID uint` + `query:"id"`）；`port.DemoteUserCommand{OperatorID, UserID uint}` / `port.DemoteUserHandler.Handle(ctx, cmd) error`；`port.DeleteUserCommand{OperatorID, UserID uint}` / `port.DeleteUserHandler.Handle(ctx, cmd) error`

- [ ] **Step 1: 在 `internal/dto/user.go` 文件末尾追加两个请求 DTO**

在 `ApproveUserReq` 定义之后追加：

```go
// DemoteUserReq 降级用户请求（user → pending，admin）
type DemoteUserReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"User ID"`
}

// DeleteUserReq 删除用户请求（admin）
type DeleteUserReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"User ID"`
}
```

- [ ] **Step 2: 在 `internal/application/identity/port/handler.go` 的 `ApproveUserHandler` 接口之后追加命令与处理器**

```go
// DemoteUserCommand 降级用户命令
type DemoteUserCommand struct {
	OperatorID uint // 操作者
	UserID     uint // 目标用户
}

// DemoteUserHandler 降级用户命令处理器
type DemoteUserHandler interface {
	Handle(ctx context.Context, cmd DemoteUserCommand) error
}

// DeleteUserCommand 删除用户命令
type DeleteUserCommand struct {
	OperatorID uint // 操作者
	UserID     uint // 目标用户
}

// DeleteUserHandler 删除用户命令处理器
type DeleteUserHandler interface {
	Handle(ctx context.Context, cmd DeleteUserCommand) error
}
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08 && go build ./...`
Expected: 编译通过，无输出（纯新增类型，无引用改动）。

- [ ] **Step 4: Commit**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08
git add internal/dto/user.go internal/application/identity/port/handler.go
git commit -m "feat(user): 新增 demote/delete 请求 DTO 与 port 接口"
```

---

### Task 2: Repository 新增 DeleteCascade（软删用户 + 级联撤销 API Keys）

**Files:**
- Modify: `internal/domain/identity/repository.go`（接口追加 `DeleteCascade`）
- Modify: `internal/infrastructure/repository/user_repository.go`（实现 `DeleteCascade`）

**Interfaces:**
- Consumes: `dao.GetProxyAPIKeyDAO()`、`constant.FieldUserID`、`r.dao.Delete`（`baseDAO` 软删）
- Produces: `UserRepository.DeleteCascade(ctx context.Context, id uint) error`

- [ ] **Step 1: `internal/domain/identity/repository.go` 的 `UserRepository` 接口末尾追加方法**

```go
	// DeleteCascade 软删除用户及其全部 API Keys（事务保护）
	DeleteCascade(ctx context.Context, id uint) error
```

- [ ] **Step 2: `internal/infrastructure/repository/user_repository.go` 文件末尾实现方法**

仿照 `endpointRepository.DeleteCascade`（`internal/infrastructure/repository/endpoint_repository.go:140`）事务模式：

```go
// DeleteCascade 软删除用户及其全部 API Keys（事务保护）
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param id uint
//	@return error
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func (r *userRepository) DeleteCascade(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := dao.GetProxyAPIKeyDAO().BatchDeleteByField(tx, constant.FieldUserID, []uint{id}); err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "cascade delete api keys by user id")
		}
		if err := r.dao.Delete(tx, &dbmodel.User{ID: id}); err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "delete user")
		}
		return nil
	})
}
```

> 所需 import（`dao`、`constant`、`ierr`、`dbmodel`、`gorm`）均已在文件头存在，无需新增。

- [ ] **Step 3: 编译验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08 && go build ./...`
Expected: 编译通过。若有 import 缺失按报错补充。

- [ ] **Step 4: Commit**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08
git add internal/domain/identity/repository.go internal/infrastructure/repository/user_repository.go
git commit -m "feat(user): UserRepository 新增 DeleteCascade 事务级联删除"
```

---

### Task 3: 领域命令 handlers（demote + delete）

**Files:**
- Create: `internal/application/identity/command/demote_user.go`
- Create: `internal/application/identity/command/delete_user.go`

**Interfaces:**
- Consumes: `port.DemoteUserCommand` / `port.DeleteUserCommand`、`identity.UserRepository`、`User.ChangePermission`、`User.Permission()`、`repo.DeleteCascade`
- Produces: `NewDemoteUserHandler(repo identity.UserRepository) port.DemoteUserHandler`、`NewDeleteUserHandler(repo identity.UserRepository) port.DeleteUserHandler`

- [ ] **Step 1: 新建 `internal/application/identity/command/demote_user.go`**

规则：不存在 → `ErrDataNotExists`；自己 → `ErrValidation`；目标权限非 `user`（覆盖 admin/pending）→ `ErrValidation`；通过则 `ChangePermission(PermissionPending)` + `Save`。

```go
// Package command Identity 域命令处理器
package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type demoteUserHandler struct {
	repo identity.UserRepository
}

// NewDemoteUserHandler 构造
//
//	@param repo identity.UserRepository
//	@return DemoteUserHandler
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func NewDemoteUserHandler(repo identity.UserRepository) port.DemoteUserHandler {
	return &demoteUserHandler{repo: repo}
}

// Handle 执行用户降级：仅允许 user → pending
//
// 规则：
//   - 用户不存在 → ErrDataNotExists
//   - 操作者即目标（禁止自降级）→ ErrValidation
//   - 目标权限非 user（admin/pending 均拒绝）→ ErrValidation
//   - 变更通过领域方法 ChangePermission + Save 持久化
//
// @receiver h *demoteUserHandler
// @param ctx context.Context
// @param cmd DemoteUserCommand
// @return error
// @author centonhuang
// @update 2026-08-08 10:00:00
func (h *demoteUserHandler) Handle(ctx context.Context, cmd port.DemoteUserCommand) error {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		log.Error("[IdentityCommand] FindByID failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	if user == nil {
		log.Warn("[IdentityCommand] Target user not found for demote", zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	if cmd.OperatorID == cmd.UserID {
		log.Warn("[IdentityCommand] Demote rejected, cannot demote self",
			zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrValidation, "cannot demote self")
	}
	if user.Permission() != enum.PermissionUser {
		log.Warn("[IdentityCommand] Demote rejected, target not user",
			zap.Uint("targetID", cmd.UserID), zap.String("permission", string(user.Permission())))
		return ierr.Newf(ierr.ErrValidation, "user %d is not regular user", cmd.UserID)
	}

	user.ChangePermission(enum.PermissionPending)
	if err := h.repo.Save(ctx, user); err != nil {
		log.Error("[IdentityCommand] Save user failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	log.Info("[IdentityCommand] Demote user",
		zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
	return nil
}
```

- [ ] **Step 2: 新建 `internal/application/identity/command/delete_user.go`**

规则：不存在 → `ErrDataNotExists`；自己 → `ErrValidation`；admin → `ErrValidation`；通过则 `repo.DeleteCascade`。

```go
// Package command Identity 域命令处理器
package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type deleteUserHandler struct {
	repo identity.UserRepository
}

// NewDeleteUserHandler 构造
//
//	@param repo identity.UserRepository
//	@return DeleteUserHandler
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func NewDeleteUserHandler(repo identity.UserRepository) port.DeleteUserHandler {
	return &deleteUserHandler{repo: repo}
}

// Handle 执行用户删除（软删除 + 级联撤销 API Keys）
//
// 规则：
//   - 用户不存在 → ErrDataNotExists
//   - 操作者即目标（禁止自删）→ ErrValidation
//   - 目标为 admin → ErrValidation（admin 只读）
//   - 通过校验 → DeleteCascade（事务内软删用户 + 批量软删 API Keys）
//
// @receiver h *deleteUserHandler
// @param ctx context.Context
// @param cmd DeleteUserCommand
// @return error
// @author centonhuang
// @update 2026-08-08 10:00:00
func (h *deleteUserHandler) Handle(ctx context.Context, cmd port.DeleteUserCommand) error {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		log.Error("[IdentityCommand] FindByID failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	if user == nil {
		log.Warn("[IdentityCommand] Target user not found for delete", zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	if cmd.OperatorID == cmd.UserID {
		log.Warn("[IdentityCommand] Delete rejected, cannot delete self", zap.Uint("operatorID", cmd.OperatorID))
		return ierr.New(ierr.ErrValidation, "cannot delete self")
	}
	if user.Permission() == enum.PermissionAdmin {
		log.Warn("[IdentityCommand] Delete rejected, target is admin",
			zap.Uint("targetID", cmd.UserID), zap.String("permission", string(user.Permission())))
		return ierr.Newf(ierr.ErrValidation, "cannot delete admin user %d", cmd.UserID)
	}

	if err := h.repo.DeleteCascade(ctx, cmd.UserID); err != nil {
		log.Error("[IdentityCommand] DeleteCascade failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	log.Info("[IdentityCommand] Delete user",
		zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
	return nil
}
```

- [ ] **Step 3: 编译验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08 && go build ./...`
Expected: 编译通过。

- [ ] **Step 4: Commit**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08
git add internal/application/identity/command/demote_user.go internal/application/identity/command/delete_user.go
git commit -m "feat(user): 新增 demote/delete 领域命令"
```

---

### Task 4: Handler + Router + DI 装配

**Files:**
- Modify: `internal/handler/user.go`
- Modify: `internal/router/user.go`
- Modify: `internal/bootstrap/modules/application.go`
- Modify: `internal/bootstrap/modules/handler.go`

**Interfaces:**
- Consumes: `port.DemoteUserHandler` / `port.DeleteUserHandler`、`dto.DemoteUserReq` / `dto.DeleteUserReq`、`util.CtxValueUint`
- Produces: `UserHandler.HandleDemoteUser(ctx, *dto.DemoteUserReq)`、`UserHandler.HandleDeleteUser(ctx, *dto.DeleteUserReq)`；路由 `POST /api/v1/user/demote`、`DELETE /api/v1/user/delete`；fx 注入 `NewDemoteUserHandler` / `NewDeleteUserHandler`

- [ ] **Step 1: `internal/handler/user.go` —— 接口与依赖扩展**

`UserHandler` 接口追加两个方法：

```go
	HandleDemoteUser(ctx context.Context, req *dto.DemoteUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteUser(ctx context.Context, req *dto.DeleteUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
```

`UserDependencies` 结构体追加两个字段：

```go
	DemoteUser    port.DemoteUserHandler
	DeleteUser    port.DeleteUserHandler
```

`userHandler` struct 追加两个字段：

```go
	demoteUser    port.DemoteUserHandler
	deleteUser    port.DeleteUserHandler
```

`NewUserHandler` 中追加赋值：

```go
		demoteUser:    deps.DemoteUser,
		deleteUser:    deps.DeleteUser,
```

在 `HandleApproveUser` 之后追加两个 handler：

```go
// HandleDemoteUser 降级用户：user → pending（admin）
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.DemoteUserReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func (h *userHandler) HandleDemoteUser(ctx context.Context, req *dto.DemoteUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	operatorID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if err := h.demoteUser.Handle(ctx, port.DemoteUserCommand{
		OperatorID: operatorID,
		UserID:     req.ID,
	}); err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Demote user failed", zap.Error(err), zap.Uint("targetID", req.ID))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, nil)
}

// HandleDeleteUser 删除用户（admin）
//
//	@receiver h *userHandler
//	@param ctx context.Context
//	@param req *dto.DeleteUserReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func (h *userHandler) HandleDeleteUser(ctx context.Context, req *dto.DeleteUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	operatorID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if err := h.deleteUser.Handle(ctx, port.DeleteUserCommand{
		OperatorID: operatorID,
		UserID:     req.ID,
	}); err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Delete user failed", zap.Error(err), zap.Uint("targetID", req.ID))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, nil)
}
```

- [ ] **Step 2: `internal/router/user.go` —— 注册两个路由**

在 `approveUser` 的 `huma.Register` 之后追加：

```go
	huma.Register(userGroup, huma.Operation{
		OperationID: "demoteUser",
		Method:      http.MethodPost,
		Path:        "/demote",
		Summary:     "DemoteUser",
		Description: "Demote a regular user back to pending (admin only)",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("demoteUser", enum.PermissionAdmin),
		},
	}, userHandler.HandleDemoteUser)

	huma.Register(userGroup, huma.Operation{
		OperationID: "deleteUser",
		Method:      http.MethodDelete,
		Path:        "/delete",
		Summary:     "DeleteUser",
		Description: "Soft-delete a user and cascade revoke their API keys (admin only)",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteUser", enum.PermissionAdmin),
		},
	}, userHandler.HandleDeleteUser)
```

- [ ] **Step 3: `internal/bootstrap/modules/application.go` —— fx 注册**

在 `fx.Provide(` 列表中 `NewApproveUserHandler,` 之后追加：

```go
		NewDemoteUserHandler,
		NewDeleteUserHandler,
```

在文件末尾的 `NewApproveUserHandler` 包装函数之后追加两个包装函数（模式照抄）：

```go
func NewDemoteUserHandler(repo identity.UserRepository) identityport.DemoteUserHandler {
	return identitycommand.NewDemoteUserHandler(repo)
}

func NewDeleteUserHandler(repo identity.UserRepository) identityport.DeleteUserHandler {
	return identitycommand.NewDeleteUserHandler(repo)
}
```

- [ ] **Step 4: `internal/bootstrap/modules/handler.go` —— 依赖装配**

`NewUserDependencies` 函数签名与返回值扩展：

```go
func NewUserDependencies(getCurrentUser identityport.GetCurrentUserHandler, updateProfile identityport.UpdateProfileHandler,
	listUsers identityport.ListUsersHandler, approveUser identityport.ApproveUserHandler,
	demoteUser identityport.DemoteUserHandler, deleteUser identityport.DeleteUserHandler) handler.UserDependencies {
	return handler.UserDependencies{
		GetCurrentUser: getCurrentUser,
		UpdateProfile:  updateProfile,
		ListUsers:      listUsers,
		ApproveUser:    approveUser,
		DemoteUser:     demoteUser,
		DeleteUser:     deleteUser,
	}
}
```

- [ ] **Step 5: 编译验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08 && go build ./... && go vet ./...`
Expected: 全部通过，无输出。

- [ ] **Step 6: Commit**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08
git add internal/handler/user.go internal/router/user.go internal/bootstrap/modules/application.go internal/bootstrap/modules/handler.go
git commit -m "feat(user): 注册 demote/delete 路由与 DI 装配"
```

---

### Task 5: 前端 types + api-client

**Files:**
- Modify: `web/src/lib/types.ts`（追加 `DemoteUserReq` / `DeleteUserReq`）
- Modify: `web/src/lib/api-client.ts`（追加 `demoteUser` / `deleteUser` 方法）

**Interfaces:**
- Consumes: `api.request` 封装、`CommonRsp`
- Produces: `api.demoteUser(id: number): Promise<CommonRsp>`、`api.deleteUser(id: number): Promise<CommonRsp>`

- [ ] **Step 1: `web/src/lib/types.ts` 的 User Management 区块（`ListUsersRsp` 之后）追加**

```ts
export interface DemoteUserReq {
  id: number;
}

export interface DeleteUserReq {
  id: number;
}
```

- [ ] **Step 2: `web/src/lib/api-client.ts` 的 `approveUser` 方法之后追加两个方法**

```ts
  async demoteUser(id: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`/api/v1/user/demote?id=${id}`, { method: "POST" });
  }
  async deleteUser(id: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`/api/v1/user/delete?id=${id}`, { method: "DELETE" });
  }
```

> `CommonRsp` 与 `approveUser` 已在文件内定义/使用，无需新增 import。

- [ ] **Step 3: 类型与构建验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npx tsc --noEmit`
Expected: 无类型错误（仅新增方法，无引用改动）。

- [ ] **Step 4: Commit**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08
git add web/src/lib/types.ts web/src/lib/api-client.ts
git commit -m "feat(web): api-client 新增 demoteUser/deleteUser"
```

---

### Task 6: i18n 三语言 keys

**Files:**
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/zh.json`
- Modify: `web/src/locales/ja.json`

**Interfaces:**
- Produces: `users.actions_aria` / `users.promote` / `users.demote` / `users.delete_menu` / `users.demote_confirm_title` / `users.demote_confirm_desc` / `users.delete_confirm_title` / `users.delete_confirm_desc` / `users.demote_success` / `users.demote_error` / `users.delete_success` / `users.delete_error`

- [ ] **Step 1: 三个语言文件在 `"users.load_error"` 之后追加同一组 keys（值按语言翻译）**

`zh.json` 追加：

```json
  "users.actions_aria": "更多操作",
  "users.promote": "升为 User",
  "users.demote": "降级为 Pending",
  "users.delete_menu": "删除用户",
  "users.demote_confirm_title": "降级用户",
  "users.demote_confirm_desc": "确定要将该用户降级为待审用户吗？降级后用户将无法使用平台功能，需重新审核才能恢复。",
  "users.delete_confirm_title": "删除用户",
  "users.delete_confirm_desc": "确定要删除该用户吗？该用户的全部 API 密钥将被一并撤销，用户重新登录后将进入待审状态。",
  "users.demote_success": "用户已降级为待审",
  "users.demote_error": "降级失败",
  "users.delete_success": "用户已删除",
  "users.delete_error": "删除失败"
```

`en.json` 追加：

```json
  "users.actions_aria": "More actions",
  "users.promote": "Promote to User",
  "users.demote": "Demote to Pending",
  "users.delete_menu": "Delete user",
  "users.demote_confirm_title": "Demote user",
  "users.demote_confirm_desc": "Demote this user to pending? They will lose access to platform features and need approval to regain them.",
  "users.delete_confirm_title": "Delete user",
  "users.delete_confirm_desc": "Delete this user? All of their API keys will be revoked, and they will enter pending status again after re-login.",
  "users.demote_success": "User demoted to pending",
  "users.demote_error": "Failed to demote user",
  "users.delete_success": "User deleted",
  "users.delete_error": "Failed to delete user"
```

`ja.json` 追加：

```json
  "users.actions_aria": "その他の操作",
  "users.promote": "User に昇格",
  "users.demote": "Pending に降格",
  "users.delete_menu": "ユーザーを削除",
  "users.demote_confirm_title": "ユーザーを降格",
  "users.demote_confirm_desc": "このユーザーを保留（Pending）に降格しますか？降格後はプラットフォーム機能を利用できず、再審査が必要になります。",
  "users.delete_confirm_title": "ユーザーを削除",
  "users.delete_confirm_desc": "このユーザーを削除しますか？すべての API キーが無効化され、再ログイン後は保留状態になります。",
  "users.demote_success": "ユーザーを保留に降格しました",
  "users.demote_error": "降格に失敗しました",
  "users.delete_success": "ユーザーを削除しました",
  "users.delete_error": "削除に失敗しました"
```

- [ ] **Step 2: JSON 合法性验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && python3 -c "import json; [json.load(open(f'src/locales/{l}.json')) for l in ('en','zh','ja')]; print('ok')"`
Expected: 输出 `ok`。

- [ ] **Step 3: Commit**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08
git add web/src/locales/en.json web/src/locales/zh.json web/src/locales/ja.json
git commit -m "feat(web): users 操作 i18n 三语言 keys"
```

---

### Task 7: users 页面 DropdownMenu 重构（桌面 + 移动端）

**Files:**
- Modify: `web/src/app/(dashboard)/users/page.tsx`

**Interfaces:**
- Consumes: `api.approveUser` / `api.demoteUser` / `api.deleteUser`、`useAuth().user`（当前用户，含 `id`）、`DeleteConfirmDialog`、`@/components/ui/dropdown-menu`、`MoreHorizontal`（lucide-react）、`UserItem`、`permission.pending|user|admin` i18n keys
- Produces: 页面级 `UserRowActions` 子组件（同文件内定义）

- [ ] **Step 1: 重写 `web/src/app/(dashboard)/users/page.tsx`**

完整替换文件内容为：

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { MoreHorizontal } from "lucide-react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useAuth } from "@/lib/auth-context";
import { PermissionGuard } from "@/components/permission-guard";
import type { PageInfo, UserItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PageHeader } from "@/components/page-header";
import { SearchInput } from "@/components/search-input";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { PaginationBar } from "@/components/pagination-bar";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { Users } from "lucide-react";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";

const PERMISSIONS = ["pending", "user", "admin"] as const;

type UserAction = "promote" | "demote" | "delete";

interface UserRowActionsProps {
  user: UserItem;
  currentUserId: number | null;
  acting: UserAction | null;
  onAction: (action: UserAction, user: UserItem) => void;
}

/** 行操作菜单：pending→升为 User；user→降级为 Pending；均可删除；admin 与自己只读 */
function UserRowActions({ user, currentUserId, acting, onAction }: UserRowActionsProps) {
  const t = useT();
  const canOperate = user.permission !== "admin" && user.id !== currentUserId;
  if (!canOperate) {
    return null;
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="icon" aria-label={t("users.actions_aria")} />}>
        <MoreHorizontal className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-36 p-1">
        {user.permission === "pending" && (
          <DropdownMenuItem onSelect={() => onAction("promote", user)}>
            {t("users.promote")}
          </DropdownMenuItem>
        )}
        {user.permission === "user" && (
          <DropdownMenuItem onSelect={() => onAction("demote", user)}>
            {t("users.demote")}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem variant="destructive" onSelect={() => onAction("delete", user)}>
          {t("users.delete_menu")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export default function UsersPage() {
  const { user: currentUser } = useAuth();
  const [items, setItems] = useState<UserItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.users.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.users.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [permission, setPermission] = useState("");
  const [acting, setActing] = useState<UserAction | null>(null);
  const [confirmAction, setConfirmAction] = useState<UserAction | null>(null);
  const [confirmUser, setConfirmUser] = useState<UserItem | null>(null);
  const t = useT();
  const isMobile = useIsMobile();

  const fetchUsers = useCallback(async (page: number, pageSize: number, query?: string, perm?: string) => {
    setLoading(true);
    try {
      const rsp = await api.listUsers(page, pageSize, {
        query: query || undefined,
        permission: perm || undefined,
      });
      setItems(rsp.items ?? []);
      if (rsp.pageInfo) {
        setPageInfo(rsp.pageInfo);
        setPersistedPage(rsp.pageInfo.page);
        setPersistedPageSize(rsp.pageInfo.pageSize);
      }
    } catch (err) {
      showErrorToast(err, { title: t("users.load_error") });
    } finally {
      setLoading(false);
    }
  }, [setPersistedPage, setPersistedPageSize, t]);

  /* eslint-disable react-hooks/set-state-in-effect -- Re-fetch list when the persisted page or size changes */
  useEffect(() => {
    fetchUsers(persistedPage, persistedPageSize, searchQuery || undefined, permission || undefined);
  }, [fetchUsers, persistedPage, persistedPageSize, searchQuery, permission]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleSearch = useCallback(() => {
    setPersistedPage(1);
    fetchUsers(1, persistedPageSize, searchQuery || undefined, permission || undefined);
  }, [fetchUsers, persistedPageSize, permission, searchQuery, setPersistedPage]);

  const runAction = useCallback(async (action: UserAction, user: UserItem) => {
    setActing(action);
    try {
      switch (action) {
        case "promote":
          await api.approveUser(user.id);
          toast.success(t("users.approved_success"));
          break;
        case "demote":
          await api.demoteUser(user.id);
          toast.success(t("users.demote_success"));
          break;
        case "delete":
          await api.deleteUser(user.id);
          toast.success(t("users.delete_success"));
          break;
      }
      fetchUsers(pageInfo.page, pageInfo.pageSize, searchQuery || undefined, permission || undefined);
    } catch (err) {
      showErrorToast(err, {
        title: action === "promote" ? t("users.approve_error") : action === "demote" ? t("users.demote_error") : t("users.delete_error"),
      });
    } finally {
      setActing(null);
    }
  }, [fetchUsers, pageInfo.page, pageInfo.pageSize, permission, searchQuery, t]);

  const handleAction = useCallback((action: UserAction, user: UserItem) => {
    if (action === "promote") {
      runAction("promote", user);
      return;
    }
    setConfirmAction(action);
    setConfirmUser(user);
  }, [runAction]);

  const handleConfirm = useCallback(() => {
    if (confirmAction && confirmUser) {
      runAction(confirmAction, confirmUser);
    }
    setConfirmAction(null);
    setConfirmUser(null);
  }, [confirmAction, confirmUser, runAction]);

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <PageHeader title={t("users.title")} description={t("users.subtitle")} />

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("users.list_title")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <Select
                value={permission}
                onValueChange={(v) => {
                  setPermission(v === "all" ? "" : (v as string));
                  setPersistedPage(1);
                }}
              >
                <SelectTrigger className="w-40" aria-label={t("users.permission_filter")}>
                  <SelectValue placeholder={t("users.all_permissions")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("users.all_permissions")}</SelectItem>
                  {PERMISSIONS.map((p) => (
                    <SelectItem key={p} value={p}>{t(`permission.${p}`)}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <SearchInput
                placeholder={t("users.search_placeholder")}
                value={searchQuery}
                onChange={setSearchQuery}
                onSearch={handleSearch}
              />
            </div>
            {loading ? (
              <TableSkeleton />
            ) : items.length === 0 ? (
              <ListEmptyState icon={<Users className="mb-3 size-10 text-muted-foreground/40" />} message={t("users.no_users")} />
            ) : (
              <>
                {isMobile ? (
                  <div className="space-y-3">
                    {items.map((user) => (
                      <div key={user.id} className="rounded-lg border border-border bg-card p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <p className="text-sm font-medium">{user.name}</p>
                            <p className="mt-0.5 text-xs text-muted-foreground">{user.email}</p>
                            <p className="mt-0.5 text-xs text-muted-foreground">
                              {t(`permission.${user.permission}`)}
                            </p>
                          </div>
                          <UserRowActions
                            user={user}
                            currentUserId={currentUser?.id ?? null}
                            acting={acting}
                            onAction={handleAction}
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("users.name")}</TableHead>
                        <TableHead>{t("users.email")}</TableHead>
                        <TableHead>{t("users.permission")}</TableHead>
                        <TableHead>{t("users.created_at")}</TableHead>
                        <TableHead>{t("users.last_login")}</TableHead>
                        <TableHead className="w-14 text-right">{t("common.actions")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {items.map((user) => (
                        <TableRow key={user.id}>
                          <TableCell className="font-medium">{user.name}</TableCell>
                          <TableCell className="text-muted-foreground">{user.email}</TableCell>
                          <TableCell className="text-muted-foreground">{t(`permission.${user.permission}`)}</TableCell>
                          <TableCell className="text-muted-foreground">
                            {user.createdAt ? new Date(user.createdAt).toLocaleDateString() : "—"}
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {user.lastLogin ? new Date(user.lastLogin).toLocaleDateString() : "—"}
                          </TableCell>
                          <TableCell className="text-right">
                            <UserRowActions
                              user={user}
                              currentUserId={currentUser?.id ?? null}
                              acting={acting}
                              onAction={handleAction}
                            />
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
                <PaginationBar
                  pageInfo={pageInfo}
                  onChange={(page, pageSize) => fetchUsers(page, pageSize, searchQuery || undefined, permission || undefined)}
                  totalLabel={t("pagination.items")}
                />
              </>
            )}
          </CardContent>
        </Card>

        <DeleteConfirmDialog
          open={confirmAction !== null}
          onOpenChange={(open) => { if (!open) { setConfirmAction(null); setConfirmUser(null); } }}
          title={confirmAction === "demote" ? t("users.demote_confirm_title") : t("users.delete_confirm_title")}
          description={confirmAction === "demote" ? t("users.demote_confirm_desc") : t("users.delete_confirm_desc")}
          confirmLabel={confirmAction === "demote" ? t("users.demote") : t("common.delete")}
          loadingLabel={t("common.processing")}
          loading={acting !== null}
          onConfirm={handleConfirm}
        />
      </div>
    </PermissionGuard>
  );
}
```

> 说明：
> - `acting` 仅在确认/执行时非空，`DeleteConfirmDialog` 的 `loading` 由它驱动。
> - `DropdownMenuTrigger` 使用 `render` prop（base-ui 风格，参照 `models/page.tsx` 用法）。
> - 原 `approving` 状态已被 `acting` 取代；原 inline approve 按钮逻辑被 `runAction("promote")` 取代。

- [ ] **Step 2: lint 验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run lint`
Expected: 0 errors（如有个别 warning 可接受，不得有 error）。

- [ ] **Step 3: 构建验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run build`
Expected: 静态导出成功（23 个页面左右，无类型错误）。

- [ ] **Step 4: Commit**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08
git add web/src/app/\(dashboard\)/users/page.tsx
git commit -m "feat(web): users 页行操作改为 DropdownMenu（升级/降级/删除）"
```

---

### Task 8: E2E 补充

**Files:**
- Create: `test/e2e/users/user_manage_actions_test.go`

**Interfaces:**
- Consumes: `BASE_URL` / `ADMIN_TOKEN` / `USER_TOKEN` 环境变量、现有 `doJSON` / `mustE2EEnv` helper（同包 `user_review_test.go` 定义）
- Produces: 4 个 E2E 用例

- [ ] **Step 1: 新建 `test/e2e/users/user_manage_actions_test.go`**

```go
// Package users 验证用户管理操作：降级（demote）与删除（delete）的权限与规则闭环。
//
// 环境变量：
//   - BASE_URL     API 根地址（必填）
//   - ADMIN_TOKEN  管理员 JWT（必填）
//   - USER_TOKEN   普通用户 JWT（必填）
package users

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

type currentUserRsp struct {
	User struct {
		ID uint `json:"id"`
	} `json:"user"`
}

func adminSelfID(t *testing.T, baseURL, adminToken string) uint {
	t.Helper()
	client := &http.Client{Timeout: e2eHTTPTimeout}
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/current", adminToken)
	if status != http.StatusOK {
		t.Fatalf("get current user expected 200, got %d: %s", status, body)
	}
	var rsp currentUserRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal current user response failed: %v", err)
	}
	return rsp.User.ID
}

func TestE2E_RegularUserCannotManageUsers(t *testing.T) {
	t.Parallel()
	baseURL, _, userToken := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	for _, tc := range []struct {
		method string
		url    string
	}{
		{http.MethodPost, baseURL + "/api/v1/user/demote?id=1"},
		{http.MethodDelete, baseURL + "/api/v1/user/delete?id=1"},
	} {
		status, body := doJSON(t, client, tc.method, tc.url, userToken)
		if status != http.StatusForbidden && status != http.StatusUnauthorized {
			t.Fatalf("regular user %s %s expected 403/401, got %d: %s", tc.method, tc.url, status, body)
		}
	}
}

func TestE2E_AdminDemoteThenReapprove(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 找第一个 user 权限用户
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=20&permission=user", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list user-permission users expected 200, got %d: %s", status, body)
	}
	var rsp listUsersRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal list response failed: %v", err)
	}
	if len(rsp.Items) == 0 {
		t.Skip("no regular users in environment, demote flow skipped")
	}
	target := rsp.Items[0]

	// 降级 → 200
	status, body = doJSON(t, client, http.MethodPost, fmt.Sprintf("%s/api/v1/user/demote?id=%d", baseURL, target.ID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("demote user expected 200, got %d: %s", status, body)
	}

	// 恢复：批准 → 200（闭环无副作用）
	status, body = doJSON(t, client, http.MethodPost, fmt.Sprintf("%s/api/v1/user/approve?id=%d", baseURL, target.ID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("re-approve after demote expected 200, got %d: %s", status, body)
	}
}

func TestE2E_AdminCannotDeleteSelf(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	adminID := adminSelfID(t, baseURL, adminToken)
	status, body := doJSON(t, client, http.MethodDelete, fmt.Sprintf("%s/api/v1/user/delete?id=%d", baseURL, adminID), adminToken)
	if status == http.StatusOK {
		t.Fatalf("delete self expected non-200, got 200: %s", body)
	}
}

func TestE2E_AdminDeleteNonexistentUser(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	status, body := doJSON(t, client, http.MethodDelete, baseURL+"/api/v1/user/delete?id=99999999", adminToken)
	if status == http.StatusOK {
		t.Fatalf("delete nonexistent user expected non-200, got 200: %s", body)
	}
}
```

- [ ] **Step 2: 编译验证（E2E 包可编译）**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08 && go vet ./test/e2e/users/`
Expected: 通过，无输出。

- [ ] **Step 3: Commit**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08
git add test/e2e/users/user_manage_actions_test.go
git commit -m "test(e2e): users 降级/删除管理操作闭环"
```

---

### Task 9: 全量验证 + 浏览器验证

**Files:** 无（验证任务）

- [ ] **Step 1: 后端全量编译**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08 && go build ./... && go vet ./...`
Expected: 全部通过。

- [ ] **Step 2: 前端 lint + build**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/web && npm run lint && npm run build`
Expected: lint 0 errors；build 静态导出成功。

- [ ] **Step 3: 本地起后端 + 前端联调**

1. 后端：`go run ./cmd/server server start --host localhost --port 8080`（需 PostgreSQL + Redis，`env/api.env`）
2. 前端：`cd web && NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev`
3. 浏览器访问 `http://localhost:3000/web`，进入 Users 页验证：
   - 桌面表格：pending/user 行显示 ⋮ 菜单（含对应操作项）；admin 行与当前登录用户无菜单
   - 移动端（Chrome devtools 模拟视口 390×844）：卡片布局 ⋮ 菜单可用
   - 升级直接执行并 toast 成功；降级/删除二次确认后执行；操作后列表刷新
   - 删除后该用户 API Key 调用返回 401（软删生效）

- [ ] **Step 4: E2E 用例（如有可用环境）**

Run: `BASE_URL=<url> ADMIN_TOKEN=<token> USER_TOKEN=<token> go test ./test/e2e/users/ -v`
Expected: 新增 4 个用例通过（无目标时 Skip 属正常）。

- [ ] **Step 5: 最终提交 + 提交前工程经验沉淀（Serena memory）**

按 AGENTS.md 要求在提交前用 `serena_write_memory` 记录本次实现的可复用经验（用户软删除级联模式、DropdownMenu 行操作模式、admin 保护双重校验），然后确认分支状态干净。

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature-users-manage-actions-2026-08-08
git status --short
```

---

## Self-Review 记录

**1. Spec coverage（对照设计文档）：**
- D1 软删除 + 级联撤销 API Keys → Task 2 `DeleteCascade` ✓
- D2 admin 只读 + 禁操作自己 → Task 3 命令校验 + Task 7 前端 `canOperate` ✓
- D3 DropdownMenu 统一桌面/移动 → Task 7 `UserRowActions` 两处复用 ✓
- D4 `DELETE /api/v1/user/delete` → Task 4 路由 `Method: http.MethodDelete` ✓
- D5 事务原子性 → Task 2 `db.Transaction` ✓
- D6 删除/降级二次确认、升级直行 → Task 7 `handleAction`/`handleConfirm` ✓
- 移动端适配 → Task 7 `isMobile` 卡片分支挂载同一 `UserRowActions` ✓
- E2E 四项 → Task 8 ✓
- i18n 三语言 → Task 6 ✓

**2. Placeholder scan:** 无 TBD/TODO；每步含完整代码。

**3. Type consistency:**
- `port.DemoteUserCommand{OperatorID, UserID}` 在 Task 1/3/4 一致 ✓
- `api.demoteUser(id)` / `api.deleteUser(id)` 在 Task 5/7 一致 ✓
- i18n keys 在 Task 6/7 一致 ✓
- `UserRowActions` props（`user`/`currentUserId`/`acting`/`onAction`）在 Task 7 内部一致 ✓
