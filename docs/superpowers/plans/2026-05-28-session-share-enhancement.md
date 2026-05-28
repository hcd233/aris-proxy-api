# Session Share Enhancement 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增强 Session Share 功能：session detail 感知分享状态、重复分享校验返回 DataExists、Shares 页面 ShareID 超链接

**Architecture:** 后端新增 Redis 反向索引 `session_shares:{sessionID}`（Set）实现 O(1) 重复校验和分享状态查询；`ShareCache` 接口新增 `IsSessionShared` 方法；`HandleGetSessionByUser` 填充 `IsShared` 字段；前端纯 UI 改动展示分享状态和超链接

**Tech Stack:** Go / Redis / Huma / React / Next.js / TypeScript

---

## Task 1: 新增 Redis 常量和 IsSessionShared 接口方法

**Files:**
- Modify: `internal/common/constant/rediskey.go`
- Modify: `internal/infrastructure/cache/share.go`

- [ ] **Step 1: 在 rediskey.go 中新增 SessionSharesKeyTemplate**

在 `internal/common/constant/rediskey.go` 的 const 块中，在 `UserSharesKeyTemplate` 之后新增：

```go
SessionSharesKeyTemplate = "session_shares:%d"
```

- [ ] **Step 2: 在 ShareCache 接口新增 IsSessionShared 方法**

在 `internal/infrastructure/cache/share.go` 的 `ShareCache` 接口中，在 `ListUserShares` 之后新增：

```go
IsSessionShared(ctx context.Context, sessionID uint) (bool, error)
```

- [ ] **Step 3: 实现 IsSessionShared 方法**

在 `internal/infrastructure/cache/share.go` 的 `shareCache` struct 上，在 `ListUserShares` 方法之后新增：

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

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 编译通过（此时 mock 还没更新，先跳过测试）

---

## Task 2: CreateShare 增加重复校验和反向索引维护

**Files:**
- Modify: `internal/infrastructure/cache/share.go`

- [ ] **Step 1: 修改 CreateShare 方法，增加重复校验和 SADD**

将 `internal/infrastructure/cache/share.go` 中的 `CreateShare` 方法替换为：

```go
func (s *shareCache) CreateShare(ctx context.Context, userID, sessionID uint) (string, time.Time, error) {
	if sessionID == 0 {
		return "", time.Time{}, ierr.New(ierr.ErrValidation, "sessionID must be greater than 0")
	}

	shared, checkErr := s.IsSessionShared(ctx, sessionID)
	if checkErr != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, checkErr, "failed to check existing share")
	}
	if shared {
		return "", time.Time{}, ierr.New(ierr.ErrDataExists, "session already has an active share")
	}

	now := time.Now()
	shareID := uuid.New().String()
	key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
	userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)
	sessionSharesKey := fmt.Sprintf(constant.SessionSharesKeyTemplate, sessionID)
	expiresAt := now.Add(constant.ShareTTL)

	record := &shareRecord{
		ShareID:   shareID,
		SessionID: sessionID,
		CreatedAt: now.Unix(),
	}
	recordJSON, err := sonic.Marshal(record)
	if err != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, err, "failed to marshal share record")
	}

	pipe := s.cache.Pipeline()
	pipe.Set(ctx, key, sessionID, constant.ShareTTL)
	pipe.ZAdd(ctx, userSharesKey, redis.Z{
		Score:  float64(record.CreatedAt),
		Member: string(recordJSON),
	})
	pipe.SAdd(ctx, sessionSharesKey, shareID)
	pipe.Expire(ctx, sessionSharesKey, constant.ShareTTL)

	if _, execErr := pipe.Exec(ctx); execErr != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, execErr, "failed to create share")
	}

	return shareID, expiresAt, nil
}
```

- [ ] **Step 2: 修改 DeleteShare 方法，增加 SREM**

在 `internal/infrastructure/cache/share.go` 的 `DeleteShare` 方法中，在找到 `targetMember` 之后、Pipeline 之前，增加 `sessionSharesKey` 变量，并在 Pipeline 中增加 `SRem`：

将 Pipeline 部分替换为：

```go
	sessionSharesKey := fmt.Sprintf(constant.SessionSharesKeyTemplate, record.SessionID)

	pipe := s.cache.Pipeline()
	pipe.Del(ctx, fmt.Sprintf(constant.ShareKeyTemplate, shareID))
	pipe.ZRem(ctx, userSharesKey, targetMember)
	pipe.SRem(ctx, sessionSharesKey, shareID)

	if _, execErr := pipe.Exec(ctx); execErr != nil {
		return ierr.Wrap(ierr.ErrInternal, execErr, "failed to delete share")
	}
	return nil
```

- [ ] **Step 3: 编译验证**

Run: `go build ./...`
Expected: 编译通过

---

## Task 3: SessionDetail DTO 新增 IsShared 字段

**Files:**
- Modify: `internal/dto/session.go`

- [ ] **Step 1: 在 SessionDetail struct 中新增 IsShared 字段**

在 `internal/dto/session.go` 的 `SessionDetail` struct 中，在 `Tools` 字段之后新增：

```go
IsShared  bool            `json:"isShared" doc:"是否已分享"`
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译通过

---

## Task 4: HandleGetSessionByUser 填充 IsShared 字段

**Files:**
- Modify: `internal/handler/session.go`

- [ ] **Step 1: 在 HandleGetSessionByUser 中调用 IsSessionShared 并填充 IsShared**

在 `internal/handler/session.go` 的 `HandleGetSessionByUser` 方法中，在构建 `rsp.Session` 赋值块之后、logger.Info 之前，新增 `IsShared` 查询和赋值：

将现有的 `rsp.Session = &dto.SessionDetail{...}` 块替换为：

```go
	isShared, sharedErr := h.shareCache.IsSessionShared(ctx, req.SessionID)
	if sharedErr != nil {
		logger.WithCtx(ctx).Warn("[SessionHandler] Check session shared status failed",
			zap.Uint("sessionID", req.SessionID), zap.Error(sharedErr))
	}

	rsp.Session = &dto.SessionDetail{
		ID:         view.ID,
		APIKeyName: view.APIKeyName,
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
		Metadata:   view.Metadata,
		Messages:   messageItems,
		Tools:      toolItems,
		IsShared:   isShared,
	}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译通过

---

## Task 5: 更新单元测试 mock 和新增测试用例

**Files:**
- Modify: `test/unit/session_share/session_share_test.go`

- [ ] **Step 1: 在 mockShareCache 中新增 IsSessionShared 方法和 sharedSessions 字段**

在 `test/unit/session_share/session_share_test.go` 的 `mockShareCache` struct 中新增字段：

```go
sharedSessions map[uint]bool
```

更新 `newMockShareCache` 函数初始化：

```go
func newMockShareCache() *mockShareCache {
	return &mockShareCache{
		shares:         make(map[string]*mockShareEntry),
		userShares:     make(map[uint][]string),
		sharedSessions: make(map[uint]bool),
	}
}
```

新增 `IsSessionShared` 方法：

```go
func (m *mockShareCache) IsSessionShared(_ context.Context, sessionID uint) (bool, error) {
	return m.sharedSessions[sessionID], nil
}
```

更新 `CreateShare` 方法，在创建成功后标记 sharedSessions：

在 `CreateShare` 方法的 `return` 之前新增：

```go
m.sharedSessions[sessionID] = true
```

更新 `DeleteShare` 方法，在删除成功后清除 sharedSessions（仅当该 session 无其他 share 时）：

在 `DeleteShare` 方法的 `delete(m.shares, shareID)` 之后新增：

```go
hasOtherShares := false
for _, id := range m.userShares[userID] {
	if id != shareID {
		entry := m.shares[id]
		if entry != nil && entry.sessionID == entry.sessionID {
			hasOtherShares = true
			break
		}
	}
}
if !hasOtherShares {
	delete(m.sharedSessions, entry.sessionID)
}
```

注意：以上逻辑简化为——遍历剩余 shares 检查是否还有同 sessionID 的 share。更简单的做法是直接检查删除后是否还有该 session 的 share：

```go
remainingForSession := false
for _, otherID := range m.userShares[userID] {
	if otherID == shareID {
		continue
	}
	if e, ok := m.shares[otherID]; ok && e.sessionID == entry.sessionID {
		remainingForSession = true
		break
	}
}
if !remainingForSession {
	delete(m.sharedSessions, entry.sessionID)
}
```

- [ ] **Step 2: 新增 TestCreateShare_AlreadyShared 测试用例**

在 `test/unit/session_share/session_share_test.go` 末尾（`containsJSONKey` 函数之前）新增：

```go
func TestCreateShare_AlreadyShared(t *testing.T) {
	sc := newMockShareCache()
	sc.sharedSessions[1] = true
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleCreateShare(ctx, &dto.CreateShareReq{Body: &dto.CreateShareReqBody{SessionID: 1}})
	if rsp.Body.Error == nil {
		t.Error("expected DataExists error for already-shared session")
	}
	if rsp.Body.Error.Code != ierr.ErrDataExists.BizError().Code {
		t.Errorf("error code = %d, want %d (DataExists)", rsp.Body.Error.Code, ierr.ErrDataExists.BizError().Code)
	}
}
```

- [ ] **Step 3: 新增 TestHandleGetSessionByUser_IsSharedField 测试用例**

```go
func TestHandleGetSessionByUser_IsSharedField(t *testing.T) {
	sc := newMockShareCache()
	sc.sharedSessions[1] = true
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleGetSessionByUser(ctx, &dto.GetSessionByUserReq{SessionID: 1})
	if rsp.Body.Session == nil {
		t.Fatal("expected non-nil session")
	}
	if !rsp.Body.Session.IsShared {
		t.Error("expected IsShared = true for shared session")
	}
}
```

```go
func TestHandleGetSessionByUser_NotShared(t *testing.T) {
	sc := newMockShareCache()
	getByUser := &mockGetSessionByUserHandler{view: map[uint]*sessionquery.SessionDetailView{1: testSessionView(1)}}
	h := newTestHandler(sc, getByUser)
	ctx := ctxWithUser(42, enum.PermissionUser)

	rsp, _ := h.HandleGetSessionByUser(ctx, &dto.GetSessionByUserReq{SessionID: 1})
	if rsp.Body.Session == nil {
		t.Fatal("expected non-nil session")
	}
	if rsp.Body.Session.IsShared {
		t.Error("expected IsShared = false for non-shared session")
	}
}
```

- [ ] **Step 4: 运行单元测试**

Run: `go test -v -count=1 ./test/unit/session_share/`
Expected: 全部 PASS

---

## Task 6: 运行 lint 和全量测试

**Files:** 无改动

- [ ] **Step 1: 运行 lint**

Run: `make lint`
Expected: 通过

- [ ] **Step 2: 运行全量测试**

Run: `go test -count=1 ./...`
Expected: 全部 PASS

---

## Task 7: 前端 — types.ts 新增 isShared 字段

**Files:**
- Modify: `web/src/lib/types.ts`

- [ ] **Step 1: 在 SessionDetail interface 中新增 isShared**

在 `web/src/lib/types.ts` 的 `SessionDetail` interface 中，在 `tools` 字段之后新增：

```typescript
isShared?: boolean;
```

---

## Task 8: 前端 — Session Detail 页面显示 Shared Badge

**Files:**
- Modify: `web/src/components/session-detail/session-detail-client.tsx`

- [ ] **Step 1: 在 header 区域显示 Shared Badge**

在 `web/src/components/session-detail/session-detail-client.tsx` 中，找到现有 `session.apiKeyName` Badge 的位置（约第 271-274 行），在其后新增：

```tsx
{session.isShared && (
  <Badge variant="outline" className="text-xs">
    Shared
  </Badge>
)}
```

- [ ] **Step 2: 验证前端编译**

Run: `cd web && npx next lint`
Expected: 通过

---

## Task 9: 前端 — ShareDialog 识别 DataExists 错误

**Files:**
- Modify: `web/src/components/share/share-dialog.tsx`

- [ ] **Step 1: 在 createShare 回调中识别 DataExists 错误码**

在 `web/src/components/share/share-dialog.tsx` 的 `createShare` 回调中，将现有的错误处理部分：

```tsx
if (rsp.error) {
  toast.error(rsp.error.message || "Failed to create share link");
  return;
}
```

替换为：

```tsx
if (rsp.error) {
  if (rsp.error.code === 10004) {
    toast.error("This session is already shared. Revoke the existing link first.");
  } else {
    toast.error(rsp.error.message || "Failed to create share link");
  }
  return;
}
```

---

## Task 10: 前端 — Shares 页面 ShareID 列改为超链接

**Files:**
- Modify: `web/src/app/(dashboard)/shares/page.tsx`

- [ ] **Step 1: 将 ShareID 列从纯文本改为超链接**

在 `web/src/app/(dashboard)/shares/page.tsx` 中，将现有的 ShareID 列：

```tsx
<TableCell className="max-w-[220px] truncate font-mono text-xs text-muted-foreground">
  {share.shareId}
</TableCell>
```

替换为：

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

注：`buildShareURL` 和 `ExternalLink` 已在文件中导入。

---

## Task 11: 前端编译验证

**Files:** 无改动

- [ ] **Step 1: 运行前端 lint**

Run: `cd web && npx next lint`
Expected: 通过

- [ ] **Step 2: 运行前端类型检查（如有）**

Run: `cd web && npx tsc --noEmit` 或项目配置的类型检查命令
Expected: 通过
