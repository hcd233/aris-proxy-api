# Audit 列表页（Web 端）设计文档

- **作者**：centonhuang（与 AI 结对）
- **日期**：2026-05-29
- **状态**：草案，待评审

## 1. 背景与目标

后端在 2026-05-11 已落地审计列表接口 `GET /api/v1/audit/logs`，但当时该接口被设计为 `apiKeyAuth` 鉴权，且 handler 强制按当前 API Key 过滤——本质是 "API Key 自查自己的调用记录"。

需求方希望 Web 端登录用户（特别是 admin）能在控制台查看审计记录。直接复用现接口存在两个硬性阻塞：

1. Web 端登录后只持有 JWT，没有 API Key，无法通过 `APIKeyMiddleware`。
2. handler 写死 `WHERE api_key_id = ctx.apiKeyID`，admin 看不到全量。

为不引入第二条 URL，本设计**改造现有 `/api/v1/audit/logs` 接口**：鉴权切到 `jwtAuth`，数据范围按用户权限分级，前端新增一个表格页面消费它。

## 2. 范围

**包含**：

- 后端：改造路由鉴权、扩展 query/repository 支持按 user 维度过滤、DTO 增加用户与 API Key 关联字段。
- 前端：在 `(dashboard)` 下新增 `/audit` 页面与 sidebar 入口、扩展 `api-client` 与 `types`。
- 测试：单元测试覆盖 query 层新分支；E2E 覆盖普通 user 和 admin 两种数据范围。

**不包含**：

- 不新增第二条 URL。
- 不实现按用户/按 API Key 的筛选下拉（YAGNI，列表展示了关联信息已能满足查找需求；筛选留到下一版）。
- 不实现 TraceID 跳转 CLS（仅复制；CLS URL 模板易过期，admin 自行去 CLS 查更稳）。
- 不实现导出 CSV / 高级排序（用现有 `sortField` query 参数即可，前端第一版仅默认按时间倒序）。
- **删除旧的 `ListByAPIKeyID` 路径**（包含 query handler `ListAuditLogsHandler` 与 repository 方法 `ListByAPIKeyID`、相关 DTO 字段 `APIKeyID`），仓库内已确认无任何调用方。

## 3. 用户流程

```
[Admin 登录 web] → sidebar 出现 "Audit" 入口 → 点击进入 /audit
  → 默认显示最近 24h 全量审计
  → 可改时间范围（最近 1h / 24h / 7d / 自定义）
  → 可在搜索框按 traceID 或 model 模糊搜
  → 表格分页浏览
  → 点击某行 traceID 单元格 → 复制完整 traceID 到剪贴板（用于 CLS 排查）

[普通 user 登录 web] → sidebar 同样显示 "Audit"
  → 进入后只看到自己名下所有 API Key 的审计记录（先查 user 的 key id 列表，再用 IN 查询过滤 audit）

[Pending user] → permission-guard 已经拦在外面
```

## 4. 后端改造

### 4.1 路由（`internal/router/audit.go`）

```go
func initAuditRouter(auditGroup huma.API, auditHandler handler.AuditHandler, db *gorm.DB, cache *redis.Client) {
    auditGroup.UseMiddleware(middleware.JwtMiddleware(db, cache))

    huma.Register(auditGroup, huma.Operation{
        OperationID: "listAuditLogs",
        Method:      http.MethodGet,
        Path:        "/logs",
        Summary:     "ListAuditLogs",
        Description: "Paginate audit logs scoped by current JWT user. Admin sees all records; regular user sees records under their own API keys.",
        Tags:        []string{"Audit"},
        Security:    []map[string][]string{{"jwtAuth": {}}},
        Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listAuditLogs", enum.PermissionUser)},
    }, auditHandler.HandleListAuditLogs)
}
```

注意点：

- `router.go` 调用 `initAuditRouter` 处需把 `deps.Cache` 透传进来（其它 JWT 路由都这样）。
- `LimitUserPermissionMiddleware("listAuditLogs", enum.PermissionUser)` 拦住 `pending`，`user` 和 `admin` 都能进，分级在 handler 内完成。

**这是破坏性变更**：原 API Key 调用方将 401。当前仓库内（前端 + E2E + 文档）无任何代码以 API Key 调用此接口，可直接切换。在 commit message 与 release notes 注明即可，无须保留兼容路径。

### 4.2 DTO（`internal/dto/audit.go`）

`ListAuditLogsReq` 保持不变（已含 `Page/PageSize/Query/Sort/SortField/StartTime/EndTime`）。

`AuditLogItem` 增加三个关联字段：

```go
type AuditLogItem struct {
    // ... 既有字段保持不变 ...
    APIKeyName string `json:"apiKeyName" doc:"调用所用的 API Key 名称"`
    UserName   string `json:"userName" doc:"调用方用户名"`
    UserEmail  string `json:"userEmail" doc:"调用方邮箱"`
}
```

> 三字段对所有调用者都填充（普通 user 看到的就是自己/自己 key），不做按权限隐藏。这避免了"DTO 字段语义随调用者改变"的耦合。

### 4.3 Query Handler（`internal/application/audit/query/list_audit_logs.go`）

**Query 不感知权限**——分发由接口层（handler）完成。本次同时**删除既有的 `ListAuditLogsHandler` / `ListAuditLogsQuery`**（按 APIKey 维度的查询），仓库内已无调用方。新增两个 query handler：

```go
// 新增：admin 看全量
type ListAllAuditLogsQuery struct {
    Page      int
    PageSize  int
    Query     string
    Sort      enum.Sort
    SortField string
    StartTime time.Time
    EndTime   time.Time
}
type ListAllAuditLogsHandler interface {
    Handle(ctx context.Context, q ListAllAuditLogsQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}

// 新增：普通 user 看自己名下所有 key 的审计
// 实现内部：先查 user 名下 keyIDs（依赖 ProxyAPIKeyDAO），再调 repo.ListByAPIKeyIDs；
// handler 调用方只关心"按 user 列表"语义，不感知 keyIDs。
type ListAuditLogsByUserQuery struct {
    UserID    uint
    Page      int
    PageSize  int
    Query     string
    Sort      enum.Sort
    SortField string
    StartTime time.Time
    EndTime   time.Time
}
type ListAuditLogsByUserHandler interface {
    Handle(ctx context.Context, q ListAuditLogsByUserQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}
```

参数清洗（page/pageSize/sort/sortField 默认值与上下界）抽到一个未导出的 helper 复用，避免在两个 handler 里重复。

### 4.4 Repository（`internal/infrastructure/repository/audit_repository.go`）

接口替换：**删除既有的 `ListByAPIKeyID` 方法**，新增两个：

```go
type AuditRepository interface {
    Save(ctx context.Context, audit *aggregate.ModelCallAudit) error

    // 新增：按 api_key_id 集合过滤；空集合 → 直接返回空结果（不打 SQL）
    ListByAPIKeyIDs(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)

    // 新增：全量
    ListAll(ctx context.Context, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}
```

**核心约束（按 review 反馈）：repository 内部不使用 JOIN，全部走多次 SQL。**

- `ListAll`：在被删除的 `ListByAPIKeyID` 实现基础上去掉 `WHERE api_key_id = ?`，其余分页/排序/时间范围/`Query` 模糊搜索逻辑不变，照样走 `dao.Paginate`。
- `ListByAPIKeyIDs`：用 `WHERE api_key_id IN (?)` 替代等值过滤，仍走 `dao.Paginate`。`apiKeyIDs` 为空时**不打 SQL**，直接返回 `(nil, &model.PageInfo{Page: param.Page, PageSize: param.PageSize, Total: 0}, nil)`，避免 `IN ()` 的方言坑。
- 两个方法都不携带关联信息——关联信息属于"展示视图"，由 handler 层组装。

### 4.5 Handler 分发与三步查询（`internal/handler/audit.go`）

**分发逻辑下沉到 handler**（按 review 反馈），分发完再做关联信息查询：

```go
func (h *auditHandler) HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error) {
    rsp := &dto.ListAuditLogsRsp{}
    userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
    permission := readPermission(ctx) // helper：从 ctx 读 enum.Permission，未知值返回空串

    var (
        audits   []*aggregate.ModelCallAudit
        pageInfo *model.PageInfo
        err      error
    )

    // step 0：分发
    switch permission {
    case enum.PermissionAdmin:
        audits, pageInfo, err = h.listAll.Handle(ctx, auditquery.ListAllAuditLogsQuery{ /* 透传 req */ })
    case enum.PermissionUser:
        // ListAuditLogsByUserHandler 内部完成两步：
        //   (a) 用 ProxyAPIKeyDAO 查 user 名下所有 keyIDs（独立 SQL）
        //   (b) 调 repo.ListByAPIKeyIDs(keyIDs, ...) 拿审计分页
        // keyIDs 为空时 (b) 不打 SQL 直接返回空。
        audits, pageInfo, err = h.listByUser.Handle(ctx, auditquery.ListAuditLogsByUserQuery{
            UserID: userID, /* 透传 req */
        })
    default:
        rsp.Error = ierr.ErrUnauthorized.BizError()
        return apiutil.WrapHTTPResponse(rsp, nil)
    }

    if err != nil { /* ierr.ToBizError */ }

    // step 1：从 audits 收集 api_key_id 集合
    apiKeyIDs := lo.Uniq(lo.Map(audits, func(a *aggregate.ModelCallAudit, _ int) uint { return a.APIKeyID() }))

    // step 2：批查 proxy_api_key（独立 SQL，IN 查询）
    keys, err := h.apiKeyDAO.ListByIDs(db, apiKeyIDs, []string{"id", "name", "user_id"})
    keyByID := lo.SliceToMap(keys, func(k *dbmodel.ProxyAPIKey) (uint, *dbmodel.ProxyAPIKey) { return k.ID, k })

    // step 3：批查 user（独立 SQL，IN 查询）
    userIDs := lo.Uniq(lo.Map(keys, func(k *dbmodel.ProxyAPIKey, _ int) uint { return k.UserID }))
    users, err := h.userDAO.ListByIDs(db, userIDs, []string{"id", "name", "email"})
    userByID := lo.SliceToMap(users, func(u *dbmodel.User) (uint, *dbmodel.User) { return u.ID, u })

    // step 4：组装 DTO
    rsp.Logs = lo.Map(audits, func(a *aggregate.ModelCallAudit, _ int) *dto.AuditLogItem {
        item := &dto.AuditLogItem{ /* 现有字段映射保持不变 */ }
        if k, ok := keyByID[a.APIKeyID()]; ok {
            item.APIKeyName = k.Name
            if u, ok := userByID[k.UserID]; ok {
                item.UserName = u.Name
                item.UserEmail = u.Email
            }
        }
        return item
    })
    rsp.PageInfo = pageInfo
    return apiutil.WrapHTTPResponse(rsp, nil)
}
```

**SQL 计数（关键）**：

| 角色 | SQL 总数（每次请求） |
|---|---|
| admin | 1 (audit 分页) + 1 (audit count) + 1 (apiKey IN) + 1 (user IN) = **4** |
| user | 1 (查自己 key id) + 1 (audit 分页) + 1 (audit count) + 1 (apiKey IN) + 1 (user IN) = **5** |

> 都在 LAN 单数字毫秒量级，相比 JOIN 多 2~3 次 round-trip，但换来：repo 与 aggregate 边界清晰、user 端"先查 keyIDs 再 IN"的两次小查询都吃 `proxy_api_key.user_id` 与主键索引、admin 端 audit 分页只过单表索引而非 JOIN。

**handler 依赖增加**：`AuditDependencies` 增加 `ListAll ListAllAuditLogsHandler`、`ListByUser ListAuditLogsByUserHandler`、`ProxyAPIKeyDAO`、`UserDAO`、`*gorm.DB`（用于 DAO 调用）。

**DAO 准备**：

- 复用现有 `proxyAPIKeyDAO.List` / `userDAO.List`（如已支持 `WHERE id IN (?)`）；如不支持，新增轻量方法 `ListByIDs(db, ids, fields)`。具体在 plan 阶段确认现有 DAO 接口形态再决定。
- 若 `apiKeyDAO` 没有现成的"按 user_id 查 ID 列表"方法，新增 `ListIDsByUserID(db, userID) ([]uint, error)` 即可。

### 4.6 路由 / 中间件 / Bootstrap

- `internal/router/audit.go`：见 §4.1，鉴权改 `JwtMiddleware` + `LimitUserPermissionMiddleware("listAuditLogs", enum.PermissionUser)`，`router.go` 调用处把 `deps.Cache` 透传进来。
- `internal/bootstrap/container.go`：
  - 删除既有 `auditquery.NewListAuditLogsHandler` 的 provider 注册。
  - 注册 `ListAllAuditLogsHandler`、`ListAuditLogsByUserHandler` 的 provider。
  - `AuditDependencies` 字段替换：去掉 `List auditquery.ListAuditLogsHandler`，增加 `ListAll auditquery.ListAllAuditLogsHandler`、`ListByUser auditquery.ListAuditLogsByUserHandler`、`ProxyAPIKeyDAO`、`UserDAO`、`*gorm.DB`。

### 4.7 数据库与索引

- 不新增表、不新增字段。
- `proxy_api_key.user_id` 已有 uniqueIndex（`idx_user_id_name_deleted` 第一列），`SELECT id FROM proxy_api_key WHERE user_id = ?` 走索引；`model_call_audit.api_key_id` 已有 `idx_api_key_id_created_at`，`WHERE api_key_id IN (?)` 走索引。
- `ListAll` 退化为 `WHERE created_at BETWEEN ? AND ? ORDER BY created_at DESC LIMIT ? OFFSET ?`。当前 `model_call_audit` 上 `created_at` 没有独立索引，只有 `idx_api_key_id_created_at` 等复合索引。**第一版不加新索引**（业务量未到瓶颈），但在设计文档中标记后续观察项：admin 全量分页慢时再加 `idx_created_at`。
- `ListAll` 退化为 `WHERE created_at BETWEEN ? AND ? ORDER BY created_at DESC LIMIT ? OFFSET ?`。当前 `model_call_audit` 上 `created_at` 没有独立索引，只有 `idx_api_key_id_created_at` 等复合索引。**第一版不加新索引**（业务量未到瓶颈），但在设计文档中标记后续观察项：admin 全量分页慢时再加 `idx_created_at`。

## 5. 前端实现

### 5.1 路由与 sidebar

- 新建 `web/src/app/(dashboard)/audit/page.tsx`。
- `web/src/app/(dashboard)/layout.tsx` 的 `navItems` 增加：

```ts
{
  label: "Audit",
  href: "/audit/",
  icon: <ScrollText className="size-4" />,
  // 注意：不加 adminOnly。普通 user 也能进，看到的是自己的数据。
},
```

### 5.2 类型与 API client

`web/src/lib/types.ts` 增加：

```ts
export interface AuditLogItem {
  id: number;
  createdAt: string;
  model: string;
  upstreamProvider: string;
  apiProvider: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationInputTokens: number;
  cacheReadInputTokens: number;
  firstTokenLatencyMs: number;
  streamDurationMs: number;
  userAgent: string;
  upstreamStatusCode: number;
  errorMessage: string;
  traceId: string;
  apiKeyName: string;
  userName: string;
  userEmail: string;
}

export interface ListAuditLogsRsp extends CommonRsp {
  logs?: AuditLogItem[];
  pageInfo?: PageInfo;
}
```

`web/src/lib/api-client.ts` 新增：

```ts
async listAuditLogs(params: {
  page: number;
  pageSize: number;
  query?: string;
  startTime?: string; // ISO8601
  endTime?: string;
}): Promise<ListAuditLogsRsp> {
  const sp = new URLSearchParams({ page: String(params.page), pageSize: String(params.pageSize) });
  if (params.query) sp.set("query", params.query);
  if (params.startTime) sp.set("startTime", params.startTime);
  if (params.endTime) sp.set("endTime", params.endTime);
  return this.request<ListAuditLogsRsp>(`/api/v1/audit/logs?${sp}`);
}
```

### 5.3 页面（`audit/page.tsx`）

骨架照搬 `sessions/page.tsx`（已经是项目里最完整的列表页范例）：

- `useIsMobile` 决定 Table（桌面）vs Card（移动）。
- 使用 `Table/TableHeader/TableBody/TableRow/TableCell` 组件。
- 分页栏 + 每页大小下拉，与 sessions 一致。
- 状态：`logs`、`pageInfo`、`loading`、`searchQuery`、`timeRange`、`pageInputValue`。

**列定义**（桌面）：

| 列 | 内容 | 备注 |
|---|---|---|
| Time | `createdAt` 本地化 | 包含日期+时分秒，方便定位 |
| Model | `model` | 模型别名 |
| User | `userName / userEmail` | 单元格内两行 |
| API Key | `apiKeyName` | |
| Status | `upstreamStatusCode` | 200 显示 success badge；非 200 显示 destructive badge + tooltip 显示 errorMessage |
| Tokens | `inputTokens` / `outputTokens` | 紧凑展示，例如 `1.2k / 800` |
| Latency | `firstTokenLatencyMs ms` | 流式有 stream duration 时再加一行 |
| TraceID | `traceId.slice(-6)` | 单元格点击 → `navigator.clipboard.writeText(full)` + toast |

**筛选区**（在 Card header 下方，搜索框上方）：

- 时间范围下拉：`Last 1 hour / Last 24 hours / Last 7 days / Custom`。  
  - 前三个选项：前端立即计算 `startTime/endTime` 的 ISO 字符串。
  - Custom：弹出两个 datetime-local input。
  - 默认 `Last 24 hours`，避免首次进入加载全量。
- 搜索框：值受控，回车触发刷新；后端 `query` 参数已支持 traceID / model 模糊。

**移动端**：单卡片渲染，至少展示 Time / Model / User / Status / TraceID（后 6 位）；其它字段折叠展示。

### 5.4 与现有页面的复用

- 不新增 ui 组件；表格、徽章、骨架屏、下拉、分页全部复用。
- 错误提示统一走 `sonner`。

## 6. 错误处理

- 后端：`ierr.ToBizError(err, ierr.ErrInternal.BizError())` 既有模式；分页 / IN 查询失败统一 5xx，时间范围非法走 huma validation 自动返回 422。
- 前端：列表加载失败 → `toast.error(rsp.error.message ?? "Failed to load audit logs")`；剪贴板复制失败 → `toast.error`。

## 7. 测试策略

### 7.1 单元测试（`test/unit/audit/`）

- `list_all_audit_logs_test.go`：构造 fake repo，断言 `ListAllAuditLogsHandler` 把 `param.Page/PageSize/Sort/SortField/Query/StartTime/EndTime` 完整透传到 `repo.ListAll`，且对默认值/上下界做了夹紧（page<1→1、pageSize 上限 100、sortField 缺省→`created_at`、sort 缺省→`desc`）。
- `list_audit_logs_by_user_test.go`：同上，断言 `ListAuditLogsByUserHandler` 透传 `userID`、内部先调 `apiKeyDAO.ListIDsByUserID` 再调 `repo.ListByAPIKeyIDs`、空 keyIDs 时直接返回空结果不调 repo（通过 fake DAO/repo 的调用计数验证）。
- 排序字段非法 → `ErrValidation` 的用例两个 handler 各自覆盖（共享 helper 测试）。
- 用 fake 实现（手写满足接口的最小实现）验证调用分发；不引入 mock 框架。
- **删除既有的** `test/unit/audit/list_audit_logs_*` 中针对旧 `ListAuditLogsHandler` 的用例（如有），避免维护无意义的死测试。
- handler 层（`internal/handler/audit.go`）的"分发 + 三步关联查询"分支不写单测——它的逻辑全是 IO 编排，没有可独立断言的纯逻辑；行为正确性由 plan 阶段的手工冒烟（启动服务后用浏览器实际访问 `/audit` 页面）验证。

### 7.2 E2E

**不做**。理由（按 review 反馈）：
- 后端核心分发逻辑已被 §7.1 单元测试覆盖。
- 三步关联查询是确定性的 SQL 编排，runtime 错误几乎只可能是字段映射打错，本地手工冒烟一次即可发现。
- E2E 还需要给测试账号准备 JWT，引入新的环境变量与运维流程，成本远大于收益。

> 若后续审计接口要扩展更多权限分支（按业务线、按租户等），再回头补 E2E。

### 7.3 验证清单

- `go test -v -count=1 ./test/unit/audit/...`
- `make lint`
- `make test`（全量回归）
- 启动本地 `server start` + `web dev`，用 admin / 普通 user 各登录一次，肉眼确认列表数据范围、关联字段、TraceID 复制行为符合预期。

## 8. 兼容性与迁移

- **破坏性**：旧 API Key 调用方将 401。已确认仓库内无调用方，release notes 明示。
- **数据库**：无 schema 变更。
- **配置**：无新增环境变量。
- **前端**：仅新增页面与导航项，不影响其它模块。

## 9. 风险与未决问题

| 风险 | 评估 | 处理 |
|---|---|---|
| `ListAll` 在大数据量下分页慢（无独立 created_at 索引） | 业务量未到瓶颈，测试库数据 < 1 万行不会感知 | 记入 follow-up，超阈值后再加 `idx_created_at` |
| handler 多次 SQL 的额外 RT（admin 4 次 / user 5 次） | DB 在 LAN，每次 < 5ms；最坏 25ms | 监控；若 P99 > 100ms 再优化（合并查询或加缓存） |
| 普通 user 看到自己审计后可能要求"按某个 key 看" | 第一版不做 | 明确写入未来扩展 |
| Permission 字段类型断言不稳 | jwt 中间件写入的是 `permission` 变量（具体类型待 plan 阶段验证） | 在 plan 阶段读源码确认；如为 string 则 handler 内 `enum.Permission(s)` 转换；如为 enum 直接断言 |
| 取消 E2E 后回归保护变弱 | 单测 + 手工冒烟覆盖核心路径 | 接受风险；future 重构时若需要再补 E2E |

## 10. 附：核心文件影响清单

**后端**

- 修改：`internal/router/audit.go`、`internal/router/router.go`（cache 传递）
- 修改：`internal/handler/audit.go`、`internal/dto/audit.go`
- 重写：`internal/application/audit/query/list_audit_logs.go`（**删除** `ListAuditLogsHandler` / `ListAuditLogsQuery`，新增两个 query handler 与共享参数清洗 helper）
- 修改：`internal/domain/modelcall/audit_repository.go`（**删除** `ListByAPIKeyID`，新增 `ListAll`、`ListByAPIKeyIDs`）
- 修改：`internal/infrastructure/repository/audit_repository.go`（**删除** `ListByAPIKeyID` 实现，新增两方法实现，多次 SQL 不 JOIN）
- 修改：`internal/bootstrap/container.go`（**删除**旧 query handler provider 与 `AuditDependencies.List`，注册新 query handler、注入 DAO 与 DB）
- 可能修改：`internal/infrastructure/database/dao/proxy_api_key.go`（如缺 `ListByIDs` / `ListIDsByUserID` 方法则补充）

**前端**

- 新增：`web/src/app/(dashboard)/audit/page.tsx`
- 修改：`web/src/app/(dashboard)/layout.tsx`（navItems）
- 修改：`web/src/lib/types.ts`、`web/src/lib/api-client.ts`

**测试**

- 新增：`test/unit/audit/list_all_audit_logs_test.go`
- 新增：`test/unit/audit/list_audit_logs_by_user_test.go`
- 删除：`test/unit/audit/` 下针对旧 `ListAuditLogsHandler` 的用例（plan 阶段先 `ls` 确认具体文件名再删）

---

**所有评审决策已锁定**：A1 + B2 + C1 + D2 + E1 + 多次 SQL 不 JOIN + 分发在 handler + 不做 E2E + 删除旧 ByAPIKey 路径。
