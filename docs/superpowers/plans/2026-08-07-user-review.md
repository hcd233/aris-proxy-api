# 用户审核闭环（pending → user）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增管理员用户列表与审核接口，管理后台提供「用户管理」页，pending 用户可一键批准为 user，消除 SSH 改库的运营断点。

**Architecture:** 在既有 identity 领域能力（`User` 聚合 `ChangePermission` + `UserRepository`）之上补齐管理员视角的查询与变更入口，全程复用现有分层：router → handler → application port/usecase → domain aggregate → repository。列表查询复用 `dao.Paginate`（多字段 OR 模糊 + 非零字段精确过滤 + 分页），审核变更复用 `UserRepository.FindByID + ChangePermission + Save`。

**Tech Stack:** Go 1.25 / Fiber v3 / Huma v2 / GORM / uber-fx；Next.js 16 / React 19 / Tailwind；测试：go test + test/e2e。

## Global Constraints

- 本期只允许 `pending → user` 一种权限变更；`approve` 接口对非 pending 目标一律返回业务错误。
- 无路径参数：目标用户 ID 一律走 `query:"id"`（全项目惯例，见 `DeleteAPIKeyReq`）。
- 列表搜索参数名用 `query`（内嵌 `model.CommonParam`，与 apikey/blocked 列表一致；spec 中"keyword"指语义，实现参数名以本项目惯例为准，文档已同步）。
- 接口仅 admin 可访问：`middleware.LimitUserPermissionMiddleware(xxx, enum.PermissionAdmin)`。
- 不建审计表，权限变更用结构化 logger 记录。
- 中文回复与注释、中文文档；代码标识符保留英文。
- 每个 Task 结束必须编译/测试通过并提交。

---

### Task 1: 领域仓储接口与实现（ListUsers）

**Files:**
- Modify: `internal/domain/identity/repository.go`
- Modify: `internal/infrastructure/repository/user_repository.go`

**Interfaces:**
- Consumes: `dao.UserDAO`（已有）、`constant.UserRepoFieldsFull`、`constant.FieldName/FieldEmail`、`dao.CommonParam`、`model.CommonParam`（internal/common/model）
- Produces: `identity.UserRepository.ListUsers(ctx, param model.CommonParam, permission enum.Permission) ([]*aggregate.User, *model.PageInfo, error)` —— 后续 usecase 依赖此签名

- [ ] **Step 1: 在领域仓储接口新增 ListUsers**

`internal/domain/identity/repository.go` 的 `UserRepository` 接口内追加：

```go
	// ListUsers 分页查询用户（管理员视图）。permission 非空时按权限精确过滤；
	// param.Query 非空时对 name/email 做模糊匹配。
	ListUsers(ctx context.Context, param model.CommonParam, permission enum.Permission) ([]*aggregate.User, *model.PageInfo, error)
```

新增 import：

```go
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
```

- [ ] **Step 2: 实现 userRepository.ListUsers**

`internal/infrastructure/repository/user_repository.go` 中 `FindByGoogleBindID` 之后新增：

```go
// ListUsers 分页查询用户（管理员视图）
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param param model.CommonParam 分页/搜索参数
//	@param permission enum.Permission 权限过滤，空串=全部
//	@return []*aggregate.User
//	@return *model.PageInfo
//	@return error
func (r *userRepository) ListUsers(ctx context.Context, param model.CommonParam, permission enum.Permission) ([]*aggregate.User, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.User{Permission: permission},
		constant.UserRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldName, constant.FieldEmail}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate users")
	}
	return lo.Map(records, func(m *dbmodel.User, _ int) *aggregate.User {
		return toUserAggregate(m)
	}), pageInfo, nil
}
```

新增 import：`"github.com/samber/lo"`、`"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"`（确认现有 import 是否已有；`model` 别名指 `internal/common/model`）。

- [ ] **Step 3: 编译验证**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature/user-review-2026-08-07 && go build ./internal/...`
Expected: 编译通过（无输出错误）。

- [ ] **Step 4: Commit**

```bash
git add internal/domain/identity/repository.go internal/infrastructure/repository/user_repository.go
git commit -m "feat(user): 仓储层新增 ListUsers 分页查询"
```

---

### Task 2: 应用层 port 定义

**Files:**
- Modify: `internal/application/identity/port/handler.go`

**Interfaces:**
- Produces: `port.ListUsersQuery`、`port.ListUsersHandler`、`port.ApproveUserCommand`、`port.ApproveUserHandler` —— Task 3/4/5 依赖

- [ ] **Step 1: 追加 port 定义**

`internal/application/identity/port/handler.go` 文件末尾新增：

```go
// ListUsersQuery 用户列表查询
type ListUsersQuery struct {
	model.CommonParam
	Permission enum.Permission
}

// ListUsersHandler 用户列表查询处理器
type ListUsersHandler interface {
	Handle(ctx context.Context, q ListUsersQuery) ([]*UserView, *model.PageInfo, error)
}

// ApproveUserCommand 审核用户命令
type ApproveUserCommand struct {
	OperatorID uint // 操作者
	UserID     uint // 目标用户
}

// ApproveUserHandler 审核用户命令处理器
type ApproveUserHandler interface {
	Handle(ctx context.Context, cmd ApproveUserCommand) error
}
```

新增 import：`"github.com/hcd233/aris-proxy-api/internal/common/model"`。

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/application/identity/...`
Expected: 编译通过。

- [ ] **Step 3: Commit**

```bash
git add internal/application/identity/port/handler.go
git commit -m "feat(user): 应用层 port 定义 ListUsers/ApproveUser"
```

---

### Task 3: ListUsers usecase + 单测（TDD）

**Files:**
- Create: `internal/application/identity/query/list_users.go`
- Create: `test/unit/user_review/fake_repo.go`
- Create: `test/unit/user_review/list_users_test.go`

**Interfaces:**
- Consumes: `port.ListUsersQuery/ListUsersHandler`（Task 2）、`identity.UserRepository.ListUsers`（Task 1）
- Produces: `query.NewListUsersHandler(repo identity.UserRepository) port.ListUsersHandler` —— Task 5 依赖

- [ ] **Step 1: 写失败测试（fake repo + 列表查询测试）**

Create `test/unit/user_review/fake_repo.go`（内存版 `identity.UserRepository`，供本 task 与 Task 4 复用）：

```go
// Package user_review 用户审核闭环的单元测试
package user_review

import (
	"context"
	"sort"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
)

// fakeUserRepo 内存版 UserRepository，支持 Save/FindByID/ListUsers。
type fakeUserRepo struct {
	users []*aggregate.User
	next  uint
}

func newFakeUserRepo(seed ...*aggregate.User) *fakeUserRepo {
	r := &fakeUserRepo{next: 1}
	for _, u := range seed {
		u.SetID(r.next)
		r.next++
		r.users = append(r.users, u)
	}
	return r
}

func (r *fakeUserRepo) Save(ctx context.Context, user *aggregate.User) error {
	if user.AggregateID() == 0 {
		user.SetID(r.next)
		r.next++
		r.users = append(r.users, user)
		return nil
	}
	for i, u := range r.users {
		if u.AggregateID() == user.AggregateID() {
			r.users[i] = user
			return nil
		}
	}
	r.users = append(r.users, user)
	return nil
}

func (r *fakeUserRepo) FindByID(ctx context.Context, id uint) (*aggregate.User, error) {
	for _, u := range r.users {
		if u.AggregateID() == id {
			return u, nil
		}
	}
	return nil, nil
}

func (r *fakeUserRepo) FindByGithubBindID(ctx context.Context, bindID string) (*aggregate.User, error) {
	return nil, nil
}

func (r *fakeUserRepo) FindByGoogleBindID(ctx context.Context, bindID string) (*aggregate.User, error) {
	return nil, nil
}

func (r *fakeUserRepo) TouchLastLogin(ctx context.Context, userID uint) error {
	return nil
}

func (r *fakeUserRepo) ListUsers(ctx context.Context, param model.CommonParam, permission enum.Permission) ([]*aggregate.User, *model.PageInfo, error) {
	matches := make([]*aggregate.User, 0, len(r.users))
	for _, u := range r.users {
		if permission != "" && u.Permission() != permission {
			continue
		}
		if param.Query != "" {
			q := strings.ToLower(param.Query)
			if !strings.Contains(strings.ToLower(u.Name().String()), q) &&
				!strings.Contains(strings.ToLower(u.Email().String()), q) {
				continue
			}
		}
		matches = append(matches, u)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].AggregateID() < matches[j].AggregateID() })
	total := int64(len(matches))
	page, pageSize := param.Page, param.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > len(matches) {
		start = len(matches)
	}
	end := start + pageSize
	if end > len(matches) {
		end = len(matches)
	}
	return matches[start:end], &model.PageInfo{Page: page, PageSize: pageSize, Total: total}, nil
}
```

Create `test/unit/user_review/list_users_test.go`：

```go
package user_review

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/application/identity/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
)

func newUser(t *testing.T, name, email string, perm enum.Permission) *aggregate.User {
	t.Helper()
	u, err := aggregate.RegisterUser(vo.UserName(name), vo.Email(email), vo.Avatar(""), "github", "bind-"+name, time.Now())
	if err != nil {
		t.Fatalf("register user failed: %v", err)
	}
	u.ChangePermission(perm)
	return u
}

func TestListUsers_PaginateAndFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(
		newUser(t, "alice", "alice@example.com", enum.PermissionUser),
		newUser(t, "bob", "bob@example.com", enum.PermissionPending),
		newUser(t, "carol", "carol@example.com", enum.PermissionAdmin),
	)
	handler := query.NewListUsersHandler(repo)

	// 全部（默认分页）
	views, pageInfo, err := handler.Handle(ctx, port.ListUsersQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
	})
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if len(views) != 3 || pageInfo.Total != 3 {
		t.Fatalf("expected 3 users, got %d (total %d)", len(views), pageInfo.Total)
	}

	// 按权限过滤 pending
	views, pageInfo, err = handler.Handle(ctx, port.ListUsersQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
		Permission:  enum.PermissionPending,
	})
	if err != nil {
		t.Fatalf("list pending failed: %v", err)
	}
	if len(views) != 1 || views[0].Name != "bob" || pageInfo.Total != 1 {
		t.Fatalf("expected only bob, got %+v", views)
	}

	// 关键词模糊匹配 email
	views, _, err = handler.Handle(ctx, port.ListUsersQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}, QueryParam: model.QueryParam{Query: "carol"}},
	})
	if err != nil {
		t.Fatalf("list by keyword failed: %v", err)
	}
	if len(views) != 1 || views[0].Name != "carol" {
		t.Fatalf("expected carol by email, got %+v", views)
	}
}
```

> 注：`model.CommonParam` 的字段布局（`PageParam/QueryParam/SortParam` 内嵌）以 `internal/common/model/param.go` 为准；`RegisterUser` 参数类型为 `vo.UserName/vo.Email/vo.Avatar`。

- [ ] **Step 2: 运行确认失败（编译失败即可，函数未实现）**

Run: `go test ./test/unit/user_review/... -run TestListUsers`
Expected: 编译错误（`query.NewListUsersHandler` 未定义）。

- [ ] **Step 3: 实现 usecase**

Create `internal/application/identity/query/list_users.go`：

```go
// Package query Identity 域查询处理器
package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
)

type listUsersHandler struct {
	repo identity.UserRepository
}

// NewListUsersHandler 构造
func NewListUsersHandler(repo identity.UserRepository) port.ListUsersHandler {
	return &listUsersHandler{repo: repo}
}

// Handle 执行用户列表查询（管理员视图）
func (h *listUsersHandler) Handle(ctx context.Context, q port.ListUsersQuery) ([]*port.UserView, *model.PageInfo, error) {
	users, pageInfo, err := h.repo.ListUsers(ctx, q.CommonParam, q.Permission)
	if err != nil {
		return nil, nil, err
	}
	views := lo.Map(users, func(u *aggregate.User, _ int) *port.UserView {
		return &port.UserView{
			ID:         u.AggregateID(),
			Name:       u.Name().String(),
			Email:      u.Email().String(),
			Avatar:     u.Avatar().String(),
			Permission: u.Permission(),
			CreatedAt:  u.CreatedAt(),
			LastLogin:  u.LastLogin(),
		}
	})
	return views, pageInfo, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./test/unit/user_review/... -v`
Expected: `TestListUsers_PaginateAndFilter` PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/application/identity/query/list_users.go test/unit/user_review/
git commit -m "feat(user): ListUsers usecase 与单测"
```

---

### Task 4: ApproveUser usecase + 单测（TDD）

**Files:**
- Create: `internal/application/identity/command/approve_user.go`
- Create: `test/unit/user_review/approve_user_test.go`

**Interfaces:**
- Consumes: `port.ApproveUserCommand/ApproveUserHandler`（Task 2）、fake repo（Task 3，`Save/FindByID` 已支持权限变更）
- Produces: `command.NewApproveUserHandler(repo identity.UserRepository) port.ApproveUserHandler` —— Task 5 依赖

- [ ] **Step 1: 写失败测试**

Create `test/unit/user_review/approve_user_test.go`：

```go
package user_review

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/command"
	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

func TestApproveUser_PendingToUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(newUser(t, "bob", "bob@example.com", enum.PermissionPending))
	target := repo.users[0]
	handler := command.NewApproveUserHandler(repo)

	if err := handler.Handle(ctx, port.ApproveUserCommand{OperatorID: 99, UserID: target.AggregateID()}); err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	updated, err := repo.FindByID(ctx, target.AggregateID())
	if err != nil || updated == nil {
		t.Fatalf("find updated user failed: %v", err)
	}
	if updated.Permission() != enum.PermissionUser {
		t.Fatalf("expected permission user, got %s", updated.Permission())
	}
}

func TestApproveUser_RejectsNonPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(
		newUser(t, "alice", "alice@example.com", enum.PermissionUser),
		newUser(t, "carol", "carol@example.com", enum.PermissionAdmin),
	)
	handler := command.NewApproveUserHandler(repo)

	for _, u := range repo.users {
		if err := handler.Handle(ctx, port.ApproveUserCommand{OperatorID: 99, UserID: u.AggregateID()}); err == nil {
			t.Fatalf("expected error for user %s (perm %s), got nil", u.Name(), u.Permission())
		} else if !ierr.Is(err, ierr.ErrValidation) {
			t.Fatalf("expected ErrValidation, got %v", err)
		}
	}
}

func TestApproveUser_UserNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	handler := command.NewApproveUserHandler(newFakeUserRepo())

	err := handler.Handle(ctx, port.ApproveUserCommand{OperatorID: 99, UserID: 404})
	if err == nil {
		t.Fatalf("expected error for missing user, got nil")
	}
	if !ierr.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("expected ErrDataNotExists, got %v", err)
	}
}
```

（`ierr.Is` 是否存在请按实际 API 确认，不存在则改用 `ierr` 哨兵比较方式，参考现有测试或直接比对 `err` 类型；若项目无 `ierr.Is`，改用 `errors.Is` 或错误码断言。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./test/unit/user_review/... -run TestApproveUser`
Expected: 编译错误（`command.NewApproveUserHandler` 未定义）。

- [ ] **Step 3: 实现 usecase**

Create `internal/application/identity/command/approve_user.go`：

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

type approveUserHandler struct {
	repo identity.UserRepository
}

// NewApproveUserHandler 构造
func NewApproveUserHandler(repo identity.UserRepository) port.ApproveUserHandler {
	return &approveUserHandler{repo: repo}
}

// Handle 执行用户审核：仅允许 pending → user
func (h *approveUserHandler) Handle(ctx context.Context, cmd port.ApproveUserCommand) error {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		log.Error("[IdentityCommand] FindByID failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	if user == nil {
		log.Warn("[IdentityCommand] Target user not found for approve", zap.Uint("targetID", cmd.UserID))
		return ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	if user.Permission() != enum.PermissionPending {
		log.Warn("[IdentityCommand] Approve rejected, target not pending",
			zap.Uint("targetID", cmd.UserID), zap.String("permission", string(user.Permission())))
		return ierr.Newf(ierr.ErrValidation, "user %d is not pending", cmd.UserID)
	}

	user.ChangePermission(enum.PermissionUser)
	if err := h.repo.Save(ctx, user); err != nil {
		log.Error("[IdentityCommand] Save user failed", zap.Error(err), zap.Uint("targetID", cmd.UserID))
		return err
	}
	log.Info("[IdentityCommand] Approve user",
		zap.Uint("operatorID", cmd.OperatorID), zap.Uint("targetID", cmd.UserID))
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./test/unit/user_review/... -v`
Expected: 三个 `TestApproveUser_*` PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/application/identity/command/approve_user.go test/unit/user_review/approve_user_test.go
git commit -m "feat(user): ApproveUser usecase 与单测（仅 pending→user）"
```

---

### Task 5: DTO + handler + bootstrap 接线

**Files:**
- Modify: `internal/dto/user.go`
- Modify: `internal/handler/user.go`
- Modify: `internal/bootstrap/modules/handler.go`

**Interfaces:**
- Consumes: `port.ListUsersHandler`/`port.ApproveUserHandler`（Task 2-4）、`apiutil.WrapHTTPResponse`、`dto.CommonRsp`
- Produces: `handler.UserHandler.HandleListUsers/HandleApproveUser` —— Task 6 路由依赖

- [ ] **Step 1: DTO 新增**

`internal/dto/user.go` 末尾新增：

```go
// ListUsersReq 用户列表请求（管理员视图）
type ListUsersReq struct {
	model.CommonParam
	Permission string `query:"permission" enum:"pending,user,admin" doc:"按权限过滤，空=全部"`
}

// ListUsersRsp 用户列表响应
type ListUsersRsp struct {
	CommonRsp
	Items    []*UserItem      `json:"items,omitempty" doc:"用户列表"`
	PageInfo *model.PageInfo  `json:"pageInfo,omitempty" doc:"分页信息"`
}

// UserItem 用户列表项
type UserItem struct {
	ID         uint      `json:"id" doc:"用户ID"`
	Name       string    `json:"name" doc:"用户名"`
	Email      string    `json:"email" doc:"邮箱"`
	Avatar     string    `json:"avatar" doc:"头像"`
	Permission string    `json:"permission" doc:"权限"`
	CreatedAt  time.Time `json:"createdAt,omitzero" doc:"注册时间"`
	LastLogin  time.Time `json:"lastLogin,omitzero" doc:"最近登录时间"`
}

// ApproveUserReq 审核用户请求
type ApproveUserReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"User ID"`
}
```

新增 import：`"github.com/hcd233/aris-proxy-api/internal/common/model"`。

- [ ] **Step 2: handler 扩展**

`internal/handler/user.go`：

- `UserDependencies` 增加 `ListUsers port.ListUsersHandler`、`ApproveUser port.ApproveUserHandler`；`userHandler` struct 同步加两个字段；`NewUserHandler` 赋值。
- `UserHandler` 接口增加 `HandleListUsers(ctx, *dto.ListUsersReq) (*dto.HTTPResponse[*dto.ListUsersRsp], error)`、`HandleApproveUser(ctx, *dto.ApproveUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)`。
- 新增两个方法（放在 `HandleUpdateUser` 之后）：

```go
// HandleListUsers 用户列表（admin）
func (h *userHandler) HandleListUsers(ctx context.Context, req *dto.ListUsersReq) (*dto.HTTPResponse[*dto.ListUsersRsp], error) {
	rsp := &dto.ListUsersRsp{}
	views, pageInfo, err := h.listUsers.Handle(ctx, port.ListUsersQuery{
		CommonParam: req.CommonParam,
		Permission:  enum.Permission(req.Permission),
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] List users failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp.Items = lo.Map(views, func(v *port.UserView, _ int) *dto.UserItem {
		return &dto.UserItem{
			ID:         v.ID,
			Name:       v.Name,
			Email:      v.Email,
			Avatar:     v.Avatar,
			Permission: string(v.Permission),
			CreatedAt:  v.CreatedAt,
			LastLogin:  v.LastLogin,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleApproveUser 审核用户：pending → user（admin）
func (h *userHandler) HandleApproveUser(ctx context.Context, req *dto.ApproveUserReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	operatorID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if err := h.approveUser.Handle(ctx, port.ApproveUserCommand{
		OperatorID: operatorID,
		UserID:     req.ID,
	}); err != nil {
		logger.WithCtx(ctx).Error("[UserHandler] Approve user failed", zap.Error(err), zap.Uint("targetID", req.ID))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, nil)
}
```

新增 import：`"github.com/hcd233/aris-proxy-api/internal/common/enum"`（若未引入）与 `"github.com/samber/lo"`（`lo.Map` 使用）。

- [ ] **Step 3: bootstrap 接线**

`internal/bootstrap/modules/handler.go` 的 `NewUserDependencies`：

```go
func NewUserDependencies(getCurrentUser identityport.GetCurrentUserHandler, updateProfile identityport.UpdateProfileHandler,
	listUsers identityport.ListUsersHandler, approveUser identityport.ApproveUserHandler) handler.UserDependencies {
	return handler.UserDependencies{
		GetCurrentUser: getCurrentUser,
		UpdateProfile:  updateProfile,
		ListUsers:      listUsers,
		ApproveUser:    approveUser,
	}
}
```

同时在 fx 模块（`internal/bootstrap/modules/` 下提供 `NewListUsersHandler`/`NewApproveUserHandler` 的位置，参照 `NewGetCurrentUserHandler` 的注册方式，通常在 `modules/usecase.go` 或类似文件中 `fx.Provide` 列表里追加两个构造器）。

- [ ] **Step 4: 编译验证**

Run: `go build ./internal/...`
Expected: 编译通过。

- [ ] **Step 5: Commit**

```bash
git add internal/dto/user.go internal/handler/user.go internal/bootstrap/modules/handler.go
git commit -m "feat(user): DTO/handler/DI 接线用户列表与审核接口"
```

---

### Task 6: 路由注册 + 限流

**Files:**
- Create: `internal/common/constant/user.go`
- Modify: `internal/router/user.go`

**Interfaces:**
- Consumes: `handler.UserHandler` 新方法（Task 5）、`middleware.LimitUserPermissionMiddleware`、`middleware.TokenBucketRateLimiterMiddleware`
- Produces: `GET /api/v1/user/list`、`POST /api/v1/user/approve` 两个 admin 专属路由

- [ ] **Step 1: 限流常量**

Create `internal/common/constant/user.go`（参照 `internal/common/constant/apikey.go` 格式）：

```go
// Package constant 常量定义
package constant

import "time"

const (
	// PeriodManageUser 用户管理接口限流窗口
	PeriodManageUser = 1 * time.Minute
	// LimitManageUser 用户管理接口限流次数
	LimitManageUser = 20
)
```

- [ ] **Step 2: 路由注册**

`internal/router/user.go` 的 `initUserRouter` 中，`JwtMiddleware` 之后追加限流中间件，并在文件末尾注册两个新操作：

```go
	userGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		cache,
		"userManage",
		constant.CtxKeyUserID,
		constant.PeriodManageUser,
		constant.LimitManageUser,
	))
```

新增操作（仿照现有 huma.Register 块）：

```go
	huma.Register(userGroup, huma.Operation{
		OperationID: "listUsers",
		Method:      http.MethodGet,
		Path:        constant.RoutePathList,
		Summary:     "ListUsers",
		Description: "List all users with pagination, keyword and permission filter (admin only)",
		Tags:        []string{"User"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listUsers", enum.PermissionAdmin),
		},
	}, userHandler.HandleListUsers)

	huma.Register(userGroup, huma.Operation{
		OperationID: "approveUser",
		Method:      http.MethodPost,
		Path:        "/approve",
		Summary:     "ApproveUser",
		Description: "Approve a pending user to regular user (admin only)",
		Tags:        []string{"User"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("approveUser", enum.PermissionAdmin),
		},
	}, userHandler.HandleApproveUser)
```

- [ ] **Step 3: 编译验证 + 启动冒烟**

Run: `go build ./internal/... && go vet ./internal/router/...`
Expected: 无错误。

（可选冒烟：本地起服务后用 admin JWT 调 `GET /api/v1/user/list` 与 `POST /api/v1/user/approve?id=<pending id>`，观察 200/403 行为。）

- [ ] **Step 4: Commit**

```bash
git add internal/common/constant/user.go internal/router/user.go
git commit -m "feat(user): 注册 /user/list 与 /user/approve 路由（admin+限流）"
```

---

### Task 7: 前端 types + api-client

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api-client.ts`

**Interfaces:**
- Consumes: 后端 `ListUsersRsp`/`UserItem` 的 JSON 字段（`items`/`pageInfo`/`id`/`name`/`email`/`avatar`/`permission`/`createdAt`/`lastLogin`）
- Produces: `api.listUsers(page, pageSize, opts)`、`api.approveUser(id)` —— Task 8 页面依赖

- [ ] **Step 1: types.ts 新增类型**

`web/src/lib/types.ts` 的 `PageInfo` 定义附近追加：

```ts
// ─── User Management ────────────────────────────────────────────────────────────

export interface UserItem {
  id: number;
  name: string;
  email: string;
  avatar: string;
  permission: "pending" | "user" | "admin";
  createdAt: string;
  lastLogin: string;
}

export interface ListUsersRsp {
  items?: UserItem[];
  pageInfo?: PageInfo;
}
```

- [ ] **Step 2: api-client.ts 新增方法**

`web/src/lib/api-client.ts` 的 user 相关方法（`getCurrentUser` 附近）追加：

```ts
  async listUsers(
    page: number,
    pageSize: number,
    opts?: { query?: string; permission?: string },
  ): Promise<ListUsersRsp> {
    const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) });
    if (opts?.query) params.set("query", opts.query);
    if (opts?.permission) params.set("permission", opts.permission);
    return this.request<ListUsersRsp>(`/api/v1/user/list?${params}`);
  }

  async approveUser(id: number): Promise<CommonRsp> {
    return this.request<CommonRsp>(`/api/v1/user/approve?id=${id}`, { method: "POST" });
  }
```

- [ ] **Step 3: 类型检查**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误。

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/types.ts web/src/lib/api-client.ts
git commit -m "feat(web): api-client 新增 listUsers/approveUser"
```

---

### Task 8: 前端用户管理页 + 侧边栏 + i18n

**Files:**
- Create: `web/src/app/(dashboard)/users/page.tsx`
- Modify: `web/src/app/(dashboard)/layout.tsx`
- Modify: `web/src/locales/zh.json`
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ja.json`

**Interfaces:**
- Consumes: `api.listUsers/approveUser`（Task 7）、公共组件 `PageHeader/SearchInput/ListEmptyState/TableSkeleton/PaginationBar`、`PermissionGuard`、`usePersistentState`、`useIsMobile`、`showErrorToast`、`useT`
- Produces: `/users/` 管理页面（adminOnly）

- [ ] **Step 1: 页面组件**

Create `web/src/app/(dashboard)/users/page.tsx`（结构仿照 `web/src/app/(dashboard)/blocked/page.tsx`，列表 + 搜索 + 分页 + 权限筛选）：

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { PermissionGuard } from "@/components/permission-guard";
import type { PageInfo, UserItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { Users } from "lucide-react";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";

const PERMISSIONS = ["pending", "user", "admin"] as const;

export default function UsersPage() {
  const [items, setItems] = useState<UserItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.users.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.users.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [permission, setPermission] = useState<string>("");
  const [approving, setApproving] = useState<number | null>(null);
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

  /* eslint-disable react-hooks/set-state-in-effect -- Re-fetch when persisted state changes */
  useEffect(() => { fetchUsers(persistedPage, persistedPageSize, searchQuery || undefined, permission || undefined); }, [fetchUsers, persistedPage, persistedPageSize, searchQuery, permission]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleApprove = useCallback(async (user: UserItem) => {
    setApproving(user.id);
    try {
      await api.approveUser(user.id);
      toast.success(t("users.approved_success"));
      fetchUsers(pageInfo.page, pageInfo.pageSize, searchQuery || undefined, permission || undefined);
    } catch (err) {
      showErrorToast(err, { title: t("users.approve_error") });
    } finally {
      setApproving(null);
    }
  }, [fetchUsers, pageInfo.page, pageInfo.pageSize, permission, searchQuery, t]);

  return (
    <PermissionGuard adminOnly>
      <div className="page-surface mx-auto w-full max-w-6xl px-4 py-8">
        <PageHeader title={t("users.title")} description={t("users.subtitle")} icon={<Users className="size-5" />} />
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-3">
            <CardTitle>{t("users.list_title")}</CardTitle>
            <div className="flex items-center gap-3">
              <Select value={permission} onValueChange={(v) => { setPermission(v); setPersistedPage(1); }}>
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
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder={t("users.search_placeholder")}
              />
            </div>
          </CardHeader>
          <CardContent>
            {loading ? (
              <TableSkeleton rows={5} />
            ) : items.length === 0 ? (
              <ListEmptyState message={t("users.no_users")} />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("users.name")}</TableHead>
                    <TableHead>{t("users.email")}</TableHead>
                    <TableHead>{t("users.permission")}</TableHead>
                    <TableHead>{t("users.created_at")}</TableHead>
                    <TableHead>{t("users.last_login")}</TableHead>
                    <TableHead className="text-right">{t("common.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((u) => (
                    <TableRow key={u.id}>
                      <TableCell className="font-medium">{u.name}</TableCell>
                      <TableCell className="text-muted-foreground">{u.email}</TableCell>
                      <TableCell>
                        <span className="badge">{t(`permission.${u.permission}`)}</span>
                      </TableCell>
                      <TableCell className="text-muted-foreground">{new Date(u.createdAt).toLocaleString()}</TableCell>
                      <TableCell className="text-muted-foreground">{u.lastLogin ? new Date(u.lastLogin).toLocaleString() : "—"}</TableCell>
                      <TableCell className="text-right">
                        {u.permission === "pending" ? (
                          <Button
                            size="sm"
                            disabled={approving === u.id}
                            onClick={() => handleApprove(u)}
                          >
                            {approving === u.id ? t("common.processing") : t("users.approve")}
                          </Button>
                        ) : (
                          <span className="text-muted-foreground">—</span>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            <PaginationBar
              page={pageInfo.page}
              pageSize={pageInfo.pageSize}
              total={pageInfo.total}
              onPageChange={(p) => setPersistedPage(p)}
              onPageSizeChange={(s) => setPersistedPageSize(s)}
            />
          </CardContent>
        </Card>
      </div>
    </PermissionGuard>
  );
}
```

> 注意：`PageHeader` 的 props（title/description/icon 是否支持）、`SearchInput` props、`PaginationBar` props、badge 类名，均以 `blocked/page.tsx` 现有用法为准调整；若 blocked 页用了 `DeleteButton` 之外的其他按钮风格，按同款对齐。

- [ ] **Step 2: 侧边栏**

`web/src/app/(dashboard)/layout.tsx` 的 nav 数组（`nav.monitor` 之后）插入：

```ts
{ labelKey: "nav.users", href: "/users/", icon: <Users className="size-4" />, adminOnly: true },
```

确认 `lucide-react` 已 import `Users`（未引入则加入 import）。

- [ ] **Step 3: i18n 三语言**

`web/src/locales/zh.json`：

```json
"nav": { "...": "...", "users": "用户管理" },
"permission": { "pending": "待审核", "user": "普通用户", "admin": "管理员" },
"users": {
  "title": "用户管理",
  "subtitle": "审核注册用户并管理平台权限",
  "list_title": "用户列表",
  "permission_filter": "按权限筛选",
  "all_permissions": "全部权限",
  "search_placeholder": "搜索昵称或邮箱",
  "no_users": "暂无用户",
  "name": "昵称",
  "email": "邮箱",
  "permission": "权限",
  "created_at": "注册时间",
  "last_login": "最近登录",
  "approve": "批准为 User",
  "approved_success": "已批准该用户",
  "approve_error": "批准失败",
  "load_error": "无法加载用户列表"
}
```

`en.json` 与 `ja.json` 同步增加相同 key（英文：`nav.users: "Users"`、`users.*` 对应英文；日文按现有 ja.json 风格翻译）。`permission.*` 若与现有 profile 页的权限文案 key 冲突，复用已有 key 或改用 `users.permission_*` 前缀（以现有 `profile.permission` 相关 key 为准）。

- [ ] **Step 4: 类型检查 + lint**

Run: `cd web && npx tsc --noEmit && npx eslint src/app/\(dashboard\)/users src/lib/api-client.ts src/lib/types.ts src/app/\(dashboard\)/layout.tsx`
Expected: 无错误。

- [ ] **Step 5: Commit**

```bash
git add web/src/app/\(dashboard\)/users web/src/app/\(dashboard\)/layout.tsx web/src/locales/zh.json web/src/locales/en.json web/src/locales/ja.json
git commit -m "feat(web): 用户管理页 + 侧边栏入口 + i18n"
```

---

### Task 9: E2E 测试

**Files:**
- Create: `test/e2e/users/user_review_test.go`

**Interfaces:**
- Consumes: 后端两个接口（Task 6）、环境变量 `BASE_URL`/`ADMIN_TOKEN`/`USER_TOKEN`（`ADMIN_TOKEN` 先例见 `test/e2e/metrics/metrics_endpoint_test.go`）
- Produces: E2E 覆盖 admin 审核流程与普通用户 403

- [ ] **Step 1: 编写 E2E**

Create `test/e2e/users/user_review_test.go`（骨架仿照 `test/e2e/cron_trigger/cron_trigger_test.go`）：

```go
// Package users 验证用户审核闭环：admin 列表+批准 pending 用户，普通用户 403。
//
// 环境变量：
//   - BASE_URL     API 根地址（必填）
//   - ADMIN_TOKEN  管理员 JWT（必填）
//   - USER_TOKEN   普通用户 JWT（必填）
package users

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

const e2eHTTPTimeout = 30 * time.Second

func mustE2EEnv(t *testing.T) (baseURL, adminToken, userToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	adminToken = os.Getenv("ADMIN_TOKEN")
	userToken = os.Getenv("USER_TOKEN")
	if baseURL == "" || adminToken == "" || userToken == "" {
		t.Skip("BASE_URL, ADMIN_TOKEN and USER_TOKEN are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), adminToken, userToken
}

func doJSON(t *testing.T, client *http.Client, method, url, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, http.NoBody)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s failed: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return resp.StatusCode, body
}

type listUsersRsp struct {
	Items []struct {
		ID         uint   `json:"id"`
		Name       string `json:"name"`
		Permission string `json:"permission"`
	} `json:"items"`
}

func TestE2E_AdminCanListAndApprovePendingUser(t *testing.T) {
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 1. admin 列表：应至少能看到用户（本测试不依赖存在 pending 用户，仅验证接口可达与权限）
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=20", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list users expected 200, got %d: %s", status, body)
	}
	var rsp listUsersRsp
	if err := json.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal list response failed: %v", err)
	}

	// 2. 权限筛选 pending 用户（若有则批准，验证 approve 链路；无则跳过）
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=20&permission=pending", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list pending users expected 200, got %d: %s", status, body)
	}
	var pending listUsersRsp
	if err := json.Unmarshal(body, &pending); err != nil {
		t.Fatalf("unmarshal pending response failed: %v", err)
	}
	if len(pending.Items) == 0 {
		t.Skip("no pending users in environment, approve flow skipped")
	}
	target := pending.Items[0]
	status, body = doJSON(t, client, http.MethodPost, fmt.Sprintf("%s/api/v1/user/approve?id=%d", baseURL, target.ID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("approve user expected 200, got %d: %s", status, body)
	}

	// 3. 重复批准同一用户应失败（业务错误）
	status, _ = doJSON(t, client, http.MethodPost, fmt.Sprintf("%s/api/v1/user/approve?id=%d", baseURL, target.ID), adminToken)
	if status == http.StatusOK {
		t.Fatalf("re-approve expected non-200, got 200")
	}
}

func TestE2E_RegularUserGetsForbidden(t *testing.T) {
	baseURL, _, userToken := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=20", userToken)
	if status != http.StatusForbidden && status != http.StatusUnauthorized {
		t.Fatalf("regular user list expected 403/401, got %d: %s", status, body)
	}

	status, body = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/user/approve?id=1", userToken)
	if status != http.StatusForbidden && status != http.StatusUnauthorized {
		t.Fatalf("regular user approve expected 403/401, got %d: %s", status, body)
	}
}
```

- [ ] **Step 2: 本地编译验证**

Run: `go vet ./test/e2e/users/... && go build ./test/e2e/users/...`
Expected: 无错误。

- [ ] **Step 3: 执行 E2E（针对已部署环境）**

Run: `cd /Users/centonhuang/Desktop/code/aris-proxy-api/.worktrees/feature/user-review-2026-08-07 && BASE_URL=https://api.lvlvko.top ADMIN_TOKEN=<admin-jwt> USER_TOKEN=<user-jwt> go test ./test/e2e/users/ -v -count=1`
Expected: PASS（无 pending 用户时 approve 子流程 skip，列表与 403 用例通过）。

- [ ] **Step 4: Commit**

```bash
git add test/e2e/users/user_review_test.go
git commit -m "test(e2e): 用户审核闭环 E2E"
```

---

### Task 10: 全量验证与文档收尾

**Files:**
- Modify: `docs/superpowers/specs/2026-08-07-user-review-design.md`（如有实现偏差，同步修正）
- Modify: `CONTEXT.md`（如需补充领域词汇）

- [ ] **Step 1: 全量测试**

Run: `go test -count=1 ./...`
Expected: 全绿（原有用例不回归）。

- [ ] **Step 2: 全量 lint**

Run: `golangci-lint run ./...`（如项目配置了，按 `docs/agents/commands.md` 的 lint 命令执行）
Expected: 0 issues。

- [ ] **Step 3: 前端全量检查**

Run: `cd web && npx tsc --noEmit && npm run lint`
Expected: 无错误。

- [ ] **Step 4: ponytail-review 审查本次 diff**

Run: 对 `git diff master...HEAD` 执行过度工程审查（投机抽象、死代码、重复造轮子），删除可删项后重新跑测试。

- [ ] **Step 5: spec 一致性修正**

如有实现与 spec 偏差（如搜索参数名 query vs keyword、DTO 字段命名），同步更新 spec 文档并提交。

- [ ] **Step 6: 提交**

```bash
git add -A
git commit -m "chore: 全量验证与文档对齐"
```

---

## 自审记录（执行前完成）

- **Spec 覆盖**：列表接口（Task 1-3, 5-6, 9）、审核接口（Task 4-6, 9）、前端页面与入口（Task 8）、i18n（Task 8）、后端单测（Task 3-4）、E2E（Task 9）、验收标准 1-5（Task 9 的 403/401 断言 + Task 10 全量验证）——全覆盖。
- **类型一致性**：`ListUsersQuery{model.CommonParam; Permission enum.Permission}` → `ListUsersHandler → []*UserView, *model.PageInfo` → `ListUsersRsp.Items []*UserItem + PageInfo`；`ApproveUserCommand{OperatorID, UserID}` → `ApproveUserHandler` → `EmptyRsp`。各 Task 间签名一致。
- **已知待验证点（实现时以实际代码为准）**：
  1. `model.CommonParam` 是否可整体嵌入 port query 与 DTO（apikey 先例支持）；
  2. `ierr.Is` 是否存在（否则用 `errors.Is`/错误码断言）；
  3. `aggregate` 的构造函数名与 `vo` 类型（`RegisterUser` + `vo.UserName/Email/Avatar`）；
  4. 前端公共组件 props 与 badge 类名（以 blocked 页为准）；
  5. fx 模块中 usecase 构造器的注册位置。
