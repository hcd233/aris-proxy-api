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
2. 缓存命中检查：GET session:meta:{id}
   ├─ 命中 → 跳到 5
   └─ 未命中 → 走 3
3. SQL: r.sessionDAO.Get(... SessionRepoFieldsReadDetail) 取 session 行
   ├─ 不存在 → 返回 ErrDataNotExists
4. 写缓存：SET session:meta:{id} EX 3600 (异步或同步均可，错误仅日志)
5. 权限校验：
   ├─ isAdmin → 通过
   └─ 否则 LookupOwnerNamesByUserID → in 比对 → 不通过返回 ErrNoPermission
6. 查 shareID（容错，与现有逻辑一致）
7. 装配响应 DTO：messageCount = len(meta.MessageIDs)，toolCount = len(meta.ToolIDs)；不透出 IDs 数组
```

#### 3.3.2 GET /session/message/list

```
1. 校验 sessionID > 0、offset >= 0、limit ∈ [1, 100]
2. 走 3.3.1 的步骤 1-5（缓存优先 + 权限校验），拿到 SessionMetaCacheRecord
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
| Step 5: 部署后线上验证 | 用 `call-api` skill 跑 `/metadata`、`/message/list`、`/tool/list` 三个接口；任一失败按 `cls-log-bugfix` 流程查 traceId |

## 7. 不在本期范围（YAGNI）

- 缓存预热（启动批量 warm session）
- 缓存 stampede 保护（lock + double-check）
- session 删除接口（不存在）
- message/tool 大字段裁剪/懒加载（如返回时只给 first N chars）
- schema 重构（`message_ids JSON 数组` → 外键关联）
- 权限校验前置 SQL（`WHERE api_key_name IN (...) AND id = ?`）

如果未来某个性能场景仍未达预期，再单独立项。
