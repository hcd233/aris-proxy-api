# 公开端点（Endpoint Visibility）设计

> 日期：2026-10-21
> 分支：`feature/public-endpoint-visibility-2026-10-21`
> 目标：admin 创建 endpoint 时可标记为「公开」，公开端点及其模型对普通用户只读可见、可调用；普通用户自建端点保持私有且可自管。

## 背景

当前 endpoint / model 是**纯多租户自助**模型：路由门槛 `PermissionUser`，普通用户可对自己名下的端点与模型做增删改（`internal/router/endpoint.go:30`、`internal/router/model.go:74`），admin 通过 `scopeFor` 拿到 `nil` scope 看全量、可代建（`internal/handler/endpoint.go:260`）。

代码里已存在一个「共享池」雏形：`user_id = 0` 的存量配置 + `gateway.shared_pool_fallback` 开关，在用户租户未命中 alias 时回退查询（`internal/domain/llmproxy/service/resolver.go:126`）。但它有两个问题：一是「无归属」表达不了「admin 拥有但对所有人公开」，二是行为被一个过渡期配置开关控制。

本需求把「公开」提升为一等公民：admin 显式声明端点可见性，网关对所有用户开放这批模型。

## 目标

1. admin 创建 / 编辑 endpoint 时可以指定 `visibility: public | private`（默认 `private`）。
2. 公开端点及其名下模型对所有普通用户与 demo 账户**可见**；普通用户可调用，demo 只读且仍然不能调用 LLM（demo 的转发在现有链路已被拒绝，本次不改变该行为）。
3. 普通用户自建端点永远私有；普通用户请求里出现 `visibility=public` 一律拒绝。
4. alias 解析优先级：**自己租户命中 → 完全用自己；未命中 → 公开池**。
5. 所有列出「用户能用的模型」的读路径（Web 列表、`/v1/models`、CLI 导出）口径一致：公开模型必须出现，否则客户端会看到「列表里有但调不通」。

## 非目标（明确不做）

- 不做模型级 visibility：模型可见性完全继承所属 endpoint。
- 不做「按用户授权某个公开端点」的白名单（目前公开 = 全员）。
- 不做公开端点的用户级配额、计费分成或成本归属（审计仍记调用者与其 API Key）。
- 不引入新的 `Permission` 枚举值：公开端点的只读性由现有租户隔离天然保证，见下。
- 不改动 `gateway.shared_pool_fallback` 与共享池存量数据；公开池是新机制，两者共存。

## 领域语义

新增词汇（需同步写入根 `CONTEXT.md`）：

**EndpointVisibility（端点可见性）**：endpoint 的访问范围属性，`private`（默认，仅归属者与 admin 可见）或 `public`（全体登录用户可见只读、可调用）。只有 admin 能创建或修改为 `public`；公开端点的归属仍是创建它的 admin。

**PublicEndpoint（公开端点）**：`visibility = public` 的 endpoint。其名下 model 随之公开；普通用户调用的仍是**自己的 API Key**，但向上游鉴权用的是该端点配置的共享密钥，因此消耗 admin 的上游额度/订阅。

## 方案选型

| 方案 | 说明 | 结论 |
|---|---|---|
| A. 复用共享池 `user_id = 0` | 零迁移，公开 = 无归属 | 放弃：语义错位（「无归属」≠「公开」）；受 `shared_pool_fallback` 开关控制；无法表达归属 admin；与既有「owner 不能为 0」校验冲突 |
| B. endpoint 布尔列 `is_public` | 加一列 bool | 放弃：用户明确要求用 `visibility: public/private` |
| C. endpoint 枚举列 `visibility`（选定） | `private`/`public` 两值字符串枚举 | 可扩展（未来若加 `unlisted` 不用改语义）；显式表达归属与可见性的分离；迁移成本仅一列 |

## 数据模型与迁移

`internal/infrastructure/database/model/endpoint.go` 新增列：

```go
Visibility string `json:"visibility" gorm:"column:visibility;not null;default:'private';comment:可见性(private/public)"`
```

- AutoMigrate 加列时 PostgreSQL 以 `DEFAULT 'private'` 回填存量行，无需单独 backfill。
- **`constant.EndpointRepoFieldsFull`（`internal/common/constant/sql.go:153`）必须同步加上新列**：仓储用显式字段白名单查询，漏加会让聚合根的 `visibility` 恒为零值（等价 private），表现为「存了公开、读出来不公开」。
- **不加索引**：endpoint 是配置表（每个用户个位数端点），`visibility` 选择性极低，全表扫描成本可忽略。若未来端点数量级上升再补部分索引 `WHERE visibility = 'public'`（`// ponytail: 配置表不建索引，量级上升再补 partial index`）。

## 领域层

`internal/common/enum/endpoint_visibility.go`（新文件，对齐 `endpoint_auth_type.go` 的写法）：

```go
type EndpointVisibility string

const (
    EndpointVisibilityPrivate EndpointVisibility = "private"
    EndpointVisibilityPublic  EndpointVisibility = "public"
)

func (v EndpointVisibility) IsValid() bool  // 基于 EndpointVisibilities 集合判断（对齐 OAuthProviders 写法）
func (v EndpointVisibility) IsPublic() bool
func (v EndpointVisibility) Normalize() EndpointVisibility // "" → private
```

`internal/domain/llmproxy/aggregate/endpoint.go`：

- 结构体新增 `visibility enum.EndpointVisibility`，`newEndpoint` 里 `visibility.Normalize()`（零值即 private，存量/漏传不产生脏数据）。
- 新增 `Visibility()` getter 与 `SetVisibility(v) error`（非法值返回 `ierr.ErrValidation`）。
- **不扩 `CreateEndpoint` / `Update` 的参数列表**：两处签名已经很长（10 / 9 个参数），再加一个只会让每个调用点更难读。可见性由 setter 在命令处理器里写入，`Update` 之后的 `SetVisibility` 与其它字段一起落库。
- `RehydrateEndpoint` 不校验，仓储读路径构造后调 `SetVisibility`（与 `SetUserID`/`SetTimestamps` 同风格）。

仓储（`internal/infrastructure/repository/endpoint_repository.go`）：

- `toEndpointAggregate` 恢复 `Visibility`；`toEndpointModel` 写回 `Visibility`。
- `Update` 的 `updates` map 增加 `constant.FieldEndpointVisibility`。

## 权限规则

权限判定集中在 handler 层（两条规则），命令层只补一处降级保护：

1. **创建**（`internal/handler/endpoint.go` → `HandleCreateEndpoint`）
   - `visibility` 缺省 = `private`。
   - 非 admin 显式传 `visibility`（无论 public 还是 private）→ `ierr.ErrUnauthorized`，不静默忽略。
   - `visibility = public` 时，归属强制为操作者本人：忽略 `ownerUserID`，不允许代建公开端点给普通用户。
2. **更新**（`HandleUpdateEndpoint`）
   - 非 admin 显式传 `visibility` → `ierr.ErrUnauthorized`。
   - admin 可以 public ↔ private 切换，但**只能改自己名下端点**的可见性（见下）。
   - **未传 `visibility` = 保持原值**，不是重置为 private：聚合根由 `FindByID` 从 DB 恢复，`EndpointRepoFieldsFull` 带上新列后 `ep.Visibility()` 已是当前值；命令层只在 `cmd.Visibility != nil` 时调 `SetVisibility`，`repo.Update` 随后整体写回的仍是原值。
   - 唯一会写 endpoint 整行的是 `Update`；额度刷新走 `UpdateQuota` 窄列更新、不会碰 visibility，因此不存在「后台任务把公开端点静默改回私有」的路径。

**可见性只能由归属者本人修改**（CR：只校验 admin 会留下跨租户发布漏洞）：`UpdateEndpointCommand` 带 `OperatorUserID`，命令内 `cmd.Visibility != nil && ep.UserID() != cmd.OperatorUserID` → `ierr.ErrUnauthorized`。否则 admin 可以把普通用户（或另一个 admin）名下的私有端点直接改成 public——等于替别人把上游密钥/额度共享给全平台，且归属者自己反而改不回来（会被「非 admin 不得改公开端点」拦住）。前端同步：编辑他人端点时可见性选择器禁用。

**普通用户改不了公开端点**：不需要新权限分支。update / delete / OAuth reauth / quota 全部走 `FindByID(ctx, id, scope)`，非 admin 的 scope 恒为自身 userID，公开端点归属是 admin，因此一律返回「endpoint not found」（`ierr.ErrDataNotExists`）。这与现有跨租户越权行为一致，不泄漏端点是否存在。

**降级保护（边界情况）**：admin 被降为 `user` 后，其名下已有的公开端点仍会被自己 scope 命中，从而可以改连接字段或删除——与「只有 admin 能维护公开端点」冲突。处理：`UpdateEndpointCommand` / `DeleteEndpointCommand` / `ReauthOAuthCommand` 增加 `IsAdmin bool`，命令内取到聚合后若 `ep.Visibility().IsPublic() && !cmd.IsAdmin` 返回 `ierr.ErrUnauthorized`。
（备选方案「降级时把其公开端点自动转私有」需要 identity 模块反向依赖 llmproxy，跨模块耦合更贵，不采用。）

**写路径守卫矩阵**（结论：endpoint 三个命令 + model 三个命令都需要 `IsAdmin`；模型侧在降级场景下靠归属拦不住）

| 写操作 | 拦截机制 |
|---|---|
| endpoint update / delete | `FindByID(scope)` 拦住跨租户；可见性变更额外要求归属者本人；降级场景由 `IsAdmin` 守卫 |
| endpoint OAuth reauth | 同上（`ScopeUserID` + `IsAdmin`） |
| endpoint OAuth quota probe | `FindByID(scope)` 已拦住（只读探测，不额外加 `IsAdmin`） |
| model create（带 `endpointID`） | 目标端点 public 且 `!IsAdmin` → `ErrUnauthorized` |
| model update / delete | 模型所属端点 public 且 `!IsAdmin` → `ErrUnauthorized`（降级 admin 仍能命中自己名下的公开端点） |
| 模型启用/禁用 | 走 model update，同上 |

> 为什么模型侧也要守卫：降级后的 admin 其 `user_id` 恰好命中自己名下的公开端点与模型，`FindByID(scope)` 不会拦，必须补 `IsAdmin`。

**OAuth 凭据投影的跨租户脱敏**：`upstream/list` 对 OAuth 端点会装配账号邮箱/订阅类型/额度/最近错误。`toOAuthView` 在 demo 视角已掩码邮箱；公开端点引入后，**非归属者**（普通用户看他人公开端点）必须同样掩码邮箱并清空 `lastError`（复用 demo 口径），否则等于把 admin 的个人邮箱与订阅错误详情下发给全员。归属者与 admin 视角保持完整值。

**没有单条详情读接口**：endpoint 只有 list/create/update/delete/oauth*，model 只有 list/create/update/delete，因此不存在「普通用户 GET 公开端点详情拿到密钥」的路径。列表只下发 `MaskedAPIKey`；`baseURL` 对普通用户可见（公开端点的连接地址本就是共享信息），demo 视角的既有脱敏保持不变。若未来新增详情接口，必须复用同一脱敏口径。

## 解析链路

`internal/infrastructure/repository/model_repository.go` 新增：

```go
// FindPublicByAlias 查询公开端点下启用中的模型（网关解析专用）
FindPublicByAlias(ctx context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error)
```

实现用子查询而非 join（与读路径保持同一写法）：

```sql
SELECT * FROM models
WHERE alias = ? AND enabled = true
  AND endpoint_id IN (SELECT id FROM endpoints WHERE visibility = 'public' AND deleted_at = 0)
```

`internal/domain/llmproxy/service/resolver.go` 的 `collectCandidates` 改为：

```
1. FindByAlias(alias, &tenantID)                 // 自己的租户
2. 空 → FindPublicByAlias(alias)                 // 公开池（无条件，不受 shared_pool_fallback 控制）
3. 仍空且 sharedPoolFallback → FindByAlias(alias, &sharedPoolID)  // 存量共享池，行为不变
4. 无候选 → ErrDataNotExists
```

同时把「取 endpoint 的 scope」从 `models[0].UserID()` 特判改为**逐模型取 `m.UserID()`**：

```go
for _, m := range models {
    owner := m.UserID()
    ep, err := r.endpointRepo.FindByID(ctx, m.EndpointID(), &owner)
```

理由：公开模型可能来自**多个 admin**（两个 admin 都公开了同名 alias），`models[0]` 不再代表全部；逐模型 scope 同时天然保留共享池原有的「不得借用任意用户 endpoint」保护。

**依赖的不变量**：`models.user_id == endpoints.user_id`。创建 model 时归属由命令从 endpoint 继承（`Create(ctx, m, ep.UserID())`），共享池存量行两边都是 0，因此逐模型 scope 与旧逻辑等价。若历史脏数据破坏该不变量，`FindByID` 返回 nil、该候选被跳过（fail-closed），不会退化成跨租户借用；实现时补一条单测覆盖。

已知语义细节（沿用现有回退逻辑）：用户把同名 alias 的模型**禁用**后，`FindByAlias` 返回空，会回退到公开池——与今天禁用后回退共享池一致。

## 查询链路

统一引入 `includePublic`，由 handler 按权限计算。**「用户能用的模型」的读路径全集**（无遗漏，其余路径都复用它们）：

| 读路径 | 消费方 | 落地方法 |
|---|---|---|
| `/api/openai/v1/models`、`/api/anthropic/v1/models` | LLM 客户端列模型 | `EndpointReadRepository.ListAliases`（`usecase/query.go:34`、`:66`） |
| `/api/cli/v1/model/list` | `aris model export` 客户端配置导出 | `EndpointReadRepository.ListEnabledModelDetails` |
| Anthropic `/v1/messages/count_tokens` | Claude Code 估算 token | `EndpointReadRepository.FindEndpointByAlias`（`usecase/query.go:104`） |
| Web `/upstream/list` | 管理页分组视图 | `EndpointRepository.FindIDsByScope` |
| Web `/model/list` | 管理页平铺视图 | `ModelRepository.PaginateWithFilter` |
| LLM 转发、playground | 实际调用 | `EndpointResolver.ResolveCandidates*`（见「解析链路」） |

权限口径（handler 计算后透传到 query port）：

| 视角 | scope | includePublic |
|---|---|---|
| admin（无 username 筛选） | `nil` | `false`（nil 本身就是全量） |
| admin（username 筛选） | `&uid` | `false` |
| 普通用户 / demo | `&userID` | `true` |

1. **Upstream 分组列表**（`internal/application/upstream/query/list_upstream.go`）
   - `port.ListUpstreamQuery` 增加 `IncludePublic bool`（handler 计算）。
   - `EndpointRepository.FindIDsByScope(ctx, scope, includePublic)` 增加布尔参数；`includePublic=true` 时条件为
     `user_id = ? OR visibility = 'public'`。
   - 组内模型仍由 `ListByEndpointIDs(allIDs)` 装载，无需改动。
2. **Web 平铺模型列表**（`internal/application/model/query/list_model.go`）
   - `port.ListModelQuery` 增加 `IncludePublic bool`，透传到 `llmproxy.ModelListFilter.IncludePublic`；
     `loadModelPage` 在 `scopeUserID != nil` 且 `IncludePublic` 时改为
     `user_id = ? OR endpoint_id IN (SELECT id FROM endpoints WHERE visibility='public' AND deleted_at = 0)`。
   - 缓存键 `modelListCacheKey` 已经把 filter 整体序列化进签名（`internal/infrastructure/repository/read_cache.go:41`），新字段自动进键，不会与旧缓存串味。
3. **网关模型目录**（`internal/infrastructure/repository/endpoint_repository.go` 读模型）
   - `loadAliases` / `loadModelDetails` / `FindEndpointByAlias` 的查询从 `user_id = ?` 改为
     `user_id = ? OR endpoint_id IN (SELECT id FROM endpoints WHERE visibility='public' AND deleted_at = 0)`。
   - 这两处都用 `lo.UniqBy(alias)` 去重（`endpoint_repository.go:353`、`:387`），同名的 own 与 public 行、或多个 admin 的公开行只保留**最先出现**的一条。为保证「取的规格/定价」与解析结果（own 优先）一致，查询加 `ORDER BY (user_id = ?) DESC`，让 own 行排在前面。`ModelAliasProjection` / `ModelDetailProjection` 不带归属字段，因此只影响规格与定价取值，不改变对外结构。
   - 缓存键 `cache:rd:alias:<userID>` / `cache:rd:detail:<userID>` 不变；写路径已 `InvalidateAll`（含新增的 visibility 更新），不会残留。

**公开条件的复用**：`visibility = 'public' AND deleted_at = 0` 会在 4 处 SQL 出现（解析 1 处 + 读模型 3 处）+ 2 处 `includePublic` 分支。在 `internal/infrastructure/repository` 里定义一个条件表达式/构造函数统一复用，禁止各处手写漂移。endpoint 表没有 enabled/status 列，公开池不需要额外过滤（模型侧 `enabled = true` 由各自查询带）。

## 接口契约

`internal/dto/endpoint.go`：

```go
// CreateEndpointReqBody / UpdateEndpointReqBody 新增
Visibility *enum.EndpointVisibility `json:"visibility,omitempty" enum:"private,public" doc:"可见性(仅管理员可设为 public)"`
```

类型选择对齐 `enum.Oauth2Platform` 的既有写法（`internal/dto/oauth2.go:11`）；若实现期发现 huma 对「指针 + 自定义 string 类型 + enum 校验」行为不符合预期，退化为 `*string`，在 handler 里 `Normalize()` + `IsValid()` 校验，接口语义不变。

`internal/dto/upstream.go` 的 `UpstreamEndpointItem` 新增 `Visibility string \`json:"visibility"\``，
`internal/application/upstream/port/handler.go` 的 `UpstreamEndpointView` 同步新增，`toEndpointView` 映射。

平铺模型列表的所属端点 `ModelListEndpointItem`（`internal/dto/model.go:106`）目前只带 `ID` / `Name`，新增 `Visibility string \`json:"visibility"\``；应用层 `ListModelEndpointView`（`internal/application/model/port/list_model.go:28`）与 `toListModelView` 同步映射。前端据此在平铺视图里判断该行是否只读。

**OAuth 订阅端点不扩创建参数**：`/endpoint/oauth/exchange` 建成后默认 private，admin 需要公开时用 PATCH 切换（更新路径已支持），避免再给授权流程加一个字段。

## 前端

- `web/src/lib/types.ts`：新增 `EndpointVisibility = "private" | "public"`；`UpstreamEndpointItem.visibility`；Create/Update body 的 `visibility?`。
- `web/src/app/(dashboard)/upstream/endpoint-form.ts` + `endpoint-dialog.tsx`：
  - admin 才渲染「公开 / 私有」选择（Radio/Select，默认私有）。
  - admin 代建（选择了 `ownerUserID` 为他人）时禁用并重置为私有，避免出现「归属他人的公开端点」。
  - 编辑态可切换；OAuth 端点同样允许切换（visibility 不属于「连接字段不可变」范围）。
- `grouped-view.tsx` / `flat-view.tsx`：
  - 公开端点显示「公开」徽标（私有不显示徽标，避免每条都挂一个无信息量的标签）。
  - 非 admin 视角下，`visibility === "public"` 的端点与其模型不渲染编辑/删除/授权按钮。
- i18n：在 `web/src/lib/i18n` 对应语言文件**末尾追加** key（避免整文件排序 diff）。

## 缓存与失效

| 缓存 | 键是否变化 | 失效方式 |
|---|---|---|
| `cache:rd:alias:<userID>` / `cache:rd:detail:<userID>` | 否 | endpoint/model 写路径 `InvalidateAll` |
| 平铺模型列表 | 是（filter 序列化） | 同上 |
| 端点亲和 `affinity:*` | 否 | TTL 5 分钟；命中端点不在候选时忽略（已实现） |

公开化/私有化是一次 endpoint 写，走现有 `InvalidateAll`，无需新增失效逻辑。

## 测试策略

**单测**

- `test/unit/endpoint_resolver/`：自己命中时不用公开池（即使公开池有同名）；未命中时回退公开池；`sharedPoolFallback=false` 时公开池照常生效；公开模型来自多个 admin 时各自用自己 owner 的 scope 取 endpoint。
- `test/unit/llmproxy_repo_scope/`：`FindIDsByScope` 的 `includePublic` 三态；`PaginateWithFilter` 的公开并集 + 缓存键区分。
- `test/unit/endpoint_command/`：非 admin 传 `visibility=public` 被拒；admin 公开时代建 owner 被忽略。

> `EndpointRepository` 加参数、`ModelRepository` 加方法都会让所有测试 stub 编译失败（`var _ llmproxy.XxxRepository` 断言会立刻暴露）。需同步更新的已知位置：`test/unit/endpoint_command/`、`test/unit/endpoint_resolver/`、`test/unit/oauth_flow/`、`test/unit/llmproxy_repo_scope/`。

**E2E**（`test/e2e/public_endpoint/`，需要 `BASE_URL` / `ADMIN_TOKEN` / `USER_TOKEN`）

1. admin 建公开端点 + 模型；user 在 `upstream/list` 看到且 `visibility=public`。
2. user 对公开端点 PATCH / DELETE、对公开模型 PATCH / DELETE / 禁用全部失败。
3. user 自建同名 alias 后，`/v1/models` 里该 alias 仍只出现一次（按 alias 去重），模型调用必须命中自己的端点。
4. user 删掉自己的模型后，同一调用回退到公开端点成功。
5. admin 把端点改回 private 后，user 的列表不再出现、调用返回模型不存在。
6. cleanup：删除临时端点/模型。

## 已知限制

- 公开端点共享 admin 的上游密钥与额度；一个用户高频调用会耗尽所有人的配额。当前不做用户级配额，依赖现有 Token/Request 限流（按调用者 API Key 维度）。
- 多个 admin 公开同名 alias 时，候选是这些公开模型的合集，按 `priority`/`weight` 调度，调用可能落到任一 admin 的端点。
- **删除用户不会级联删除其端点**（identity 既有行为，只软删 user 与 API Key）。因此「admin 降级后被删除」会让其公开端点继续对全员开放并使用已删用户的凭据；运维需先把端点改回私有或删除。若后续要收敛，应在 user 删除流程里把其公开端点置 private 或级联清理。
- demo 账户与普通用户同口径：能看到公开端点（只读），仍然不能调用 LLM。
- 公开模型解析 `FindPublicByAlias` 没有 `user_id` 前导列，走不上现有复合索引 `(user_id, alias, endpoint_id, deleted_at)`，会按 alias 扫描 `models` 表。该表是配置数据（万级以下），当前可接受；若增长到十万级，补 `(alias, enabled)` 索引（`// ponytail: 配置表先不建索引，行数上万再评估`）。

## 实现期项目硬约束

- 分支在 `.worktrees/` 下创建；Go 改动前按 `docs/agents/workflow.md` 加载 skill 清单并先跑 `use-modern-go list`。
- 代码搜索用 CodeGraph；跨文件符号重构用 Serena；提交前写 Serena memory。
- 新增 DTO 字段遵循 `huma-dto-conventions`；新增常量进 `internal/common/constant`（业务包禁 local const）。
- **GORM 的 OR 必须显式分组**：`db.Where("user_id = ? OR visibility = ?", ...)` 再叠加 `.Where("deleted_at = 0")`
  会被拼成 `deleted_at = 0 AND user_id = ? OR visibility = 'public'`，优先级错误会把已删除的公开端点放进来。
  统一写成 `db.Where(db.Where(userCond).Or(publicCond))`，让 GORM 生成 `(A OR B) AND C`。
- `// ponytail:` 标记本设计中的刻意简化（不建索引、不做模型级 visibility）。
