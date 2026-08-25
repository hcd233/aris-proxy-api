# API Keys 列表增加 User 列（用户名 + 头像）设计

- 日期：2026-08-25
- 状态：已确认
- 范围：现有 `GET /api/v1/apikey/list` 接口响应嵌套所属 user 信息 + 前端 apikeys 页面加 User 列

## 背景与目标

API Keys 页面（`/web/apikeys/`）当前只展示 key 名称、masked key、创建时间。管理
员视角（`PaginateAll`）下无法区分某个 key 属于哪个用户。目标是在**现有列表接口**
的每个 key 条目中嵌套所属用户对象，前端以 Avatar + 用户名形式展示。

不新建接口、不改路由、不改权限模型。

## 接口变更

`GET /api/v1/apikey/list` 响应的 `APIKeyItem` 嵌套 `user` 对象，对齐
`ModelItem.endpoint` 的既有嵌套风格（`dto/model.go:75`）：

```json
{
  "id": 1,
  "name": "my-key",
  "key": "sk-aris-***",
  "user": {
    "id": 1,
    "name": "centonhuang",
    "avatar": "https://avatars.example.com/u/1.png"
  },
  "createdAt": "2026-04-08T10:00:00Z"
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `user.id` | uint | 所属用户 ID |
| `user.name` | string | 所属用户名 |
| `user.avatar` | string | 所属用户头像 URL |
| `user` 整体 | object/null | legacy key（user_id=0）或用户已软删时整个字段缺省（omitempty） |

嵌套对象只含 `id/name/avatar` 三个字段：本需求只需用户名与头像，不透出
email/permission/lastLogin（不同于 model→endpoint 嵌套全量，因 models 页实际展示
endpoint 多字段，而 apikeys 页只用 name+avatar）。

## 数据流（与 model→endpoint 模式同构）

```
GET /api/v1/apikey/list
  → handler/apikey.go                      （CtxKeyUserID + permission，不变）
  → application/apikey/query/list_api_keys.go
      ① repo.PaginateAll / PaginateByUser   → []*ProxyAPIKey（已有 UserID()）
      ② loadUsers: lo.Uniq(keys.UserID) 过滤 0
         → identity.UserRepository.BatchFindByIDs → map[uint]*aggregate.User
      ③ view 组装: User: toUserView(usersByID[k.UserID()])   （nil → nil）
  → dto.APIKeyItem.User *APIKeyUser（handler 里 nil 判断后映射）
  → 前端 <Avatar> + 用户名
```

普通用户视角所有行都是自己，去重后仅 1 次 user 查询；admin 视角按页内去重
user 数查询（pageSize≤50，实测用户量级很小）。

## 方案决策

**采用 application 层批量组装 + 嵌套 DTO**，与 `listModelsHandler` /
`loadEndpoints` / `toEndpointView`（`internal/application/model/query/list_models.go`）
逐点同构：

| model→endpoint 既有模式 | apikey→user 本次改动 |
|---|---|
| `ModelItem.Endpoint *EndpointItem` | `APIKeyItem.User *APIKeyUser` |
| `ModelView.Endpoint *EndpointView`（port 定义） | `APIKeyView.User *UserView`（apikeyport 定义） |
| `endpointRepo.BatchFindByIDs → map[uint]*Endpoint` | `identity.UserRepository.BatchFindByIDs → map[uint]*User` |
| `loadEndpoints`（uniq + 批量，避免 N+1） | `loadUsers` |
| handler 里 `if v.Endpoint != nil { … }` | 同构 |

否决的备选：
- 扁平 `userName`/`userAvatar` 字段（audit 风格）——不符合本仓库对关联领域模型
  的嵌套惯例（用户确认）。
- 仿 audit `BatchGetRelations` 在 apikey 仓储加关系查询——apikey 聚合天生有
  `UserID()`，让 apikey 仓储裸调 User DAO 属于职责错位。
- SQL JOIN 一次查询——破坏"仓储返回聚合根"惯例。

## 边界情况

| 场景 | 行为 |
|---|---|
| `UserID==0` legacy key / 用户已软删 | `user` 字段缺省（nil + omitempty），前端显示 `—` |
| 非 admin 用户 | 所有行都是自己，同一逻辑（去重后 1 次查询） |
| demo 权限 | 路由 `LimitUserPermissionMiddleware(user)` 已拒绝，无脱敏需求 |
| BatchFindByIDs 查询失败 | 整个列表请求失败（fail-fast，与 loadEndpoints 行为一致） |

## 改动清单

### 后端

1. `internal/domain/identity/repository.go`：接口加
   `BatchFindByIDs(ctx context.Context, ids []uint) (map[uint]*aggregate.User, error)`
   （命名对齐 `llmproxy.EndpointRepository.BatchFindByIDs`；未找到的 ID 不出现在 map；
   入参过滤 0 与去重由实现负责）
2. `internal/infrastructure/repository/user_repository.go`：实现（去重过滤 0 →
   `dao.GetUserDAO().BatchGetByField(db, FieldID, ids, []string{FieldID, FieldName, FieldAvatar})`
   → `lo.SliceToMap` + `toUserAggregate`，字段列表内联，参考
   `audit_repository.go:226` 的精简字段先例）
3. `internal/application/apikey/port/handler.go`：`APIKeyView` 加 `User *UserView`；
   新增 `UserView{ID uint; Name string; Avatar string}`
4. `internal/application/apikey/query/list_api_keys.go`：注入
   `identity.UserRepository`；`loadUsers` + `toUserView` 组装
5. `internal/dto/apikey.go`：`APIKeyItem` 加
   `User *APIKeyUser `json:"user,omitempty" doc:"所属用户信息"``；新增
   `APIKeyUser{ID, Name, Avatar}`
6. `internal/handler/apikey.go`：`HandleListAPIKeys` 映射加 `if v.User != nil` 分支
7. `internal/bootstrap/modules/application.go`：`NewListAPIKeysHandler(repo, userRepo
   identity.UserRepository)` 签名加参（fx 自动解析，`NewUserRepository` 已注册）
8. 测试 stub：`test/unit/oauth2_callback/oauth2_callback_test.go`（stubUserRepo 有
   编译断言，需补 `BatchFindByIDs`）、`test/unit/user_review/fake_repo.go`（逐方法
   实现，需补）；`test/unit/demo_access_audit` 为嵌入接口无需改

### 前端

9. `web/src/lib/types.ts`：新增 `APIKeyUser{ id: number; name: string; avatar: string }`；
   `APIKeyItem` 加 `user?: APIKeyUser`
10. `web/src/app/(dashboard)/apikeys/page.tsx`：
    - 桌面表格：Name 列后加 User 列（`Avatar size="sm"` + AvatarImage/AvatarFallback
      + 用户名），`key.user` 缺省显示 `—`
    - mobile 卡片：同步加用户行
11. `web/src/locales/{en,zh,ja}.json`：`apikeys.user`：User / 用户 / ユーザー

## 测试与验收

1. 单测（`test/unit/application_apikey/`）：List 组装用例——
   - 正常填充：2 个 key 属于同一 user → view 的 `User` 嵌套对象字段正确
   - legacy：UserID=0 的 key → `User` 为 nil
   - mockUserRepository 需新增（含 BatchFindByIDs 可注入实现）
2. e2e（`test/e2e/apikeys/`，参考 `test/e2e/users/` 模式）：admin 登录 →
   list 接口返回的条目含 `user.name` 非空且与 users 表一致
3. `go build ./... && go test ./test/unit/...` 绿
4. `cd web && npm run lint && npm run build` 绿

## 部署

推 master 自动触发 `docker-publish.yml`（web/** 与 internal/** 均在 path filter），
无数据库 schema 变更，无需迁移。
