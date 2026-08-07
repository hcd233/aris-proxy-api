# 用户审核闭环（pending → user）设计文档

> 日期：2026-08-07
> 分支：`feature/user-review-2026-08-07`
> 状态：待评审

## 背景

系统支持 OAuth2（GitHub / Google）注册登录，新用户默认权限为 `pending`，经审核升为 `user` 或 `admin`（`internal/domain/identity/aggregate/user.go` `RegisterUser`）。但目前**管理员侧没有任何用户管理能力**：`UserHandler` 仅有 `HandleGetCurUser` / `HandleUpdateUser`（本人档案），路由无用户列表资源。新用户注册后只能 SSH 到生产库手工改权限，运营断点真实存在。

本功能补齐最小审核闭环：管理员在 Web 后台查看用户列表并审核 `pending` 用户，`pending → user` 后用户即可正常使用网关。

## 需求决策（已与用户确认）

1. **本期只做 `pending → user`**：审核升权是唯一权限变更操作。不做降级（`user → pending`、`admin → user`）、不做 `user → admin` 的界面入口（后端校验也仅允许 `pending → user`）。
2. **接口形态**：列表用 `GET /user/list`（沿用现有 user group base path `/user` 与列表约定 `RoutePathList="/list"`），审核操作用 `POST /user/approve`（**无路径参数**，目标用户 ID 走 `query:"id"`，与项目所有按 ID 定位资源的接口惯例一致，如 `DeleteAPIKeyReq`），而非通用 `PATCH /permission`——业务规则只有一种变更，专用接口在路由层即可表达语义，无需在 DTO 里做枚举校验。
3. **目标非 `pending` 一律拒绝**：重复批准（已是 `user`）或试图操作 `admin`/`user` 返回业务错误，前端提示。
4. **不建操作审计表**：权限变更走结构化 logger 记录（操作者 + 目标 + 结果）。现有审计体系（ModelCallAudit / CronCallAudit）均为业务运行维度，为单次权限变更建表属于过度工程。
5. **列表能力**：`GET /user/list` 支持分页 + 关键词（name/email 模糊）+ 权限筛选（不传则全部），管理员可以筛出 `pending` 待审用户，也能总览系统用户。

## 核心思路

用户审核 = 在既有 identity 领域能力（`ChangePermission` 聚合方法 + `UserRepository.Save`）之上补齐**管理员视角的查询与变更入口**，全部复用现有分层（handler → application port/usecase → domain aggregate → repository），不新建领域概念、不改权限枚举。

### 涉及链路（现状 → 新增）

| 层 | 现状 | 新增 |
|---|---|---|
| 路由 | `internal/router/user.go` 仅 2 个操作 | 注册 `listUsers`（GET `/user/list`）、`approveUser`（POST `/user/approve`），均挂 `LimitUserPermissionMiddleware(admin)` + 管理类限流中间件（复用 apikey 路由模式） |
| handler | `UserHandler` 2 方法 | `HandleListUsers` / `HandleApproveUser` |
| port | `GetCurrentUserHandler` / `UpdateProfileHandler` | `ListUsersHandler` / `ApproveUserHandler`（+ `UserView` 已有，可直接复用做列表项投影） |
| usecase | — | 新增 list / approve 两个用例（identity 应用层） |
| repository | `FindByID` / `FindByGithubBindID` / `FindByGoogleBindID` / `TouchLastLogin` / `Save` | 新增 `List(ctx, filter)` 分页查询（走现有 dao 分页模式） |
| 前端 | profile 页 | 新增 adminOnly「用户管理」页 + api-client 方法 + i18n 键 |

## 架构与改动清单

### 1. 后端

#### 1.1 路由（`internal/router/user.go`）

```go
// GET /api/v1/user/list  admin 专属（userGroup base path = /user）
huma.Register(userGroup, huma.Operation{
    OperationID: "listUsers",
    Method:      http.MethodGet,
    Path:        constant.RoutePathList, // "/list"
    ...
    Middlewares: huma.Middlewares{
        middleware.LimitUserPermissionMiddleware("listUsers", enum.PermissionAdmin),
    },
}, userHandler.HandleListUsers)

// POST /api/v1/user/approve  admin 专属，无路径参数，目标用户 ID 走 query
huma.Register(userGroup, huma.Operation{
    OperationID: "approveUser",
    Method:      http.MethodPost,
    Path:        "/approve",
    ...
    Middlewares: huma.Middlewares{
        middleware.LimitUserPermissionMiddleware("approveUser", enum.PermissionAdmin),
    },
}, userHandler.HandleApproveUser)
```

限流：`userGroup` 增加管理类 `TokenBucketRateLimiterMiddleware`（key 维度与现有管理接口一致），参考 `initAPIKeyRouter` 模式。

#### 1.2 DTO（`internal/dto/user.go`）

```go
type ListUsersReq struct {
    Page       int    `query:"page" required:"true" minimum:"1"`                                     // 页码（沿用 session 分页模式）
    PageSize   int    `query:"pageSize" required:"true" minimum:"1" maximum:"500" default:"20"`       // 每页条数
    Keyword    string `query:"keyword" maxLength:"200"`                                               // name/email 模糊
    Permission string `query:"permission" enum:"pending,user,admin"`                                   // 空=全部
}
type ListUsersRsp struct {
    Items []*UserItem `json:"items"`
    Total int64       `json:"total"`
}
type UserItem struct { // 列表投影，字段与 DetailedUser 对齐
    ID         uint      `json:"id"`
    Name       string    `json:"name"`
    Email      string    `json:"email"`
    Avatar     string    `json:"avatar"`
    Permission string    `json:"permission"`
    CreatedAt  time.Time `json:"createdAt"`
    LastLogin  time.Time `json:"lastLogin"`
}
type ApproveUserReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"User ID"`
}
```

#### 1.3 port（`internal/application/identity/port/handler.go`）

```go
type ListUsersQuery struct {
    Page       int
    PageSize   int
    Keyword    string
    Permission string
}

type ListUsersHandler interface {
    Handle(ctx context.Context, q ListUsersQuery) ([]*UserView, int64, error)
}

type ApproveUserCommand struct {
    OperatorID uint // 操作者，用于日志
    UserID     uint // 目标用户
}

type ApproveUserHandler interface {
    Handle(ctx context.Context, cmd ApproveUserCommand) error
}
```

#### 1.4 usecase

- **ListUsers**：校验 page/pageSize 合法（沿用现有分页校验），组装过滤条件调 `UserRepository.List`，返回 `UserView` 列表 + total。
- **ApproveUser**：
  1. `FindByID(userID)`，不存在 → `ErrDataNotExists`
  2. 目标 `Permission != pending` → 业务错误（重复批准/非法目标），消息明确（如 `user %d is not pending`）
  3. `user.ChangePermission(enum.PermissionUser)`（领域方法）
  4. `UserRepository.Save(ctx, user)`
  5. logger 记录 `[UserUseCase] Approve user`：operatorID / targetID / 结果
  6. 操作者与目标相同的场景：admin 自己不会是 pending，天然无自伤路径，无需额外防呆。

#### 1.5 repository（`internal/infrastructure/repository/user_repository.go`）

新增 `List(ctx, page, pageSize, keyword, permission)`：走 `dbmodel.User` dao 分页查询，`keyword` 用 `name LIKE ? OR email LIKE ?`（参照 `session` 列表 keyword 模式，量级为个人系统无需 trigram），`permission` 非空时精确过滤；返回聚合列表 + total。

### 2. 前端

#### 2.1 侧边栏（`web/src/app/(dashboard)/layout.tsx`）

新增 `{ labelKey: "nav.users", href: "/users/", icon: <Users .../>, adminOnly: true }`。

#### 2.2 api-client（`web/src/lib/api-client.ts`）

```ts
listUsers(params: { page?: number; pageSize?: number; keyword?: string; permission?: string })
approveUser(id: number)
```

#### 2.3 页面（`web/src/app/(dashboard)/users/page.tsx`）

- 复用公共组件：`PageHeader` / `SearchInput` / `ListEmptyState` / `TableSkeleton`（8/4 重构产物）
- 列表列：头像、名称、邮箱、权限徽标（复用 models 页能力徽章 Tooltip 模式）、注册时间、最近登录、操作（pending 行显示「批准为 User」按钮，非 pending 显示 `—`）
- 权限筛选下拉（全部 / pending / user / admin）
- 批准操作：确认对话框（复用 `useDeleteConfirm` 同源 hook 或 `confirm` 组件）→ 调 `approveUser` → 成功 toast + 刷新列表；失败走统一错误展示（`showErrorToast`）
- 权限控制：页面组件层按现有 adminOnly 路由守卫机制处理

#### 2.4 i18n

新增 `nav.users` 与页面文案（列表标题/筛选/批准按钮/确认文案/成功提示），沿用现有 locale 结构与语序模板。

### 3. 测试

#### 3.1 后端单测

- `ApproveUser`：pending→user 成功；目标非 pending 拒绝；目标不存在拒绝
- `ListUsers`：分页 / keyword / permission 过滤

#### 3.2 E2E（`test/e2e/users/`，沿用现有骨架）

- 管理员：list users → approve pending 用户 → 再次 approve 返回业务错误
- 普通用户（user）：访问 users 接口返回 403

## 验收标准

1. pending 用户可在管理后台可见，批准为 `user` 后无需任何 SQL 操作即可正常使用网关（新建 API Key / 调用模型）。
2. 普通用户访问 `GET /api/v1/user/list` 或 `POST /api/v1/user/approve` 返回 403；未登录返回 401。
3. 重复批准 / 操作非 pending 用户返回明确业务错误。
4. 权限变更日志可追溯（操作者 + 目标 + 结果）。
5. `go test -count=1 ./...` 全绿；前端 `tsc --noEmit` + ESLint 通过；E2E `test/e2e/users/` 通过。

## 非目标（本期明确不做）

- 用户降级 / 删除 / 封禁
- `user → admin` 的界面操作与审核流
- 批量审核
- 邀请制注册
- 独立操作审计表
- 用户管理的实时同步 / 通知（如邮件通知用户"已通过审核"）
