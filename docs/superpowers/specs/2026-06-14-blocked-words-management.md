# Blocked Words Management

## 概述

为 aris-proxy-api 增加敏感词黑名单功能：管理员可增删敏感词，LLM 代理请求内容（消息 + 推理内容）命中黑名单时返回 403 Forbidden，同时记录审计和命中计数。

## 命名约定

| 层级 | 命名 |
|------|------|
| 概念 | `blocked`（形容词作名词，意为"被屏蔽的词"） |
| API 路由组 | `/api/v1/block` |
| 数据库表 | `blocked_words`（GORM 模型名 `Blocked`，`TableName()` 返回 `blocked_words`） |
| 前端路径 | `/web/block/` |
| 侧边栏标签 | "Blocked" |

## 架构

```
┌──────────────────────────────────────────────────────────┐
│                     API Server                            │
│                                                          │
│  ┌─────────────┐   ┌──────────────────────────────┐      │
│  │ /api/v1/    │   │  /api/openai/v1              │      │
│  │ block       │   │  /api/anthropic/v1            │      │
│  │  (CRUD,     │   │                              │      │
│  │   JWT+Admin)│   │  APIKey → Usecase → Forward   │      │
│  └──────┬──────┘   │               │               │      │
│         │          │         ┌─────▼──────┐        │      │
│         │          │         │ Blocked    │        │      │
│         │          │         │ Service    │        │      │
│         │          │         │ .Check()   │        │      │
│         │          │         └─────┬──────┘        │      │
│         │          │               │               │      │
│  ┌──────▼──────────▼───────────────▼──────────────┐      │
│  │              DB + Redis                         │      │
│  │  blocked_words  │  blocked:hit:{id}            │      │
│  └───────────────────────┬─────────────────────────┘      │
│                          │                                │
│  ┌───────────────────────▼─────────────────────────┐      │
│  │  Cron (每 5min): Redis → DB 同步 hit_count      │      │
│  └─────────────────────────────────────────────────┘      │
└──────────────────────────────────────────────────────────┘
```

## 存储模型

### DB 表 `blocked_words`

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint (PK, auto_increment) | |
| word | string (unique, not null) | 敏感词内容 |
| hit_count | uint (default 0) | 命中次数（由 cron 从 Redis 同步） |
| created_at | datetime | |
| updated_at | datetime | |
| deleted_at | int64 (default 0) | 软删除 |

### Redis Key

```
blocked:hit:{word_id} → integer (INCRBY 原子递增)
```

注：使用 word_id 而非 word 本身，避免长 key 和编码问题。

## 匹配引擎

使用 **Aho-Corasick 多模式匹配算法**，保证 O(n) 时间复杂度的子串匹配。

### 生命周期

1. **初始化**：应用启动时从 `blocked_words` 表加载所有非删除词，构建 AC 自动机
2. **更新**：创建/删除敏感词后，重建自动机（`sync.RWMutex` 保护）
3. **使用**：`BlockedService.Check(text string) []uint` 返回所有命中的 word ID

### Check 的文本范围

对 LLM 代理请求的所有消息内容做子串匹配：
- OpenAI: `req.Body.Messages[].Content` 中的所有文本段（text content parts + text 类型消息），以及 `req.Body.Messages[].ReasoningContent`
- Anthropic: `req.Body.Messages[].Content` 中的所有文本段，以及 `req.Body.Messages[].ReasoningContent`

## 接口设计

### 管理 API（JWT + Admin）

| Method | Path | OperationID | 说明 |
|--------|------|-------------|------|
| POST | `/api/v1/block` | `createBlocked` | 创建敏感词（body: `{word: string}`） |
| GET | `/api/v1/block/list` | `listBlocked` | 列表（含 hit_count，支持分页 + 搜索） |
| DELETE | `/api/v1/block/{id}` | `deleteBlocked` | 删除 |

### 错误码

新增错误哨兵 `ErrContentBlocked`（业务码 10011，HTTP 403），用于 LLM 代理请求被拦截时返回。

## 审核与审计

### 管理操作审计

CRUD 操作的审计通过现有 `ModelCallAudit` 记录（使用 `AuditTask` 异步写入），`remark` 字段写 `"trigger blocked word"`。

### 请求拦截审计

当 LLM 代理请求因敏感词被拦截时，创建 `ModelCallAudit` 记录：
- `call_type`: `text_completion`（OpenAI）/ `message`（Anthropic）
- `blocked`: `true`
- `response_code`: `403`
- `response_body`: 包含命中词列表的 JSON
- `remark`: 包含 `[Blocked] <matched_words>`

## 前端

### 新增页面：`/web/block/`

- 侧边栏 "Blocked" 图标：`<Ban />`（lucide-react）
- `adminOnly: true`
- 页面结构与 endpoints 一致（表格展示 + Dialog 创建 + AlertDialog 删除）
- 表格列：ID, Word, Hit Count, Created At, Actions（Delete 按钮）
- 搜索功能：按 word 模糊搜索

### API Client 新增方法

```typescript
createBlocked(body: CreateBlockedReqBody): Promise<CommonRsp>
listBlocked(page: number, pageSize: number, query?: string): Promise<ListBlockedRsp>
deleteBlocked(id: number): Promise<CommonRsp>
```

### 类型定义

```typescript
interface CreateBlockedReqBody { word: string }
interface BlockedItem { id: number; word: string; hitCount: number; createdAt: string; }
interface ListBlockedRsp extends CommonRsp { blocked?: BlockedItem[]; pageInfo?: PageInfo; }
```

## 实现文件清单

### 新增文件

1. `internal/infrastructure/database/model/blocked.go` - DB model
2. `internal/infrastructure/database/dao/blocked.go` - DAO
3. `internal/domain/blocked/aggregate/blocked.go` - Domain aggregate
4. `internal/domain/blocked/repository.go` - Repository interface
5. `internal/application/blocked/port/handler.go` - Port types
6. `internal/application/blocked/command/create_blocked.go` - Create command
7. `internal/application/blocked/command/delete_blocked.go` - Delete command
8. `internal/application/blocked/query/list_blocked.go` - List query
9. `internal/application/blocked/service.go` - `BlockedService`（AC 自动机 + Check 方法）
10. `internal/infrastructure/repository/blocked_repository.go` - Repository impl
11. `internal/infrastructure/cache/blocked.go` - Redis hit counter
12. `internal/dto/blocked.go` - DTOs
13. `internal/handler/blocked.go` - Handler
14. `internal/router/blocked.go` - Router
15. `web/src/app/(dashboard)/block/page.tsx` - Frontend page

### 修改文件

1. `internal/infrastructure/database/model/base.go` - 注册 `Blocked` 模型
2. `internal/infrastructure/database/dao/singleton.go` - 注册 DAO 单例
3. `internal/common/ierr/sentinels.go` - 增加 `ErrContentBlocked`
4. `internal/common/constant/string.go` - 增加 tags/field 常量
5. `internal/common/constant/sql.go` - 增加 SQL 字段常量
6. `internal/application/llmproxy/usecase/port.go` - 增加 `BlockedService` 到 usecase port
7. `internal/application/llmproxy/usecase/openai.go` - 注入内容检查
8. `internal/application/llmproxy/usecase/anthropic.go` - 注入内容检查
9. `internal/router/router.go` - 注册 block 路由组
10. `internal/bootstrap/modules/repository.go` - 注册 repository
11. `internal/bootstrap/modules/application.go` - 注册 application
12. `internal/bootstrap/modules/handler.go` - 注册 handler
13. `internal/bootstrap/router.go` - 注册路由
14. `internal/bootstrap/container.go` - 注册 cron job
15. `internal/bootstrap/modules/cron.go` - 注册 blocked hit sync cron
16. `web/src/lib/types.ts` - 增加类型
17. `web/src/lib/api-client.ts` - 增加 API 方法
18. `web/src/app/(dashboard)/layout.tsx` - 增加 nav item
