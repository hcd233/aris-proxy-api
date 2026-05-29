# Session 详情接口性能优化设计

- 作者: centonhuang
- 创建: 2026-05-29
- 状态: 设计完成，待用户审阅

## 1. 背景与问题

### 1.1 当前实现

`GET /api/v1/session/?sessionId=X` 由 `sessionHandler.HandleGetSessionByUser` 经 `getSessionByUserHandler.Handle` 调 `sessionReadRepository.GetSessionDetail` 完成，单次请求开销分布如下：

| 步骤 | 操作 | 备注 |
|---|---|---|
| SQL #1 | `SELECT id, api_key_name, created_at, updated_at, message_ids, tool_ids, metadata FROM sessions WHERE id=?` | 轻量 |
| SQL #2 | `SELECT id, model, message, created_at FROM messages WHERE id IN (?,?,...)` | **重头**，`message` 是大 JSON 列 |
| SQL #3 | `SELECT id, tool, created_at FROM tools WHERE id IN (?,?,...)` | 中量，`tool` 是 JSON 列 |
| SQL #4 | `apiKeyRepo.LookupOwnerNamesByUserID(userID)` | 权限校验 |
| Redis #1 | `SMEMBERS session_shares:{id}` | 分享状态查询，容错 |
| Go 内存 | 按 `messageIds` 顺序重排投影 + sonic 序列化整包 | 输出体积与 session 长度成正比 |

### 1.2 问题清单

1. **响应体随会话长度无界增长**：单 session 没有 message/tool 数量上限，长会话可达几 MB。
2. **整包返回，无分页**：即使前端只渲染前 N 条也需要拉全量。
3. **`messages.message`/`tools.tool` 是大 JSON 列**：`IN (?,...)` 查询 + 反序列化是绝对耗时大头。
4. **重复请求开销不分摊**：同一个 session 反复打开都会重跑全量 SQL。
5. **DTO 把 `messages`/`tools` 与 `session metadata` 强耦合**：前端没法只刷新摘要数据。

### 1.3 关键事实（影响设计选型）

- **Session 永远是 Create，没有追加写**：`pool.SubmitMessageStoreTask` 每次 LLM 请求 = 1 行全新 `sessions`，从不读取已有 session 再追加 `message_ids`。来源：`internal/infrastructure/pool/store_pool.go:78`。
- **Message/Tool 完全不可变**：`MessageRepository`/`ToolRepository` 接口仅暴露 `BatchSaveDedup`/`FindByIDs`，没有 `Update`；cron 任务（summarize/score/dedup）都只读 message/tool。来源：`internal/domain/conversation/repository.go`、`internal/cron/session_dedup.go`。
- **唯一会改 session 行的路径**：`cron/session_dedup` 每小时跑一次，可能修改 `tool_ids` 或软删除冗余 session；`agent_pool.UpdateSummary`/`UpdateScore` 改 summary/score 字段（不影响详情接口透出的 `messageIds`/`toolIds`）。
- **Redis 基础设施已就绪**：`*redis.Client` 在 dig 容器里就绪，`cache.ShareCache` 是现成的引用模板。

## 2. 设计原则与决策

### 2.1 决策记录

| 决策 | 选项 | 选择 | 理由 |
|---|---|---|---|
| 首屏体验 | (a) metadata 接口顺带返回首页 / (b) 完全解耦 | **b** | 用户决定优先解耦，前端两次请求 |
| metadata 是否暴露 IDs 数组 | (a) 返回 `messageIds`/`toolIds` / (b) 完全不返回 / (c) 仅返回 `messageCount`/`toolCount` | **c** | 前端需要 total 做"共 X 条"和 hasMore 判断；IDs 数组对前端无用且会让响应变大；count 字段命名与现有 `SessionSummary` 保持一致 |
| 分页参数模型 | A 任意 IDs / **B offset+limit** / C cursor | **B** | session 内 message 顺序固定（`messageIds` 数组顺序），offset 不会跳号；权限校验最简（slice 切片即合法子集） |
| 缓存 TTL | session.meta / message / tool | **三者统一 1h** | 一致性窗口可控；用户决定 |
| 权限前置 SQL（之前方向 4） | 纳入 / 不纳入 | **不纳入** | 加缓存后 SQL #1 已大幅减负，权限前置的边际收益不抵复杂度 |
| 路径命名 | `/session/messages` / `/session/message/list` | **`/session/message/list`** | 与现有 `/session/share/list`、CODEBUDDY.md 11.2/11.3 节"资源单数 + `/list` 表示列表"约定一致 |
| 权限校验位置 | (a) 数据加载之后比对 / (b) 逻辑顺序前置 + 数据加载之后比对 / (c) SQL 前置（一次查询完成校验） | **b** | 顺序更清晰；不引入"权限前置 SQL"侵入缓存模型；越权请求仍要读 1 次缓存/DB（与决策"不纳入方向 4"一致） |
| 前端滚动加载方向 | A 向旧加载（IM 风格）/ **B 向下加载（阅读历史）** / C 页码切换 | **B** | session 详情是只读历史而非实时聊天；append 到尾部最简单；无需维护反向滚动锚点 |

### 2.2 设计原则（来自 CODEBUDDY.md 第 1 节 Karpathy 原则）

- **简约优先**：复用 `cache.ShareCache` 的代码组织模板（同包、同 redis client、同 dig provider 模式），不新建框架。
- **精准修改**：现有 `GET /session/` 接口**不动**（仍返回完整详情，向后兼容），新增两个接口承担优化路径。
- **目标驱动**：每一步都有可验证标准（见第 6 节）。
- **YAGNI**：不预先实现"取消 session"按钮、不做缓存预热、不做缓存 stampede 保护——session 详情读取频率不会高到需要这些。

## 3. 架构设计

### 3.1 接口契约（最终形态）

#### 3.1.1 新增接口 1：获取 session 元数据

```
GET /api/v1/session/metadata?sessionId={id}
认证: jwtAuth
权限: PermissionUser
```

**响应**：
```json
{
  "session": {
    "id": 123,
    "apiKeyName": "user-key-1",
    "createdAt": "2026-05-29T10:00:00Z",
    "updatedAt": "2026-05-29T10:01:00Z",
    "metadata": {"...": "..."},
    "messageCount": 156,
    "toolCount": 8,
    "shareID": ""
  }
}
```

**行为**：
- 走权限校验（`api_key_name ∈ user.apiKeyNames`）
- **不返回** `messages`/`tools` 内容，**也不返回** `messageIds`/`toolIds` 数组（IDs 仅缓存内部使用，分页接口走它）
- 暴露 `messageCount`/`toolCount`：前端用于显示总数、计算 `hasMore`、渲染滚动条
- 字段命名 `messageCount`/`toolCount` 与现有 `dto.SessionSummary` 保持一致，前端组件可复用
- 包含 `shareID`（沿用现有逻辑，容错）
- session 不存在或权限不足：与现有 `HandleGetSessionByUser` 一致——HTTP 200 + `rsp.Error` 装载 `ErrDataNotExists` / `ErrNoPermission` 的业务错误（`apiutil.WrapHTTPResponse` 模式），**不返回 4xx**

#### 3.1.2 新增接口 2：分页获取 messages

```
GET /api/v1/session/message/list?sessionId={id}&offset={n}&limit={m}
认证: jwtAuth
权限: PermissionUser
```

**入参约束**：
- `offset` 默认 0，`limit` 默认 20
- `offset >= 0`、`limit ∈ [1, 100]`；超出范围由 huma 校验直接拒绝（400），**不做服务端 clamp**，避免静默降级让前端误以为拿到了完整数据

**响应**：
```json
{
  "messages": [
    {"id": 1, "model": "gpt-4", "message": {...}, "createdAt": "..."},
    ...
  ],
  "pageInfo": {
    "offset": 0,
    "limit": 20,
    "total": 156
  }
}
```

注：`pageInfo` 不沿用 `model.PageInfo`（其字段是 `Page/PageSize/Total`），改用 `offset/limit/total`，更贴合滚动加载语义。

#### 3.1.3 新增接口 3：分页获取 tools

```
GET /api/v1/session/tool/list?sessionId={id}&offset={n}&limit={m}
```

结构对称于 messages 接口。

#### 3.1.4 现有 `GET /api/v1/session/` 处理

**保留不动**。仍然返回完整详情，作为向后兼容兜底。前端切换到新接口后可考虑移除，但本期不做。

### 3.2 缓存设计

#### 3.2.1 缓存 key 规约

新增到 `internal/common/constant/rediskey.go`：

```go
SessionMetaKeyTemplate    = "session:meta:%d"     // sessionID -> JSON of SessionMetaCacheRecord
MessageKeyTemplate        = "message:%d"          // messageID -> JSON of MessageCacheRecord
ToolKeyTemplate           = "tool:%d"             // toolID -> JSON of ToolCacheRecord
```

新增 TTL 常量到 `internal/common/constant/session.go`：

```go
SessionDetailCacheTTL = time.Hour
```

#### 3.2.2 缓存载荷

```go
// internal/infrastructure/cache/session_detail.go (新文件)

// SessionMetaCacheRecord 是 session 元数据的缓存载荷。
// 注意：MessageIDs/ToolIDs 是 cache 内部字段，不直接透出给 API 响应。
// metadata 接口只透出 messageCount/toolCount = len(MessageIDs)/len(ToolIDs)；
// message/tool 分页接口在内部读它们做 offset+limit 切片。
type SessionMetaCacheRecord struct {
    ID         uint              `json:"id"`
    APIKeyName string            `json:"apiKeyName"`
    CreatedAt  time.Time         `json:"createdAt"`
    UpdatedAt  time.Time         `json:"updatedAt"`
    Metadata   map[string]string `json:"metadata,omitempty"`
    MessageIDs []uint            `json:"messageIds"`  // 内部使用
    ToolIDs    []uint            `json:"toolIds"`     // 内部使用
}

type MessageCacheRecord struct {
    ID        uint               `json:"id"`
    Model     string             `json:"model"`
    Message   *vo.UnifiedMessage `json:"message"`
    CreatedAt time.Time          `json:"createdAt"`
}

type ToolCacheRecord struct {
    ID        uint            `json:"id"`
    Tool      *vo.UnifiedTool `json:"tool"`
    CreatedAt time.Time       `json:"createdAt"`
}
```

#### 3.2.3 SessionDetailCache 接口

```go
// internal/infrastructure/cache/session_detail.go

type SessionDetailCache interface {
    GetSessionMeta(ctx context.Context, sessionID uint) (*SessionMetaCacheRecord, error)
    SetSessionMeta(ctx context.Context, record *SessionMetaCacheRecord) error

    GetMessages(ctx context.Context, ids []uint) (hits map[uint]*MessageCacheRecord, missing []uint, err error)
    SetMessages(ctx context.Context, records []*MessageCacheRecord) error

    GetTools(ctx context.Context, ids []uint) (hits map[uint]*ToolCacheRecord, missing []uint, err error)
    SetTools(ctx context.Context, records []*ToolCacheRecord) error
}
```

**关键约定**：
- `Get*` 系列：cache miss 不算 error；error 只代表 Redis 通信故障，调用方应该 fallback 到 DB 而非中断请求（参考 `share.go` 的容错模式）。
- `SetMessages`/`SetTools` 用 Redis Pipeline 批量写入。
- `GetMessages`/`GetTools` 用 Redis MGET / Pipeline 批量读取。

### 3.3 读路径流程

#### 3.3.1 GET /session/metadata

```
1. 校验 req.SessionID > 0
2. 拿用户的 apiKeyNames（admin 跳过此步）：
   ├─ isAdmin → ownerNames = nil（标记为豁免）
   └─ 否则 LookupOwnerNamesByUserID(userID) → ownerNames
   说明：此步只准备校验所需的"用户身份范围"，不接触 session 数据
3. 缓存命中检查：GET session:meta:{id}
   ├─ 命中 → 跳到 6
   └─ 未命中 → 走 4
4. SQL: r.sessionDAO.Get(... SessionRepoFieldsReadDetail) 取 session 行
   ├─ 不存在 → 返回 ErrDataNotExists
5. 写缓存：SET session:meta:{id} EX 3600 (异步或同步均可，错误仅日志)
6. 权限比对：
   ├─ ownerNames == nil（admin） → 通过
   └─ 否则 detail.APIKeyName ∈ ownerNames → 通过；不通过返回 ErrNoPermission
7. 查 shareID（容错，与现有逻辑一致）
8. 装配响应 DTO：messageCount = len(meta.MessageIDs)，toolCount = len(meta.ToolIDs)；不透出 IDs 数组
```

**为什么 LookupOwnerNamesByUserID 在第 2 步而不是放最后？**
- 逻辑顺序更清晰："先看你是谁 → 再看数据是不是你的"
- 即使最坏情况（恶意构造越权请求），也会在 LookupOwnerNamesByUserID 阶段就锁定"用户身份"，而不是等到把 session 加载完才发现没权限
- 但**注意**：本设计**不做**"权限校验前置 SQL"（即 `WHERE api_key_name IN (?, ?) AND id = ?` 的方案），因此越权请求**仍会读一次缓存或 DB**——这是有意保留的权衡（避免侵入 SQL 模型，与第 2.1 节"不纳入权限前置 SQL"决策一致）

#### 3.3.2 GET /session/message/list

```
1. 校验 sessionID > 0、offset >= 0、limit ∈ [1, 100]
2. 走 3.3.1 的步骤 1-6（拿 ownerNames → 拿 meta → 权限比对），拿到合法的 SessionMetaCacheRecord
3. ids = meta.MessageIDs
   total = len(ids)
   page = ids[min(offset, total) : min(offset+limit, total)]
4. 缓存批量读：GetMessages(ctx, page)
   hits, missing = (...)
5. missing 不为空 → SQL 批量回源：messageDAO.BatchGetByField(WhereFieldID, missing, MessageRepoFieldsDetail)
   合并到 hits；并 SetMessages(ctx, fetched) 回填缓存
6. 按 page 顺序构造响应数组（跳过缺失 ID，复用现有 BuildOrderedMessageProjections 思路）
7. 返回 {messages, pageInfo: {offset, limit, total}}
```

#### 3.3.3 GET /session/tool/list

对称，复用 `SessionMetaCacheRecord.ToolIDs`。

### 3.4 写路径与缓存失效

| 触发点 | 失效动作 | 实现复杂度 |
|---|---|---|
| `pool.SubmitMessageStoreTask` 创建新 session | **不需要**——新 session ID 本来就不在缓存里 | 0 |
| `agent_pool.UpdateSummary`/`UpdateScore` | **不需要**——本接口不透出 summary/score | 0 |
| `cron/session_dedup` 修改 `tool_ids` | **不挂 hook，靠 1h TTL 自然过期**。dedup 每小时跑一次，缓存窗口 ≤ 1h，最多有 1 次返回旧 `tool_ids`；前端刷新即恢复。 | 0 |
| `cron/session_dedup` 软删除 session | 同上 | 0 |
| `sessionRepository.Delete`（如有 admin 删除路径） | 主动 DEL `session:meta:{id}`（本期暂不实现，因当前没有删除 session 的 API） | 0 |

**结论：本期不实现任何主动失效，全部依赖 1h TTL 兜底**。这是基于第 1.3 节关键事实推断出的最简方案。

### 3.5 依赖注入与代码组织

#### 3.5.1 文件清单

后端：
```
internal/infrastructure/cache/session_detail.go    新增 (SessionDetailCache 实现)
internal/dto/session.go                            新增 metadata / message-list / tool-list 的 Req/Rsp
internal/application/session/query/jwt_session_queries.go    新增 3 个 handler
internal/handler/session.go                        新增 3 个 handler 方法
internal/router/session.go                         新增 3 条路由注册（/metadata, /message/list, /tool/list）
internal/bootstrap/container.go                    新增 newSessionDetailCache provider + 3 个 query handler provider + 注入 handler.SessionDependencies
internal/common/constant/rediskey.go               新增 3 个 key 模板
internal/common/constant/session.go                新增 SessionDetailCacheTTL 常量
```

前端（详见 §3.7）：
```
web/src/lib/types.ts                                          新增/扩展类型
web/src/lib/api-client.ts                                     新增 3 个 API 方法
web/src/hooks/use-infinite-list.ts                            新增通用滚动加载 hook
web/src/components/session-detail/session-detail-client.tsx   主组件改造
```

#### 3.5.2 dig 注入模板（参考 `newShareCache`）

```go
// internal/bootstrap/container.go

func newSessionDetailCache(redisClient *redis.Client) cache.SessionDetailCache {
    return cache.NewSessionDetailCache(redisClient)
}
```

`SessionDependencies` 扩充：

```go
type SessionDependencies struct {
    ListByUser            sessionquery.ListSessionsByUserHandler
    GetByUser             sessionquery.GetSessionByUserHandler              // 现有，保留
    GetMetaByUser         sessionquery.GetSessionMetaByUserHandler          // 新增
    ListSessionMessages   sessionquery.ListSessionMessagesHandler           // 新增（对应 /message/list）
    ListSessionTools      sessionquery.ListSessionToolsHandler              // 新增（对应 /tool/list）
    ShareCache            cache.ShareCache
}
```

OperationID 命名（huma.Operation.OperationID）：
- `getSessionMetadata` → `GET /metadata`
- `listSessionMessages` → `GET /message/list`
- `listSessionTools` → `GET /tool/list`

### 3.6 错误处理与日志

- 沿用 `ierr` + `apiutil.WrapHTTPResponse` 模式（与现有 session handler 一致）。
- 缓存失败 → `logger.WithCtx(ctx).Warn("[SessionCache] xxx failed, fallback to DB", zap.Error(err))`，**不阻断请求**。
- 模块前缀：query 层 `[SessionQuery]`，cache 层 `[SessionDetailCache]`，handler 层沿用现有 `[SessionHandler]`。
- Mask：缓存层不打印 message/tool 内容（可能含敏感 prompt），只打印 ID/数量。

### 3.7 前端改造设计

#### 3.7.1 前端现状（事实）

- 详情页入口：`web/src/app/(dashboard)/sessions/detail/page.tsx`（thin wrapper，传 `sessionId` 给 client 组件）
- 详情主组件：`web/src/components/session-detail/session-detail-client.tsx`（867 行）当前一次性 `await api.getSession(id)` 拉取完整 `SessionDetail`，把 `messages`/`tools` 全量 `.map()` 渲染
- API client：`web/src/lib/api-client.ts` 是手写 `fetch` wrapper（非 OpenAPI 生成），含自动 401 → refresh → retry 流程
- 类型：`web/src/lib/types.ts` 全部手写，**与后端 DTO 同步维护**（无 codegen）
- 状态管理：**完全没有** SWR/react-query/Zustand；所有数据获取都是 `useState + useEffect + await api.xxx()`
- 唯一的 IntersectionObserver 在 `session-detail-client.tsx` 内服务于 sticky header 的 compact 状态切换，**不能直接复用**为加载触发器（语义不同）
- UI 库：shadcn/ui（@base-ui/react）+ Tailwind v4 + lucide-react

#### 3.7.2 改造目标

把"一次 `getSession` 拿全量"改成 **metadata + 滚动加载 + 缓存命中** 模式，且**保持现有 mobile/desktop 双布局视觉不变**。

#### 3.7.3 滚动语义（决策 B：向下加载）

- **首屏行为**：详情页打开 → 调 `/metadata` 拿 `messageCount`/`toolCount`/`apiKeyName`/`shareID` → 同时调 `/message/list?offset=0&limit=20` 拿首屏消息 → 可选并发调 `/tool/list?offset=0&limit=50` 拿工具列表（工具一般数量少，看 toolCount 是否 > 50 决定是否分页）
- **后续行为**：用户向下滚动到列表底部 → IntersectionObserver 触发 sentinel → `loadMore()` 调下一页 → append 到 `messages` state 末尾
- **结束条件**：`loadedCount >= total` 时停止 sentinel 观察，显示原有 "end of conversation" 文案
- **滚动锚点**：因为是 append 到尾部，新加载内容**不会触发视图跳变**，无需保存/恢复 scrollTop

#### 3.7.4 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 是否引入 SWR/react-query | 不引入 | 与现有页面风格一致；只是一个详情页改造，YAGNI |
| 是否抽 hook（如 `useInfiniteMessages`） | **抽** | 主组件已 867 行，再塞分页 state 会失控；hook 集中管理 `items / offset / total / loading / hasMore` |
| 滚动触发器实现 | IntersectionObserver sentinel `<div>` 放在列表底部 | 与现有 sticky header 的 IO 用法一致；阈值 `rootMargin: "200px"` 预加载 |
| 工具列表是否分页 | 视 `toolCount` 决定：≤ 50 一次拉完；> 50 也用 sentinel 分页 | 工具数量长尾分布，多数 session 工具少；阈值简单粗暴够用 |
| metadata 加载失败时的 UI | 与现有 `Session not found` 一致 | 不为新接口单独设计错误态 |
| messages/tools 部分加载失败 | 静默不阻断（沿用现有 `try/catch` 静默风格）+ 控制台 warn | 与现有 `fetchSession` 的 `catch {}` 模式一致 |

#### 3.7.5 文件清单

```
web/src/lib/types.ts                                       新增/修改类型
web/src/lib/api-client.ts                                  新增 3 个 API 方法
web/src/hooks/use-infinite-list.ts                         新增（通用滚动加载 hook）
web/src/components/session-detail/session-detail-client.tsx  改造主组件
```

不动的文件：
- `web/src/components/chat/chat-message.tsx`（消息渲染逻辑）
- `web/src/components/share/share-dialog.tsx`
- mobile/desktop 视觉布局（仅替换数据来源）

#### 3.7.6 类型层（`web/src/lib/types.ts`）

```ts
// 修改：SessionDetail 不再含 messages/tools，改用 count 字段
export interface SessionMetadata {
  id: number;
  apiKeyName: string;
  createdAt: string;
  updatedAt: string;
  metadata?: Record<string, string>;
  messageCount: number;
  toolCount: number;
  shareID?: string;
}

// 新增：metadata 接口响应
export interface GetSessionMetadataRsp extends CommonRsp {
  session?: SessionMetadata;
}

// 新增：分页 PageInfo（与现有 PageInfo 不同字段）
export interface OffsetPageInfo {
  offset: number;
  limit: number;
  total: number;
}

// 新增：消息分页响应
export interface ListSessionMessagesRsp extends CommonRsp {
  messages?: MessageItem[];
  pageInfo?: OffsetPageInfo;
}

// 新增：工具分页响应
export interface ListSessionToolsRsp extends CommonRsp {
  tools?: ToolItem[];
  pageInfo?: OffsetPageInfo;
}
```

**保留**：`SessionDetail`/`GetSessionRsp` 不动（旧接口仍可用，分享公开页 `web/src/app/share/page.tsx` 也仍调旧接口；不在本期改造）。

#### 3.7.7 API 客户端层（`web/src/lib/api-client.ts`）

在 `// ─── Session (JWT auth) ───` 块追加 3 个方法：

```ts
async getSessionMetadata(sessionId: number): Promise<GetSessionMetadataRsp> {
  return this.request<GetSessionMetadataRsp>(
    `/api/v1/session/metadata?sessionId=${sessionId}`
  );
}

async listSessionMessages(
  sessionId: number,
  offset: number = 0,
  limit: number = 20
): Promise<ListSessionMessagesRsp> {
  return this.request<ListSessionMessagesRsp>(
    `/api/v1/session/message/list?sessionId=${sessionId}&offset=${offset}&limit=${limit}`
  );
}

async listSessionTools(
  sessionId: number,
  offset: number = 0,
  limit: number = 50
): Promise<ListSessionToolsRsp> {
  return this.request<ListSessionToolsRsp>(
    `/api/v1/session/tool/list?sessionId=${sessionId}&offset=${offset}&limit=${limit}`
  );
}
```

#### 3.7.8 通用 hook（`web/src/hooks/use-infinite-list.ts`）

```ts
// 通用的"向下滚动加载更多"hook，复用于 messages / tools
export interface UseInfiniteListOptions<T> {
  fetcher: (offset: number, limit: number) => Promise<{ items: T[]; total: number }>;
  pageSize: number;
  enabled: boolean;  // sessionId 不合法时禁用
}

export interface UseInfiniteListResult<T> {
  items: T[];
  total: number;
  loading: boolean;
  hasMore: boolean;
  loadMore: () => Promise<void>;
  reset: () => void;
}

export function useInfiniteList<T>(opts: UseInfiniteListOptions<T>): UseInfiniteListResult<T>;
```

**约束**：
- 内部用 `useRef` 保护并发调用（一次只允许一个 in-flight loadMore）
- `reset()` 在 `sessionId` 变化时调用，清空 state 重新从 offset=0 拉
- 不做 SWR 风格的"stale-while-revalidate"，请求失败静默 + console.warn

#### 3.7.9 主组件改造（`session-detail-client.tsx`）

state 改造：
```ts
// 旧
const [session, setSession] = useState<SessionDetail | null>(null);

// 新
const [metadata, setMetadata] = useState<SessionMetadata | null>(null);
const messagesList = useInfiniteList<MessageItem>({
  fetcher: async (offset, limit) => {
    const rsp = await api.listSessionMessages(sessionId, offset, limit);
    return { items: rsp.messages ?? [], total: rsp.pageInfo?.total ?? 0 };
  },
  pageSize: 20,
  enabled: !!sessionId && !Number.isNaN(sessionId),
});
const toolsList = useInfiniteList<ToolItem>({
  fetcher: async (offset, limit) => {
    const rsp = await api.listSessionTools(sessionId, offset, limit);
    return { items: rsp.tools ?? [], total: rsp.pageInfo?.total ?? 0 };
  },
  pageSize: 50,
  enabled: !!sessionId && !Number.isNaN(sessionId),
});
```

数据流：
1. `useEffect` 调 `api.getSessionMetadata(sessionId)` → 设 `metadata`
2. metadata 拿到后才开始拉 messages/tools 首页（避免越权请求拉了 metadata 又徒劳拉 list）
3. 在 messages 列表底部加 `<div ref={loadMoreSentinelRef} />`，IntersectionObserver 监听 → 命中调 `messagesList.loadMore()`
4. 工具列表（mobile 抽屉 + desktop 侧边栏）同样加 sentinel；只在 `toolsList.hasMore` 时观察
5. `messageCount` 显示来自 `metadata.messageCount`（首屏即可显示总数，不依赖任何 list 加载完成）

加载状态：
- metadata 加载中：复用现有的 `loading` skeleton
- messages 列表加载中（loadMore）：列表底部显示一个细 skeleton 行 + 旋转图标，已有内容不消失
- tools 同理

**视觉/交互保持不变**：mobile sticky header、抽屉、share dialog、双布局结构全部沿用现有代码，只替换 `messages`/`tools` 数据来源 + 加 sentinel。

#### 3.7.10 不在本期前端范围

- `web/src/app/share/page.tsx`（公开分享页）继续走旧接口 `getShareContent` —— 它返回的是 `ShareContentSessionDetail`（不含 apiKeyName），后端没改过，无需联动
- 虚拟滚动（react-virtual / react-window）—— append 模式下消息数量超过几百条才会有 DOM 性能问题，`metadata.messageCount` 异常大的 session 是少数，留给后续按需引入
- 缓存层（前端的 SWR/react-query）—— 用户主动切换 session 才需要重新拉，不需要跨组件缓存
- 错误重试 UI —— 当前页面就是 `try/catch` 静默风格，本期保持一致；后续可统一改造

#### 3.7.11 前端验证标准

| 步骤 | 验证 |
|---|---|
| 类型 + API client | `npm run lint` 全绿，`tsc --noEmit` 通过 |
| `useInfiniteList` hook | 在 dev 模式下手动测试：sessionId 切换会 reset、loadMore 并发安全、hasMore 判断正确 |
| 详情页改造 | 手动测试：长 session（≥ 50 条）滚动到底部能加载下一页；mobile 抽屉里 tools 也能滚动加载；切换不同 session 列表正确重置；首屏 `messageCount` 立即显示 |
| 视觉回归 | 与改造前对比：mobile sticky header / desktop 双栏 / share dialog 触发 / "end of conversation" 文案 全部一致 |

不写前端单元测试（项目目前前端 0 测试，本期不引入测试基础设施）。


## 4. 测试策略

### 4.1 单元测试（`test/unit/`）

| 包 | 测试目标 | 关键用例 |
|---|---|---|
| `test/unit/session_detail_cache/` | `cache.SessionDetailCache` | `Set/Get` 往返、批量 partial hit、Redis 错误降级 |
| `test/unit/session_metadata_query/` | `getSessionMetaByUserHandler` | 缓存命中、缓存未命中回源、权限拒绝、404、count 正确等于 IDs 长度 |
| `test/unit/session_message_list_query/` | `listSessionMessagesHandler` | offset 越界、limit 边界、partial cache hit、空 messageIDs |
| `test/unit/session_tool_list_query/` | `listSessionToolsHandler` | 同上 |
| `test/unit/session_dto/` (扩) | 新 DTO 序列化 | metadata（含 count 字段）、page response、错误响应 |

测试规范严格遵循 CODEBUDDY.md 第 6 节：仅 `testing` 标准库、sonic、不内联大 JSON。

### 4.2 E2E 测试（`test/e2e/session_detail_perf/`）

新增 `session_detail_perf_test.go`，至少覆盖：

- `TestSessionDetailPerf_GetMetadata_Success`：metadata 接口返回正确字段（含 messageCount/toolCount），不含 messageIds/toolIds
- `TestSessionDetailPerf_GetMetadata_NoPermission`：他人 session → 业务错误 ErrNoPermission
- `TestSessionDetailPerf_ListMessages_Pagination`：offset+limit 正确切片，total 等于 metadata.messageCount
- `TestSessionDetailPerf_ListMessages_LimitRejected`：limit > 100 → 400 校验错误（不 clamp）
- `TestSessionDetailPerf_ListTools_Pagination`：同上
- `TestSessionDetailPerf_CacheConsistency`：两次连续请求结果一致（不直接断言缓存命中，但可观测响应一致性）

E2E 强约束：`BASE_URL`/`API_KEY` 缺失则 `t.Skip`，HTTP client 显式超时，不内联大 JSON。

### 4.3 性能验证（人工，非自动化）

记录在 PR 描述里：

- 老接口 `GET /session/?sessionId=X` vs 新接口 `GET /session/metadata` 响应体积对比（同一 session）。
- 新接口冷启动 vs 缓存命中下的耗时对比（curl 多次）。
- 长 session（≥ 50 messages）下 `/message/list` 接口的耗时分布。

## 5. 兼容性与回滚

- **向后兼容**：现有 `GET /api/v1/session/` 不改动，前端可分阶段切换。
- **回滚**：新接口与缓存层独立成模块，回滚只需 revert 这个分支；现有路径不受影响。
- **缓存层故障**：所有 `cache.SessionDetailCache` 操作失败都降级到 DB，请求不中断（与 share 模块一致的容错策略）。

## 6. 验证标准（目标驱动）

| 步骤 | 验证 |
|---|---|
| Step 1: 缓存层落地 | `test/unit/session_detail_cache/` 全绿，包括 partial hit、降级 |
| Step 2: query handler 落地 | `test/unit/session_metadata_query/`、`session_message_list_query/`、`session_tool_list_query/` 全绿 |
| Step 3: DTO + handler + router 接通 | `make lint` + `go test -count=1 ./...` 全绿 |
| Step 4: E2E 落地 | `BASE_URL`/`API_KEY` 设置后 `test/e2e/session_detail_perf/` 全绿 |
| Step 5: 部署后接口验证 | 用 `call-api` skill 跑 `/metadata`、`/message/list`、`/tool/list` 三个接口；任一失败按 `cls-log-bugfix` 流程查 traceId |
| Step 6: 前端类型 + API client | `web/` 下 `npm run lint` 全绿、`tsc --noEmit` 通过 |
| Step 7: 前端详情页改造 | dev 模式手动验证：长 session 滚动加载、首屏 messageCount 立即显示、切换 session 列表正确重置、mobile/desktop 视觉与改造前一致（参见 §3.7.11） |

## 7. 不在本期范围（YAGNI）

- 缓存预热（启动批量 warm session）
- 缓存 stampede 保护（lock + double-check）
- session 删除接口（不存在）
- message/tool 大字段裁剪/懒加载（如返回时只给 first N chars）
- schema 重构（`message_ids JSON 数组` → 外键关联）
- 权限校验前置 SQL（`WHERE api_key_name IN (...) AND id = ?`）

如果未来某个性能场景仍未达预期，再单独立项。
