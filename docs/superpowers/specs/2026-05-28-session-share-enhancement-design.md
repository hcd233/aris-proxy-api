# Session Share 增强设计文档

**日期**: 2026-05-28
**作者**: Sisyphus
**状态**: 待批准
**前置文档**: [2026-05-28-session-share-design.md](./2026-05-28-session-share-design.md)

---

## 1. 需求概述

在现有 Session Share 功能基础上，新增三项增强：

1. **Session Detail 感知分享状态**：前端在会话详情页需要知道当前会话是否已被分享
2. **重复分享校验**：已分享的会话（存在未过期分享链接）被重新创建分享时返回 DataExists 错误
3. **ShareID 超链接**：分享列表页的 ShareID 列改为超链接，可直接跳转到分享页

---

## 2. 需求 1：Session Detail 感知分享状态

### 2.1 设计

在 `GetSessionRsp` 的 `SessionDetail` 中新增 `isShared` 字段，后端通过 Redis `EXISTS session_shares:{sessionID}` 判断该 session 是否存在活跃分享。

### 2.2 后端改动

**`internal/dto/session.go`** — `SessionDetail` 新增字段：

```go
type SessionDetail struct {
    // ... 现有字段 ...
    IsShared bool `json:"isShared" doc:"是否已分享"`
}
```

**`internal/infrastructure/cache/share.go`** — `ShareCache` 接口新增方法：

```go
type ShareCache interface {
    // ... 现有方法 ...
    IsSessionShared(ctx context.Context, sessionID uint) (bool, error)
}
```

实现逻辑：

```go
func (s *shareCache) IsSessionShared(ctx context.Context, sessionID uint) (bool, error) {
    key := fmt.Sprintf(constant.SessionSharesKeyTemplate, sessionID)
    exists, err := s.cache.Exists(ctx, key).Result()
    if err != nil {
        return false, ierr.Wrap(ierr.ErrInternal, err, "failed to check session share status")
    }
    return exists > 0, nil
}
```

**`internal/handler/session.go`** — `SessionDependencies` 注入 `ShareCache`（当前 session handler 未注入），`HandleGetSessionByUser` 在构建 `SessionDetail` 时调用 `IsSessionShared`。

### 2.3 前端改动

**`web/src/lib/types.ts`** — `SessionDetail` 新增：

```typescript
export interface SessionDetail {
    // ... 现有字段 ...
    isShared?: boolean;
}
```

**`web/src/components/session-detail/session-detail-client.tsx`** — header 区域在 Share 按钮旁显示 "Shared" Badge：

- `isShared === true` 时显示 `<Badge variant="outline">Shared</Badge>`，与现有 API Key Name Badge 并列
- 未分享时不显示

---

## 3. 需求 2：重复分享校验

### 3.1 设计

新增 Redis 反向索引 `session_shares:{sessionID}`（Set 类型），存储该 session 的所有活跃 shareID。与 `share:{shareID}` 共享 TTL，在 CreateShare/DeleteShare 的 Pipeline 中同步维护。`CreateShare` 前先检查该 Set 是否存在，非空则返回 `ErrDataExists`。

### 3.2 后端改动

**`internal/common/constant/rediskey.go`** — 新增常量：

```go
SessionSharesKeyTemplate = "session_shares:%d"
```

**`internal/infrastructure/cache/share.go`** — 改动：

1. `CreateShare` 方法：Pipeline 中增加 `SADD session_shares:{sessionID} {shareID}` + `EXPIRE session_shares:{sessionID} 24h`；方法开头增加 `IsSessionShared` 检查，若已存在返回 `ierr.New(ierr.ErrDataExists, "session already has an active share")`
2. `DeleteShare` 方法：Pipeline 中增加 `SREM session_shares:{sessionID} {shareID}`；若 `SCARD session_shares:{sessionID}` 为 0 则 `DEL` 该 key（或依赖 TTL 自然过期）

### 3.3 错误响应

已存在活跃分享时，`CreateShare` 返回：

```json
{
  "shareId": "",
  "expiresAt": "0001-01-01T00:00:00Z",
  "error": {
    "code": 10004,
    "message": "DataExists"
  }
}
```

前端在 `ShareDialog` 的 `createShare` 回调中识别 error code 10004，显示 "This session is already shared" 提示。

---

## 4. 需求 3：ShareID 超链接

### 4.1 设计

纯前端 UI 改动，零后端改动。在 Shares 页面将 ShareID 列从纯文本改为超链接，点击后在新标签页打开分享页。

### 4.2 前端改动

**`web/src/app/(dashboard)/shares/page.tsx`** — ShareID 列改动：

将现有纯文本：
```tsx
<TableCell className="max-w-[220px] truncate font-mono text-xs text-muted-foreground">
  {share.shareId}
</TableCell>
```

改为超链接（与 Session 列样式一致）：
```tsx
<TableCell className="max-w-[220px] truncate font-mono text-xs">
  <a
    href={buildShareURL(share.shareId)}
    target="_blank"
    rel="noopener noreferrer"
    className="inline-flex items-center gap-1 text-primary hover:underline"
  >
    {share.shareId}
    <ExternalLink className="size-3" />
  </a>
</TableCell>
```

`buildShareURL` 已从 `@/components/share/share-dialog` 导入，`ExternalLink` 图标已导入。

---

## 5. 数据结构变更汇总

### 5.1 新增 Redis Key

| Key 格式 | 类型 | 说明 | TTL |
|----------|------|------|-----|
| `session_shares:{sessionID}` | Set | 存储该 session 的所有活跃 shareID | 与 share key 相同 (24h) |

### 5.2 Redis 操作变更

| 操作 | 变更 |
|------|------|
| `CreateShare` | Pipeline 增加 `SADD` + `EXPIRE`；前置 `EXISTS` 检查 |
| `DeleteShare` | Pipeline 增加 `SREM` |
| `IsSessionShared`（新增） | `EXISTS session_shares:{sessionID}` |

---

## 6. 改动文件清单

### 6.1 后端

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `internal/common/constant/rediskey.go` | 修改 | 新增 `SessionSharesKeyTemplate` |
| `internal/dto/session.go` | 修改 | `SessionDetail` 新增 `IsShared` 字段 |
| `internal/infrastructure/cache/share.go` | 修改 | 新增 `IsSessionShared` 方法；`CreateShare` 增加重复校验和反向索引维护；`DeleteShare` 增加反向索引清理 |
| `internal/handler/session.go` | 修改 | `SessionDependencies` 注入 `ShareCache`；`HandleGetSessionByUser` 填充 `IsShared` |
| `internal/bootstrap/container.go` | 修改 | `newSessionDependencies` 注入 `ShareCache` |
| `test/unit/session_share/session_share_test.go` | 修改 | 新增重复分享校验测试用例；mock 新增 `IsSessionShared` |

### 6.2 前端

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `web/src/lib/types.ts` | 修改 | `SessionDetail` 新增 `isShared` 字段 |
| `web/src/components/session-detail/session-detail-client.tsx` | 修改 | 显示 "Shared" Badge |
| `web/src/components/share/share-dialog.tsx` | 修改 | 识别 DataExists 错误码 (10004) 显示提示 |
| `web/src/app/(dashboard)/shares/page.tsx` | 修改 | ShareID 列改为超链接 |

---

## 7. 测试策略

### 7.1 新增单元测试用例

- `TestCreateShare_AlreadyShared`：已分享 session 再次创建返回 DataExists
- `TestCreateShare_ExpiredShareAllowsReshare`：过期分享后允许重新分享（mock `IsSessionShared` 返回 false）
- `TestIsSessionShared_True` / `TestIsSessionShared_False`
- `TestHandleGetSessionByUser_IsSharedField`：验证 `SessionDetail.IsShared` 正确填充

### 7.2 前端测试

手动验证：
1. Session detail 页面分享后显示 "Shared" Badge
2. 分享对话框重复分享时显示 "This session is already shared"
3. Shares 页面 ShareID 列可点击跳转
