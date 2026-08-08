# Users 管理页三种操作（升级/降级/删除）设计文档

> 日期：2026-08-08
> 状态：已评审
> 分支：`feature/users-manage-actions-2026-08-08`

## 背景

当前 `web/src/app/(dashboard)/users/page.tsx` 仅支持管理员对 pending 用户执行「批准升级」（`POST /api/v1/user/approve?id=X`，admin only）。需求扩展为三种操作：

1. **删除用户**（pending/user）
2. **user → pending 降级**
3. **pending → user 升级**（已有 approve，保持不变）

同时要求移动端适配（当前已有移动端卡片布局，需保证新增操作在移动端可用）。

## 领域语义（沿用 CONTEXT.md 词汇表）

- **User**：自然人，权限三级：`pending`（功能受限）→ `user`（普通）→ `admin`（管理）。
- **Permission**：通过 `Permission.Level()` 比较等级。
- **APIKeyOwner**：API Key 属于某个 User（`user_id` 关联）。

## 决策记录

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| D1 | 删除用户语义 | **软删除 + 级联撤销 API Keys + 重新登录重新审核** | users 表已支持 `deleted_at` int64 软删；OAuth 唯一索引按 `(bind_id, deleted_at)` 区分，软删后同账号重登自动注册为新 pending 用户；仅删用户不动 Keys 会导致 Key 调用时 `userDAO.Get` 查无用户返回 500 |
| D2 | admin 保护 | **admin 完全只读 + 禁止操作自己** | 防止误删/自降级导致平台失控；前端隐藏菜单 + 后端 `ErrValidation` 兜底双重保护 |
| D3 | 前端交互 | **行操作统一 DropdownMenu（⋮ 更多操作）**，桌面表格与移动端卡片复用同一逻辑 | 三种操作收纳整洁、桌面/移动交互一致、规避三语言按钮宽度预留问题 |
| D4 | 删除 HTTP 方法 | **`DELETE /api/v1/user/delete?id=X`**（用户指定） | 语义化；approve/demote 沿用现有 `POST` 模式 |
| D5 | 删除原子性 | **`UserRepository.DeleteCascade` 事务内软删用户 + `BatchDeleteByField(user_id)` 软删 API Keys** | 仿照 `endpointRepository.DeleteCascade` 既有事务模式 |
| D6 | 确认交互 | 删除、降级需二次确认；升级直接执行 | 删除/降级影响面大，升级维持现状 |

## 后端设计

### DTO（`internal/dto/user.go`）

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

响应均复用 `EmptyRsp`（与 `ApproveUserReq` 一致）。

### Port（`internal/application/identity/port/handler.go`）

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

### Command（`internal/application/identity/command/`）

**`demote_user.go`**（规则）：
- 目标不存在 → `ErrDataNotExists`
- 目标是自己（`OperatorID == UserID`）→ `ErrValidation`
- 目标权限非 `user` → `ErrValidation`（天然覆盖 admin 与 pending）
- 通过校验 → `ChangePermission(PermissionPending)` + `Save`

**`delete_user.go`**（规则）：
- 目标不存在 → `ErrDataNotExists`
- 目标是自己 → `ErrValidation`
- 目标是 admin（`Permission() == PermissionAdmin`）→ `ErrValidation`
- 通过校验 → `repo.DeleteCascade(ctx, cmd.UserID)`

### Repository

**接口**（`internal/domain/identity/repository.go`）新增：

```go
// DeleteCascade 软删除用户及其全部 API Keys（事务保护）
DeleteCascade(ctx context.Context, id uint) error
```

**实现**（`internal/infrastructure/repository/user_repository.go`），仿照 `endpointRepository.DeleteCascade`：

```go
func (r *userRepository) DeleteCascade(ctx context.Context, id uint) error {
    db := r.db.WithContext(ctx)
    return db.Transaction(func(tx *gorm.DB) error {
        // 1. 批量软删该用户的 API Keys
        if err := dao.GetProxyAPIKeyDAO().BatchDeleteByField(tx, constant.FieldUserID, []uint{id}); err != nil {
            return ierr.Wrap(ierr.ErrDBDelete, err, "cascade delete api keys by user id")
        }
        // 2. 软删用户
        if err := r.dao.Delete(tx, &dbmodel.User{ID: id}); err != nil {
            return ierr.Wrap(ierr.ErrDBDelete, err, "delete user")
        }
        return nil
    })
}
```

### Handler（`internal/handler/user.go`）

- `HandleDemoteUser` / `HandleDeleteUser`，模式与 `HandleApproveUser` 完全一致（取 `OperatorID` → 调 port → 日志）。
- `UserDependencies` 增加 `DemoteUser`、`DeleteUser` 两个 port。
- DI 注入点：`internal/bootstrap/container.go` 中按现有 `ApproveUserHandler` 模式注册。

### Router（`internal/router/user.go`）

```go
huma.Register(userGroup, huma.Operation{
    OperationID: "demoteUser",
    Method:      http.MethodPost,
    Path:        "/demote",
    Summary:     "DemoteUser",
    Description: "Demote a regular user back to pending (admin only)",
    Tags:        []string{constant.TagUser},
    Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
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
    Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
    Middlewares: huma.Middlewares{
        middleware.LimitUserPermissionMiddleware("deleteUser", enum.PermissionAdmin),
    },
}, userHandler.HandleDeleteUser)
```

> 现有 `userGroup` 已挂载 `JwtMiddleware` + `TokenBucketRateLimiterMiddleware("userManage")`，自动覆盖新端点。

## 前端设计

### types.ts

后端 DTO 改动同步：

```ts
export interface DemoteUserReq {
  id: number;
}
export interface DeleteUserReq {
  id: number;
}
```

### api-client.ts

```ts
async demoteUser(id: number): Promise<CommonRsp> {
  return this.request<CommonRsp>(`/api/v1/user/demote?id=${id}`, { method: "POST" });
}
async deleteUser(id: number): Promise<CommonRsp> {
  return this.request<CommonRsp>(`/api/v1/user/delete?id=${id}`, { method: "DELETE" });
}
```

### users/page.tsx

**状态新增**：
- `pendingAction: { type: "promote" | "demote" | "delete"; user: UserItem } | null` —— 确认对话框目标
- `acting: { type: "promote" | "demote" | "delete"; id: number } | null` —— 请求进行中（替换现有 `approving`）
- `currentUserId: number | null` —— 挂载时调 `getCurrentUser` 获取，用于隐藏自己的操作菜单

**行操作 DropdownMenu（桌面表格 + 移动端卡片共用 `<UserRowActions>` 子组件）**：

```tsx
function UserRowActions({ user, currentUserId, onAction }: { ... }) {
  const canOperate = user.permission !== "admin" && user.id !== currentUserId;
  if (!canOperate) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("users.actions_aria")}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
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
```

**动作分发**：
- `promote` → 直接 `api.approveUser(id)`（现状逻辑不变，成功 toast + 刷新列表）
- `demote` → 打开确认框，确认后 `api.demoteUser(id)`
- `delete` → 打开确认框，确认后 `api.deleteUser(id)`

**确认框**：复用 `DeleteConfirmDialog`（通用确认框，含 loading 态）：
- 降级：title `users.demote_confirm_title`，desc `users.demote_confirm_desc`（说明功能受限）
- 删除：title `users.delete_confirm_title`，desc `users.delete_confirm_desc`（说明级联撤销 API Keys + 重新登录重新审核）

**移动端卡片**：右侧区域改为 `<UserRowActions>`，与桌面 actions 列一致。

### i18n（en/zh/ja 三语言新增 keys）

```
users.actions_aria        // 更多操作
users.promote             // 升为 User
users.demote              // 降级为 Pending
users.delete_menu         // 删除用户
users.demote_confirm_title
users.demote_confirm_desc
users.delete_confirm_title
users.delete_confirm_desc
users.demote_success
users.demote_error
users.delete_success
users.delete_error
```

> 注意：`web/CONTEXT.md` 的 i18n 布局稳定性契约——DropdownMenu 菜单项为弹性元素不强制预留；操作触发按钮为图标按钮（`size="icon"`）无文字宽度问题。

## E2E（`test/e2e/users/` 补充）

沿用现有 `user_review_test.go` 风格（环境变量 `BASE_URL`/`ADMIN_TOKEN`/`USER_TOKEN`，找不到目标则 `t.Skip`）：

1. **`TestE2E_RegularUserCannotManageUsers`**：普通用户调 `demote`/`delete` → 403/401（安全，不改变数据）
2. **`TestE2E_AdminDemoteThenReapprove`**：找第一个 user 权限用户 → `demote` 200 → 再 `approve` 200 恢复（闭环无副作用；找不到 user 用户则 skip）
3. **`TestE2E_AdminCannotDeleteSelf`**：`getCurrentUser` 拿 admin id → `delete` 自己 → 非 200 业务错误
4. **`TestE2E_AdminDeleteNonexistentUser`**：`delete` 不存在的 id（如 999999）→ 非 200 业务错误

## 验证清单

- [ ] `cd web && npm run lint && npm run build`（类型 + 静态导出）
- [ ] `go build ./...`、`go vet ./...`
- [ ] 后端单测（如有受影响用例）
- [ ] 本地起后端（`go run ./cmd/server server start --host localhost --port 8080`）+ `npm run dev`，浏览器验证：
  - 桌面表格：pending/user 行显示 ⋮ 菜单，admin/自己无菜单
  - 移动端（Chrome devtools 模拟视口）：卡片布局菜单可用
  - 升级直接执行；降级/删除二次确认后执行；操作后列表刷新
- [ ] E2E 用例跑通（生产环境或本地全链路）

## 不在范围内（YAGNI）

- 不做批量操作（多选删除）
- 不做 admin 的管理操作（admin 只读，无删除/降级入口）
- 不物理删除用户数据（软删除 + 历史数据保留）
- 不新增"停用/启用"状态（当前 pending 已承载功能受限语义）
