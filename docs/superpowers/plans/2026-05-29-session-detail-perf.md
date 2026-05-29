# Session 详情接口性能优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 session 详情接口拆为 metadata + message 分页 + tool 分页 三个接口，引入 1h Redis 缓存（session.meta / message / tool 三级），并改造前端从一次性加载改为向下滚动加载。

**Architecture:** 后端新增 `SessionDetailCache` 接口（与 `ShareCache` 同模式注入），三个新 huma 路由（`/metadata`、`/message/list`、`/tool/list`）在 `session` 路由组下挂载。读路径采用"权限身份前置（拿 ownerNames）→ 缓存优先 → DB 回源 → 权限比对"的顺序。前端抽 `useInfiniteList` hook 复用于 messages/tools。现有 `GET /session/` 保留向后兼容。

**Tech Stack:** Go 1.25 + huma v2 + GORM + go-redis v9 + uber/dig（后端）；Next.js 16 + React 19 + TypeScript + shadcn/ui（前端）

**关联设计:** `docs/superpowers/specs/2026-05-29-session-detail-perf-design.md`

**约定：** 每个任务结束跑 `make lint`（如适用）+ 聚焦测试，并独立 commit；命令的 cwd 默认 `.worktrees/feature/session-detail-perf-2026-05-29/`。

---

## 任务总览

| # | 任务 | 类型 |
|---|---|---|
| 1 | 新增缓存常量（rediskey + TTL） | 后端 |
| 2 | 新增 cache.SessionDetailCache 接口与实现 | 后端 |
| 3 | 缓存层载荷序列化测试 | 后端测试 |
| 4 | 新增 DTO（metadata / messageList / toolList） | 后端 |
| 5 | DTO 单元测试扩充 | 后端测试 |
| 6 | 新增 GetSessionMetaByUser query handler | 后端 |
| 7 | 新增 ListSessionMessages query handler | 后端 |
| 8 | 新增 ListSessionTools query handler | 后端 |
| 9 | 在 handler/session.go 新增 3 个 HTTP handler | 后端 |
| 10 | 在 router/session.go 注册 3 条新路由 | 后端 |
| 11 | 在 bootstrap/container.go 注册 cache provider 与 query providers | 后端 |
| 12 | E2E 测试 | 后端测试 |
| 13 | 前端：扩展类型 + API client | 前端 |
| 14 | 前端：新增 useInfiniteList hook | 前端 |
| 15 | 前端：改造 session-detail-client.tsx | 前端 |
| 16 | 全量回归 + 推送分支 | 收尾 |

---

## Task 1: 新增缓存常量

**Files:**
- Modify: `internal/common/constant/rediskey.go`
- Modify: `internal/common/constant/session.go`

- [ ] **Step 1: 编辑 `internal/common/constant/rediskey.go`**

整个文件替换为：

```go
package constant

const (
	LockKeyTemplateMiddleware = "%s:%s:%v"
	JWTUserCacheKeyTemplate   = "jwt:user:%d"
	TokenBucketKeyTemplate    = "tb:%s:%s:%v"
	ScannerBanKeyTemplate     = "scanner:ban:%s"
	ScannerStrikeKeyTemplate  = "scanner:strike:%s"
	ShareKeyTemplate          = "share:%s"
	UserSharesKeyTemplate     = "user_shares:%d"
	SessionSharesKeyTemplate  = "session_shares:%d"

	// SessionMetaKeyTemplate 缓存 session 元数据（含 messageIDs/toolIDs，仅内部使用）
	SessionMetaKeyTemplate = "session:meta:%d"
	// MessageKeyTemplate 缓存单条 message 详情（不可变，TTL 内永远有效）
	MessageKeyTemplate = "message:%d"
	// ToolKeyTemplate 缓存单条 tool 详情（不可变，TTL 内永远有效）
	ToolKeyTemplate = "tool:%d"
)
```

- [ ] **Step 2: 编辑 `internal/common/constant/session.go`**

整个文件替换为：

```go
package constant

import "time"

const (
	// SessionDetailCacheTTL session 详情相关缓存（meta / message / tool）的统一 TTL
	SessionDetailCacheTTL = time.Hour

	SummarizeMaxRetries = 3
	SummarizeMaxTokens  = 20

	ScoreMaxRetries = 3
	ScoreMaxTokens  = 200
	ScoreVersion    = "v1.0.0"

	EmptySessionSummary = "空会话"

	CronModuleSessionSummarize   = "SessionSummarizeCron"
	CronModuleSessionScore       = "SessionScoreCron"
	CronModuleSessionDeduplicate = "SessionDeduplicateCron"

	CronSpecSessionSummarize   = "0 2 * * *"
	CronSpecSessionScore       = "0 3 * * *"
	CronSpecSessionDeduplicate = "0 * * * *"
)
```

- [ ] **Step 3: 编译检查**

Run: `go build ./internal/common/constant/...`
Expected: 无输出

- [ ] **Step 4: Commit**

```bash
git add internal/common/constant/rediskey.go internal/common/constant/session.go
git commit -m "feat(session-detail-perf): 新增 session 详情缓存的 redis key 与 TTL 常量"
```

---

## Task 2: 新增 SessionDetailCache 接口与实现

**Files:**
- Create: `internal/infrastructure/cache/session_detail.go`

**前置参考（不要改动）：** `internal/infrastructure/cache/share.go`（构造函数 + Redis 操作模板）

- [ ] **Step 1: 创建 `internal/infrastructure/cache/session_detail.go`**

完整内容见任务文档 Task 2 附录 A（下方）。新建一个文件，按附录 A 复制内容。

- [ ] **Step 2: 编译**

Run: `go build ./internal/infrastructure/cache/...`
Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/cache/session_detail.go
git commit -m "feat(session-detail-perf): 新增 SessionDetailCache 接口与实现（meta/messages/tools）"
```

### 附录 A：`session_detail.go` 完整内容

```go
// Package cache Session 详情缓存操作
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// SessionMetaCacheRecord 是 session 元数据的缓存载荷。
//
// MessageIDs/ToolIDs 是 cache 内部字段，不直接透出给 API 响应：
// metadata 接口只透出 messageCount = len(MessageIDs)；
// message/tool 分页接口在内部读它们做 offset+limit 切片。
type SessionMetaCacheRecord struct {
	ID         uint              `json:"id"`
	APIKeyName string            `json:"apiKeyName"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	MessageIDs []uint            `json:"messageIds"`
	ToolIDs    []uint            `json:"toolIds"`
}

// MessageCacheRecord 单条 message 缓存载荷
type MessageCacheRecord struct {
	ID        uint               `json:"id"`
	Model     string             `json:"model"`
	Message   *vo.UnifiedMessage `json:"message"`
	CreatedAt time.Time          `json:"createdAt"`
}

// ToolCacheRecord 单条 tool 缓存载荷
type ToolCacheRecord struct {
	ID        uint            `json:"id"`
	Tool      *vo.UnifiedTool `json:"tool"`
	CreatedAt time.Time       `json:"createdAt"`
}

// SessionDetailCache session 详情相关的缓存接口
//
// 设计原则：
//   - Get* 系列：cache miss 不算 error；error 仅代表 Redis 通信故障，调用方应当 fallback 到 DB
//   - Set* 系列：用 Pipeline 批量写入；Redis 故障不阻断主流程
//   - message / tool 是不可变的，缓存内容一旦写入 TTL 内永远有效
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type SessionDetailCache interface {
	GetSessionMeta(ctx context.Context, sessionID uint) (*SessionMetaCacheRecord, error)
	SetSessionMeta(ctx context.Context, record *SessionMetaCacheRecord) error

	GetMessages(ctx context.Context, ids []uint) (hits map[uint]*MessageCacheRecord, missing []uint, err error)
	SetMessages(ctx context.Context, records []*MessageCacheRecord) error

	GetTools(ctx context.Context, ids []uint) (hits map[uint]*ToolCacheRecord, missing []uint, err error)
	SetTools(ctx context.Context, records []*ToolCacheRecord) error
}

type sessionDetailCache struct {
	cache *redis.Client
}

// NewSessionDetailCache 创建 session 详情缓存操作实例
//
//	@param cache *redis.Client
//	@return SessionDetailCache
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewSessionDetailCache(cache *redis.Client) SessionDetailCache {
	return &sessionDetailCache{cache: cache}
}

func (s *sessionDetailCache) GetSessionMeta(ctx context.Context, sessionID uint) (*SessionMetaCacheRecord, error) {
	key := fmt.Sprintf(constant.SessionMetaKeyTemplate, sessionID)
	val, err := s.cache.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrInternal, err, "failed to get session meta cache")
	}
	var record SessionMetaCacheRecord
	if unmarshalErr := sonic.UnmarshalString(val, &record); unmarshalErr != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, unmarshalErr, "failed to unmarshal session meta cache")
	}
	return &record, nil
}

func (s *sessionDetailCache) SetSessionMeta(ctx context.Context, record *SessionMetaCacheRecord) error {
	if record == nil {
		return ierr.New(ierr.ErrValidation, "session meta record cannot be nil")
	}
	payload, err := sonic.MarshalString(record)
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to marshal session meta cache")
	}
	key := fmt.Sprintf(constant.SessionMetaKeyTemplate, record.ID)
	if setErr := s.cache.Set(ctx, key, payload, constant.SessionDetailCacheTTL).Err(); setErr != nil {
		return ierr.Wrap(ierr.ErrInternal, setErr, "failed to set session meta cache")
	}
	return nil
}

func (s *sessionDetailCache) GetMessages(ctx context.Context, ids []uint) (map[uint]*MessageCacheRecord, []uint, error) {
	if len(ids) == 0 {
		return map[uint]*MessageCacheRecord{}, nil, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = fmt.Sprintf(constant.MessageKeyTemplate, id)
	}
	values, err := s.cache.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, ids, ierr.Wrap(ierr.ErrInternal, err, "failed to mget messages cache")
	}
	hits := make(map[uint]*MessageCacheRecord, len(values))
	missing := make([]uint, 0, len(ids))
	for i, v := range values {
		if v == nil {
			missing = append(missing, ids[i])
			continue
		}
		raw, ok := v.(string)
		if !ok {
			missing = append(missing, ids[i])
			continue
		}
		var record MessageCacheRecord
		if unmarshalErr := sonic.UnmarshalString(raw, &record); unmarshalErr != nil {
			missing = append(missing, ids[i])
			continue
		}
		hits[ids[i]] = &record
	}
	return hits, missing, nil
}

func (s *sessionDetailCache) SetMessages(ctx context.Context, records []*MessageCacheRecord) error {
	if len(records) == 0 {
		return nil
	}
	pipe := s.cache.Pipeline()
	for _, r := range records {
		if r == nil {
			continue
		}
		payload, err := sonic.MarshalString(r)
		if err != nil {
			return ierr.Wrap(ierr.ErrInternal, err, "failed to marshal message cache")
		}
		key := fmt.Sprintf(constant.MessageKeyTemplate, r.ID)
		pipe.Set(ctx, key, payload, constant.SessionDetailCacheTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to pipeline set messages cache")
	}
	return nil
}

func (s *sessionDetailCache) GetTools(ctx context.Context, ids []uint) (map[uint]*ToolCacheRecord, []uint, error) {
	if len(ids) == 0 {
		return map[uint]*ToolCacheRecord{}, nil, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = fmt.Sprintf(constant.ToolKeyTemplate, id)
	}
	values, err := s.cache.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, ids, ierr.Wrap(ierr.ErrInternal, err, "failed to mget tools cache")
	}
	hits := make(map[uint]*ToolCacheRecord, len(values))
	missing := make([]uint, 0, len(ids))
	for i, v := range values {
		if v == nil {
			missing = append(missing, ids[i])
			continue
		}
		raw, ok := v.(string)
		if !ok {
			missing = append(missing, ids[i])
			continue
		}
		var record ToolCacheRecord
		if unmarshalErr := sonic.UnmarshalString(raw, &record); unmarshalErr != nil {
			missing = append(missing, ids[i])
			continue
		}
		hits[ids[i]] = &record
	}
	return hits, missing, nil
}

func (s *sessionDetailCache) SetTools(ctx context.Context, records []*ToolCacheRecord) error {
	if len(records) == 0 {
		return nil
	}
	pipe := s.cache.Pipeline()
	for _, r := range records {
		if r == nil {
			continue
		}
		payload, err := sonic.MarshalString(r)
		if err != nil {
			return ierr.Wrap(ierr.ErrInternal, err, "failed to marshal tool cache")
		}
		key := fmt.Sprintf(constant.ToolKeyTemplate, r.ID)
		pipe.Set(ctx, key, payload, constant.SessionDetailCacheTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to pipeline set tools cache")
	}
	return nil
}
```

---

## Task 3: 缓存层载荷序列化测试

**Files:**
- Create: `test/unit/session_detail_cache/session_detail_cache_test.go`

**说明：** go-redis 无官方 mock，Redis 实际通信由 E2E 覆盖（Task 12）。本任务只测载荷类型的 sonic 往返一致性（最容易出错的点）。

- [ ] **Step 1: 创建目录**

```bash
mkdir -p test/unit/session_detail_cache
```

- [ ] **Step 2: 创建测试文件**

完整内容（创建 `test/unit/session_detail_cache/session_detail_cache_test.go`）：

```go
// Package session_detail_cache_test 测试 cache.SessionDetailCache 的载荷类型
//
// 由于 go-redis 无官方 mock，Redis 实际通信由 E2E 覆盖。
// 本单元测试聚焦"载荷类型的序列化往返一致性"。
package session_detail_cache_test

import (
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
)

func TestSessionMetaCacheRecord_RoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	original := &cache.SessionMetaCacheRecord{
		ID:         42,
		APIKeyName: "user-key-1",
		CreatedAt:  now,
		UpdatedAt:  now.Add(time.Minute),
		Metadata:   map[string]string{"source": "openai"},
		MessageIDs: []uint{1, 2, 3, 5, 8},
		ToolIDs:    []uint{10, 20},
	}

	payload, err := sonic.MarshalString(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded cache.SessionMetaCacheRecord
	if err := sonic.UnmarshalString(payload, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.APIKeyName != original.APIKeyName {
		t.Errorf("APIKeyName mismatch")
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch")
	}
	if len(decoded.MessageIDs) != len(original.MessageIDs) {
		t.Fatalf("MessageIDs length mismatch")
	}
	for i, id := range original.MessageIDs {
		if decoded.MessageIDs[i] != id {
			t.Errorf("MessageIDs[%d] mismatch", i)
		}
	}
	if decoded.Metadata["source"] != "openai" {
		t.Errorf("Metadata not preserved")
	}
}

func TestSessionMetaCacheRecord_EmptyIDs(t *testing.T) {
	original := &cache.SessionMetaCacheRecord{
		ID:         1,
		APIKeyName: "k",
		MessageIDs: []uint{},
		ToolIDs:    []uint{},
	}
	payload, err := sonic.MarshalString(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded cache.SessionMetaCacheRecord
	if err := sonic.UnmarshalString(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.MessageIDs) != 0 {
		t.Errorf("expected empty MessageIDs")
	}
}

func TestMessageCacheRecord_RoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 29, 11, 0, 0, 0, time.UTC)
	original := &cache.MessageCacheRecord{
		ID:    100,
		Model: "gpt-4",
		Message: &vo.UnifiedMessage{
			Role:    enum.RoleUser,
			Content: &vo.UnifiedContent{Text: "hello"},
		},
		CreatedAt: now,
	}

	payload, err := sonic.MarshalString(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded cache.MessageCacheRecord
	if err := sonic.UnmarshalString(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != 100 || decoded.Model != "gpt-4" {
		t.Errorf("scalar fields mismatch")
	}
	if decoded.Message == nil {
		t.Fatalf("Message is nil after round trip")
	}
	if decoded.Message.Role != enum.RoleUser {
		t.Errorf("Role mismatch")
	}
	if decoded.Message.Content == nil || decoded.Message.Content.Text != "hello" {
		t.Errorf("Content text mismatch")
	}
}

func TestToolCacheRecord_RoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	original := &cache.ToolCacheRecord{
		ID: 7,
		Tool: &vo.UnifiedTool{
			Name:        "search",
			Description: "do a web search",
		},
		CreatedAt: now,
	}

	payload, err := sonic.MarshalString(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded cache.ToolCacheRecord
	if err := sonic.UnmarshalString(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != 7 {
		t.Errorf("ID mismatch")
	}
	if decoded.Tool == nil || decoded.Tool.Name != "search" {
		t.Errorf("Tool.Name not preserved")
	}
	if decoded.Tool.Description != "do a web search" {
		t.Errorf("Tool.Description not preserved")
	}
}
```

- [ ] **Step 3: 运行测试**

Run: `go test -v -count=1 ./test/unit/session_detail_cache/`
Expected: 4 个测试 PASS

- [ ] **Step 4: Commit**

```bash
git add test/unit/session_detail_cache/
git commit -m "test(session-detail-perf): 新增 SessionDetailCache 载荷序列化往返测试"
```

---

## Task 4: 新增 DTO

**Files:**
- Modify: `internal/dto/session.go`

- [ ] **Step 1: 在 `internal/dto/session.go` 文件末尾追加**

```go
// SessionMetadata Session 元数据（不含 messages/tools 内容）
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type SessionMetadata struct {
	ID           uint              `json:"id" doc:"Session ID"`
	APIKeyName   string            `json:"apiKeyName" doc:"API密钥名称"`
	CreatedAt    time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt    time.Time         `json:"updatedAt" doc:"更新时间"`
	Metadata     map[string]string `json:"metadata,omitempty" doc:"请求元数据"`
	MessageCount int               `json:"messageCount" doc:"消息总数"`
	ToolCount    int               `json:"toolCount" doc:"工具总数"`
	ShareID      string            `json:"shareID" doc:"分享ID（已分享时非空）"`
}

// GetSessionMetadataReq 获取 Session 元数据请求（JWT 认证）
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type GetSessionMetadataReq struct {
	SessionID uint `query:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
}

// GetSessionMetadataRsp 获取 Session 元数据响应
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type GetSessionMetadataRsp struct {
	CommonRsp
	Session *SessionMetadata `json:"session,omitempty" doc:"Session 元数据"`
}

// OffsetPageInfo 基于 offset+limit 的分页信息（用于滚动加载）
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type OffsetPageInfo struct {
	Offset int   `json:"offset" doc:"偏移量"`
	Limit  int   `json:"limit" doc:"页大小"`
	Total  int64 `json:"total" doc:"总数"`
}

// ListSessionMessagesReq 分页获取 Session 消息请求
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListSessionMessagesReq struct {
	SessionID uint `query:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
	Offset    int  `query:"offset" minimum:"0" default:"0" doc:"偏移量"`
	Limit     int  `query:"limit" minimum:"1" maximum:"100" default:"20" doc:"页大小"`
}

// ListSessionMessagesRsp 分页获取 Session 消息响应
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListSessionMessagesRsp struct {
	CommonRsp
	Messages []*MessageItem  `json:"messages,omitempty" doc:"消息列表"`
	PageInfo *OffsetPageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ListSessionToolsReq 分页获取 Session 工具请求
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListSessionToolsReq struct {
	SessionID uint `query:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
	Offset    int  `query:"offset" minimum:"0" default:"0" doc:"偏移量"`
	Limit     int  `query:"limit" minimum:"1" maximum:"100" default:"50" doc:"页大小"`
}

// ListSessionToolsRsp 分页获取 Session 工具响应
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListSessionToolsRsp struct {
	CommonRsp
	Tools    []*ToolItem     `json:"tools,omitempty" doc:"工具列表"`
	PageInfo *OffsetPageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}
```

- [ ] **Step 2: 编译 + 现有测试**

Run: `go build ./internal/dto/... && go test -count=1 ./test/unit/session_dto/`
Expected: 全 PASS

- [ ] **Step 3: Commit**

```bash
git add internal/dto/session.go
git commit -m "feat(session-detail-perf): 新增 SessionMetadata / OffsetPageInfo / 3 组 Req-Rsp DTO"
```

---

## Task 5: DTO 单元测试扩充

**Files:**
- Modify: `test/unit/session_dto/session_dto_test.go`

- [ ] **Step 1: 检查现有 import**

```bash
head -20 test/unit/session_dto/session_dto_test.go
```

确认包含 `"strings"`、`"time"`、`"github.com/bytedance/sonic"`、`dto "github.com/hcd233/aris-proxy-api/internal/dto"`。如缺，补上。

- [ ] **Step 2: 在文件末尾追加**

```go
func TestSessionMetadata_JSONSerialization(t *testing.T) {
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	meta := &dto.SessionMetadata{
		ID:           42,
		APIKeyName:   "user-key-1",
		CreatedAt:    now,
		UpdatedAt:    now.Add(time.Minute),
		Metadata:     map[string]string{"source": "openai"},
		MessageCount: 156,
		ToolCount:    8,
		ShareID:      "abc123",
	}

	payload, err := sonic.MarshalString(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(payload, `"messageCount":156`) {
		t.Errorf("payload should contain messageCount: %s", payload)
	}
	if !strings.Contains(payload, `"toolCount":8`) {
		t.Errorf("payload should contain toolCount: %s", payload)
	}
	if strings.Contains(payload, "messageIds") || strings.Contains(payload, "toolIds") {
		t.Errorf("metadata response must NOT expose IDs arrays: %s", payload)
	}

	var decoded dto.SessionMetadata
	if err := sonic.UnmarshalString(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MessageCount != 156 || decoded.ToolCount != 8 {
		t.Errorf("count fields not preserved")
	}
	if decoded.ShareID != "abc123" {
		t.Errorf("ShareID mismatch")
	}
}

func TestListSessionMessagesRsp_PageInfoFields(t *testing.T) {
	rsp := &dto.ListSessionMessagesRsp{
		Messages: []*dto.MessageItem{},
		PageInfo: &dto.OffsetPageInfo{Offset: 20, Limit: 20, Total: 156},
	}
	payload, err := sonic.MarshalString(rsp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(payload, `"offset":20`) ||
		!strings.Contains(payload, `"limit":20`) ||
		!strings.Contains(payload, `"total":156`) {
		t.Errorf("OffsetPageInfo fields missing in payload: %s", payload)
	}
	if strings.Contains(payload, `"page"`) || strings.Contains(payload, `"pageSize"`) {
		t.Errorf("OffsetPageInfo must NOT use page/pageSize naming: %s", payload)
	}
}

func TestListSessionToolsRsp_EmptyTools(t *testing.T) {
	rsp := &dto.ListSessionToolsRsp{
		Tools:    nil,
		PageInfo: &dto.OffsetPageInfo{Offset: 0, Limit: 50, Total: 0},
	}
	payload, err := sonic.MarshalString(rsp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(payload, `"tools"`) {
		t.Errorf("nil tools should be omitted, got: %s", payload)
	}
}
```

- [ ] **Step 3: 测试**

Run: `go test -v -count=1 ./test/unit/session_dto/`
Expected: 包含新 3 个测试在内的全部 PASS

- [ ] **Step 4: Commit**

```bash
git add test/unit/session_dto/
git commit -m "test(session-detail-perf): 扩充 session DTO 测试覆盖新增类型"
```

---

## Task 6: 新增 GetSessionMetaByUser query handler

**Files:**
- Create: `internal/application/session/query/session_meta_query.go`

- [ ] **Step 1: 创建文件**

完整内容见附录 B。

- [ ] **Step 2: 编译**

Run: `go build ./internal/application/session/query/...`
Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add internal/application/session/query/session_meta_query.go
git commit -m "feat(session-detail-perf): 新增 GetSessionMetaByUser query handler（缓存优先 + 权限前置）"
```

### 附录 B：`session_meta_query.go` 完整内容

```go
package query

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// GetSessionMetaByUserQuery 获取 session 元数据查询参数
type GetSessionMetaByUserQuery struct {
	UserID    uint
	IsAdmin   bool
	SessionID uint
}

// SessionMetaView session 元数据视图（含 IDs 数组，仅在 application 层内部使用）
type SessionMetaView struct {
	ID           uint
	APIKeyName   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Metadata     map[string]string
	MessageIDs   []uint
	ToolIDs      []uint
	MessageCount int
	ToolCount    int
}

// GetSessionMetaByUserHandler 元数据查询 handler 接口
type GetSessionMetaByUserHandler interface {
	Handle(ctx context.Context, q GetSessionMetaByUserQuery) (*SessionMetaView, error)
}

type getSessionMetaByUserHandler struct {
	db         *gorm.DB
	sessionDAO *dao.SessionDAO
	apiKeyRepo apikey.APIKeyRepository
	cache      cache.SessionDetailCache
}

// NewGetSessionMetaByUserHandler 构造 handler
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewGetSessionMetaByUserHandler(db *gorm.DB, apiKeyRepo apikey.APIKeyRepository, detailCache cache.SessionDetailCache) GetSessionMetaByUserHandler {
	return &getSessionMetaByUserHandler{
		db:         db,
		sessionDAO: dao.GetSessionDAO(),
		apiKeyRepo: apiKeyRepo,
		cache:      detailCache,
	}
}

// Handle 流程见 spec §3.3.1：
//   1. 校验 SessionID
//   2. 拿 user 的 ownerNames（admin 跳过）
//   3. 缓存命中检查
//   4. SQL 取 session 行（缓存未命中时）
//   5. 写缓存
//   6. 权限比对
func (h *getSessionMetaByUserHandler) Handle(ctx context.Context, q GetSessionMetaByUserQuery) (*SessionMetaView, error) {
	log := logger.WithCtx(ctx)

	if q.SessionID == 0 {
		return nil, ierr.New(ierr.ErrValidation, "sessionID must be greater than 0")
	}

	var ownerNames []string
	if !q.IsAdmin {
		names, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner names", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, lookupErr
		}
		ownerNames = names
	}

	record, cacheErr := h.cache.GetSessionMeta(ctx, q.SessionID)
	if cacheErr != nil {
		log.Warn("[SessionQuery] GetSessionMeta cache failed, fallback to DB",
			zap.Uint("sessionID", q.SessionID), zap.Error(cacheErr))
		record = nil
	}

	if record == nil {
		dbRecord, sqlErr := h.sessionDAO.Get(h.db.WithContext(ctx), &dbmodel.Session{ID: q.SessionID}, constant.SessionRepoFieldsReadDetail)
		if sqlErr != nil {
			if errors.Is(sqlErr, gorm.ErrRecordNotFound) {
				return nil, ierr.New(ierr.ErrDataNotExists, "session not found")
			}
			log.Error("[SessionQuery] Get session row failed",
				zap.Uint("sessionID", q.SessionID), zap.Error(sqlErr))
			return nil, ierr.Wrap(ierr.ErrDBQuery, sqlErr, "get session row")
		}

		record = &cache.SessionMetaCacheRecord{
			ID:         dbRecord.ID,
			APIKeyName: dbRecord.APIKeyName,
			CreatedAt:  dbRecord.CreatedAt,
			UpdatedAt:  dbRecord.UpdatedAt,
			Metadata:   dbRecord.Metadata,
			MessageIDs: dbRecord.MessageIDs,
			ToolIDs:    dbRecord.ToolIDs,
		}

		if setErr := h.cache.SetSessionMeta(ctx, record); setErr != nil {
			log.Warn("[SessionQuery] SetSessionMeta cache failed",
				zap.Uint("sessionID", q.SessionID), zap.Error(setErr))
		}
	}

	if !q.IsAdmin {
		allowed := false
		for _, name := range ownerNames {
			if record.APIKeyName == name {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Warn("[SessionQuery] No permission to access session",
				zap.Uint("sessionID", q.SessionID),
				zap.String("owner", record.APIKeyName),
				zap.Uint("userID", q.UserID))
			return nil, ierr.New(ierr.ErrNoPermission, "no permission to access session")
		}
	}

	return &SessionMetaView{
		ID:           record.ID,
		APIKeyName:   record.APIKeyName,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
		Metadata:     record.Metadata,
		MessageIDs:   record.MessageIDs,
		ToolIDs:      record.ToolIDs,
		MessageCount: len(record.MessageIDs),
		ToolCount:    len(record.ToolIDs),
	}, nil
}
```

---

## Task 7: 新增 ListSessionMessages query handler

**Files:**
- Create: `internal/application/session/query/session_message_list_query.go`

- [ ] **Step 1: 创建文件，完整内容如下**

```go
package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ListSessionMessagesQuery 分页获取 session messages 查询参数
type ListSessionMessagesQuery struct {
	UserID    uint
	IsAdmin   bool
	SessionID uint
	Offset    int
	Limit     int
}

// ListSessionMessagesResult 分页结果
type ListSessionMessagesResult struct {
	Messages []*MessageView
	Total    int64
}

// ListSessionMessagesHandler 分页获取 messages handler 接口
type ListSessionMessagesHandler interface {
	Handle(ctx context.Context, q ListSessionMessagesQuery) (*ListSessionMessagesResult, error)
}

type listSessionMessagesHandler struct {
	db         *gorm.DB
	messageDAO *dao.MessageDAO
	metaQuery  GetSessionMetaByUserHandler
	cache      cache.SessionDetailCache
}

// NewListSessionMessagesHandler 构造
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewListSessionMessagesHandler(db *gorm.DB, metaQuery GetSessionMetaByUserHandler, detailCache cache.SessionDetailCache) ListSessionMessagesHandler {
	return &listSessionMessagesHandler{
		db:         db,
		messageDAO: dao.GetMessageDAO(),
		metaQuery:  metaQuery,
		cache:      detailCache,
	}
}

// Handle 复用 metaQuery 完成"权限校验+元数据获取"，再在内存中切片+缓存批读
func (h *listSessionMessagesHandler) Handle(ctx context.Context, q ListSessionMessagesQuery) (*ListSessionMessagesResult, error) {
	log := logger.WithCtx(ctx)

	meta, err := h.metaQuery.Handle(ctx, GetSessionMetaByUserQuery{
		UserID:    q.UserID,
		IsAdmin:   q.IsAdmin,
		SessionID: q.SessionID,
	})
	if err != nil {
		return nil, err
	}

	total := int64(len(meta.MessageIDs))
	if total == 0 {
		return &ListSessionMessagesResult{Messages: []*MessageView{}, Total: 0}, nil
	}

	offset := q.Offset
	if offset > len(meta.MessageIDs) {
		offset = len(meta.MessageIDs)
	}
	end := offset + q.Limit
	if end > len(meta.MessageIDs) {
		end = len(meta.MessageIDs)
	}
	pageIDs := meta.MessageIDs[offset:end]
	if len(pageIDs) == 0 {
		return &ListSessionMessagesResult{Messages: []*MessageView{}, Total: total}, nil
	}

	hits, missing, cacheErr := h.cache.GetMessages(ctx, pageIDs)
	if cacheErr != nil {
		log.Warn("[SessionQuery] GetMessages cache failed, fallback to DB",
			zap.Error(cacheErr), zap.Int("idsLen", len(pageIDs)))
		hits = map[uint]*cache.MessageCacheRecord{}
		missing = pageIDs
	}

	if len(missing) > 0 {
		records, sqlErr := h.messageDAO.BatchGetByField(h.db.WithContext(ctx), constant.WhereFieldID, lo.Uniq(missing), constant.MessageRepoFieldsDetail)
		if sqlErr != nil {
			log.Error("[SessionQuery] BatchGet messages failed", zap.Error(sqlErr))
			return nil, ierr.Wrap(ierr.ErrDBQuery, sqlErr, "batch get messages")
		}

		fetched := make([]*cache.MessageCacheRecord, 0, len(records))
		for _, m := range records {
			rec := &cache.MessageCacheRecord{
				ID:        m.ID,
				Model:     m.Model,
				Message:   m.Message,
				CreatedAt: m.CreatedAt,
			}
			hits[m.ID] = rec
			fetched = append(fetched, rec)
		}
		if setErr := h.cache.SetMessages(ctx, fetched); setErr != nil {
			log.Warn("[SessionQuery] SetMessages cache failed", zap.Error(setErr))
		}
	}

	views := make([]*MessageView, 0, len(pageIDs))
	for _, id := range pageIDs {
		rec, ok := hits[id]
		if !ok {
			continue
		}
		views = append(views, &MessageView{
			ID:        rec.ID,
			Model:     rec.Model,
			Message:   rec.Message,
			CreatedAt: rec.CreatedAt,
		})
	}
	return &ListSessionMessagesResult{Messages: views, Total: total}, nil
}
```

- [ ] **Step 2: 编译**

Run: `go build ./internal/application/session/query/...`
Expected: 无输出

- [ ] **Step 3: Commit**

```bash
git add internal/application/session/query/session_message_list_query.go
git commit -m "feat(session-detail-perf): 新增 ListSessionMessages query handler（offset+limit 分页 + 缓存）"
```

---

## Task 8: 新增 ListSessionTools query handler

**Files:**
- Create: `internal/application/session/query/session_tool_list_query.go`

完全对称于 Task 7，replace `Message` → `Tool`，复制下面完整内容：

```go
package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type ListSessionToolsQuery struct {
	UserID    uint
	IsAdmin   bool
	SessionID uint
	Offset    int
	Limit     int
}

type ListSessionToolsResult struct {
	Tools []*ToolView
	Total int64
}

type ListSessionToolsHandler interface {
	Handle(ctx context.Context, q ListSessionToolsQuery) (*ListSessionToolsResult, error)
}

type listSessionToolsHandler struct {
	db        *gorm.DB
	toolDAO   *dao.ToolDAO
	metaQuery GetSessionMetaByUserHandler
	cache     cache.SessionDetailCache
}

// NewListSessionToolsHandler 构造
func NewListSessionToolsHandler(db *gorm.DB, metaQuery GetSessionMetaByUserHandler, detailCache cache.SessionDetailCache) ListSessionToolsHandler {
	return &listSessionToolsHandler{
		db:        db,
		toolDAO:   dao.GetToolDAO(),
		metaQuery: metaQuery,
		cache:     detailCache,
	}
}

func (h *listSessionToolsHandler) Handle(ctx context.Context, q ListSessionToolsQuery) (*ListSessionToolsResult, error) {
	log := logger.WithCtx(ctx)

	meta, err := h.metaQuery.Handle(ctx, GetSessionMetaByUserQuery{
		UserID:    q.UserID,
		IsAdmin:   q.IsAdmin,
		SessionID: q.SessionID,
	})
	if err != nil {
		return nil, err
	}

	total := int64(len(meta.ToolIDs))
	if total == 0 {
		return &ListSessionToolsResult{Tools: []*ToolView{}, Total: 0}, nil
	}

	offset := q.Offset
	if offset > len(meta.ToolIDs) {
		offset = len(meta.ToolIDs)
	}
	end := offset + q.Limit
	if end > len(meta.ToolIDs) {
		end = len(meta.ToolIDs)
	}
	pageIDs := meta.ToolIDs[offset:end]
	if len(pageIDs) == 0 {
		return &ListSessionToolsResult{Tools: []*ToolView{}, Total: total}, nil
	}

	hits, missing, cacheErr := h.cache.GetTools(ctx, pageIDs)
	if cacheErr != nil {
		log.Warn("[SessionQuery] GetTools cache failed, fallback to DB",
			zap.Error(cacheErr), zap.Int("idsLen", len(pageIDs)))
		hits = map[uint]*cache.ToolCacheRecord{}
		missing = pageIDs
	}

	if len(missing) > 0 {
		records, sqlErr := h.toolDAO.BatchGetByField(h.db.WithContext(ctx), constant.WhereFieldID, lo.Uniq(missing), constant.ToolRepoFieldsDetail)
		if sqlErr != nil {
			log.Error("[SessionQuery] BatchGet tools failed", zap.Error(sqlErr))
			return nil, ierr.Wrap(ierr.ErrDBQuery, sqlErr, "batch get tools")
		}

		fetched := make([]*cache.ToolCacheRecord, 0, len(records))
		for _, t := range records {
			rec := &cache.ToolCacheRecord{
				ID:        t.ID,
				Tool:      t.Tool,
				CreatedAt: t.CreatedAt,
			}
			hits[t.ID] = rec
			fetched = append(fetched, rec)
		}
		if setErr := h.cache.SetTools(ctx, fetched); setErr != nil {
			log.Warn("[SessionQuery] SetTools cache failed", zap.Error(setErr))
		}
	}

	views := make([]*ToolView, 0, len(pageIDs))
	for _, id := range pageIDs {
		rec, ok := hits[id]
		if !ok {
			continue
		}
		views = append(views, &ToolView{
			ID:        rec.ID,
			Tool:      rec.Tool,
			CreatedAt: rec.CreatedAt,
		})
	}
	return &ListSessionToolsResult{Tools: views, Total: total}, nil
}
```

- [ ] **Step 2: 编译 + commit**

```bash
go build ./internal/application/session/query/...
git add internal/application/session/query/session_tool_list_query.go
git commit -m "feat(session-detail-perf): 新增 ListSessionTools query handler（offset+limit 分页 + 缓存）"
```

---

## Task 9: 在 handler/session.go 新增 3 个 HTTP handler

**Files:**
- Modify: `internal/handler/session.go`

- [ ] **Step 1: 接口扩充**

定位到 `type SessionHandler interface { ... }`，在末尾追加 3 个方法签名（保留原有 6 个）：

```go
// 新增（详情接口性能优化）
HandleGetSessionMetadata(ctx context.Context, req *dto.GetSessionMetadataReq) (*dto.HTTPResponse[*dto.GetSessionMetadataRsp], error)
HandleListSessionMessages(ctx context.Context, req *dto.ListSessionMessagesReq) (*dto.HTTPResponse[*dto.ListSessionMessagesRsp], error)
HandleListSessionTools(ctx context.Context, req *dto.ListSessionToolsReq) (*dto.HTTPResponse[*dto.ListSessionToolsRsp], error)
```

- [ ] **Step 2: SessionDependencies 扩充**

在 `type SessionDependencies struct { ... }` 末尾追加：

```go
// 新增（详情接口性能优化）
GetMetaByUser sessionquery.GetSessionMetaByUserHandler
ListMessages  sessionquery.ListSessionMessagesHandler
ListTools     sessionquery.ListSessionToolsHandler
```

- [ ] **Step 3: sessionHandler 结构与构造函数**

替换 `type sessionHandler struct { ... }` 为：

```go
type sessionHandler struct {
	listByUser    sessionquery.ListSessionsByUserHandler
	getByUser     sessionquery.GetSessionByUserHandler
	shareCache    cache.ShareCache
	getMetaByUser sessionquery.GetSessionMetaByUserHandler
	listMessages  sessionquery.ListSessionMessagesHandler
	listTools     sessionquery.ListSessionToolsHandler
}
```

替换 `func NewSessionHandler(...)` 为：

```go
func NewSessionHandler(deps SessionDependencies) SessionHandler {
	return &sessionHandler{
		listByUser:    deps.ListByUser,
		getByUser:     deps.GetByUser,
		shareCache:    deps.ShareCache,
		getMetaByUser: deps.GetMetaByUser,
		listMessages:  deps.ListMessages,
		listTools:     deps.ListTools,
	}
}
```

- [ ] **Step 4: 在文件末尾追加 3 个 handler 方法实现**

```go
// HandleGetSessionMetadata 获取 Session 元数据（不含 messages/tools 内容）
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (h *sessionHandler) HandleGetSessionMetadata(ctx context.Context, req *dto.GetSessionMetadataReq) (*dto.HTTPResponse[*dto.GetSessionMetadataRsp], error) {
	rsp := &dto.GetSessionMetadataRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	view, err := h.getMetaByUser.Handle(ctx, sessionquery.GetSessionMetaByUserQuery{
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: req.SessionID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Get session metadata failed",
			zap.Uint("sessionID", req.SessionID), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	shareID, sharedErr := h.shareCache.GetSessionShareID(ctx, req.SessionID)
	if sharedErr != nil {
		logger.WithCtx(ctx).Warn("[SessionHandler] Check session shared status failed",
			zap.Uint("sessionID", req.SessionID), zap.Error(sharedErr))
		shareID = ""
	}

	rsp.Session = &dto.SessionMetadata{
		ID:           view.ID,
		APIKeyName:   view.APIKeyName,
		CreatedAt:    view.CreatedAt,
		UpdatedAt:    view.UpdatedAt,
		Metadata:     view.Metadata,
		MessageCount: view.MessageCount,
		ToolCount:    view.ToolCount,
		ShareID:      shareID,
	}

	logger.WithCtx(ctx).Info("[SessionHandler] Get session metadata",
		zap.Uint("sessionID", req.SessionID),
		zap.Uint("userID", userID),
		zap.Int("messageCount", view.MessageCount),
		zap.Int("toolCount", view.ToolCount))

	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListSessionMessages 分页获取 Session 消息
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (h *sessionHandler) HandleListSessionMessages(ctx context.Context, req *dto.ListSessionMessagesReq) (*dto.HTTPResponse[*dto.ListSessionMessagesRsp], error) {
	rsp := &dto.ListSessionMessagesRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	result, err := h.listMessages.Handle(ctx, sessionquery.ListSessionMessagesQuery{
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: req.SessionID,
		Offset:    req.Offset,
		Limit:     req.Limit,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] List session messages failed",
			zap.Uint("sessionID", req.SessionID),
			zap.Int("offset", req.Offset), zap.Int("limit", req.Limit), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Messages = lo.Map(result.Messages, func(m *sessionquery.MessageView, _ int) *dto.MessageItem {
		return &dto.MessageItem{
			ID:        m.ID,
			Model:     m.Model,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		}
	})
	rsp.PageInfo = &dto.OffsetPageInfo{
		Offset: req.Offset,
		Limit:  req.Limit,
		Total:  result.Total,
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListSessionTools 分页获取 Session 工具
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (h *sessionHandler) HandleListSessionTools(ctx context.Context, req *dto.ListSessionToolsReq) (*dto.HTTPResponse[*dto.ListSessionToolsRsp], error) {
	rsp := &dto.ListSessionToolsRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	result, err := h.listTools.Handle(ctx, sessionquery.ListSessionToolsQuery{
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: req.SessionID,
		Offset:    req.Offset,
		Limit:     req.Limit,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] List session tools failed",
			zap.Uint("sessionID", req.SessionID),
			zap.Int("offset", req.Offset), zap.Int("limit", req.Limit), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Tools = lo.Map(result.Tools, func(t *sessionquery.ToolView, _ int) *dto.ToolItem {
		return &dto.ToolItem{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		}
	})
	rsp.PageInfo = &dto.OffsetPageInfo{
		Offset: req.Offset,
		Limit:  req.Limit,
		Total:  result.Total,
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 5: 编译**

Run: `go build ./internal/handler/...`
Expected: 暂时会失败（因为 container.go 还没注入新依赖），编译错误应当只与 SessionDependencies 字段相关或 NewSessionHandler 调用相关——这是预期的，会在 Task 11 修复。仅确认 handler 包**自身**编译错误为 0：

Run: `go vet ./internal/handler/...`
Expected: 仅 SessionDependencies 字段未在 container.go 提供的间接错误（如有）

如果只是 handler 包本身的报错（缺 import 等），现在修；container 相关错误留到 Task 11。

- [ ] **Step 6: Commit**

```bash
git add internal/handler/session.go
git commit -m "feat(session-detail-perf): handler/session.go 新增 3 个 HTTP handler 方法"
```

---

## Task 10: 在 router/session.go 注册 3 条新路由

**Files:**
- Modify: `internal/router/session.go`

- [ ] **Step 1: 在 `initSessionJWTRouter` 函数中、`initSessionShareRouter` 调用之前，追加 3 条 huma.Register**

定位到 `func initSessionJWTRouter(...) {`，在现有 `getSession` 注册之后、`initSessionShareRouter(sessionGroup, sessionHandler)` 之前，插入：

```go
huma.Register(sessionGroup, huma.Operation{
	OperationID: "getSessionMetadata",
	Method:      http.MethodGet,
	Path:        "/metadata",
	Summary:     "GetSessionMetadata",
	Description: "Get session metadata (without messages/tools content)",
	Tags:        []string{"Session"},
	Security:    []map[string][]string{{"jwtAuth": {}}},
	Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getSessionMetadata", enum.PermissionUser)},
}, sessionHandler.HandleGetSessionMetadata)

huma.Register(sessionGroup, huma.Operation{
	OperationID: "listSessionMessages",
	Method:      http.MethodGet,
	Path:        "/message/list",
	Summary:     "ListSessionMessages",
	Description: "Paginate session messages by offset+limit",
	Tags:        []string{"Session"},
	Security:    []map[string][]string{{"jwtAuth": {}}},
	Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listSessionMessages", enum.PermissionUser)},
}, sessionHandler.HandleListSessionMessages)

huma.Register(sessionGroup, huma.Operation{
	OperationID: "listSessionTools",
	Method:      http.MethodGet,
	Path:        "/tool/list",
	Summary:     "ListSessionTools",
	Description: "Paginate session tools by offset+limit",
	Tags:        []string{"Session"},
	Security:    []map[string][]string{{"jwtAuth": {}}},
	Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listSessionTools", enum.PermissionUser)},
}, sessionHandler.HandleListSessionTools)
```

- [ ] **Step 2: 编译**

Run: `go build ./internal/router/...`
Expected: 与 handler 类似，可能仅在 container 修复后才能整体编译通过

- [ ] **Step 3: Commit**

```bash
git add internal/router/session.go
git commit -m "feat(session-detail-perf): 注册 /metadata /message/list /tool/list 三条路由"
```

---

## Task 11: 在 bootstrap/container.go 注册 cache provider 与 query providers

**Files:**
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: 在 `provideInfrastructure` 函数中、`newShareCache` 注册之后，追加 newSessionDetailCache provider**

定位到 `if err := container.Provide(newShareCache); err != nil { return err }`，在其之后插入：

```go
if err := container.Provide(newSessionDetailCache); err != nil {
	return err
}
```

- [ ] **Step 2: 在 `provideQueries` 中（即 `newGetSessionByUserHandler` 注册的同一个函数），追加 3 个新 query provider**

定位到 `if err := container.Provide(newGetSessionByUserHandler); err != nil { return err }`（约第 256 行附近），在其之后插入：

```go
if err := container.Provide(newGetSessionMetaByUserHandler); err != nil {
	return err
}
if err := container.Provide(newListSessionMessagesHandler); err != nil {
	return err
}
if err := container.Provide(newListSessionToolsHandler); err != nil {
	return err
}
```

- [ ] **Step 3: 修改 `newSessionDependencies` 函数签名与实现**

定位到现有：

```go
func newSessionDependencies(listByUser sessionquery.ListSessionsByUserHandler, getByUser sessionquery.GetSessionByUserHandler, shareCache cache.ShareCache) handler.SessionDependencies {
	return handler.SessionDependencies{ListByUser: listByUser, GetByUser: getByUser, ShareCache: shareCache}
}
```

替换为：

```go
func newSessionDependencies(
	listByUser sessionquery.ListSessionsByUserHandler,
	getByUser sessionquery.GetSessionByUserHandler,
	shareCache cache.ShareCache,
	getMetaByUser sessionquery.GetSessionMetaByUserHandler,
	listMessages sessionquery.ListSessionMessagesHandler,
	listTools sessionquery.ListSessionToolsHandler,
) handler.SessionDependencies {
	return handler.SessionDependencies{
		ListByUser:    listByUser,
		GetByUser:     getByUser,
		ShareCache:    shareCache,
		GetMetaByUser: getMetaByUser,
		ListMessages:  listMessages,
		ListTools:     listTools,
	}
}
```

- [ ] **Step 4: 在文件末尾（`newShareCache` 之后）追加 4 个 provider 工厂函数**

```go
func newSessionDetailCache(redisClient *redis.Client) cache.SessionDetailCache {
	return cache.NewSessionDetailCache(redisClient)
}

func newGetSessionMetaByUserHandler(db *gorm.DB, apiKeyRepo apikey.APIKeyRepository, detailCache cache.SessionDetailCache) sessionquery.GetSessionMetaByUserHandler {
	return sessionquery.NewGetSessionMetaByUserHandler(db, apiKeyRepo, detailCache)
}

func newListSessionMessagesHandler(db *gorm.DB, metaQuery sessionquery.GetSessionMetaByUserHandler, detailCache cache.SessionDetailCache) sessionquery.ListSessionMessagesHandler {
	return sessionquery.NewListSessionMessagesHandler(db, metaQuery, detailCache)
}

func newListSessionToolsHandler(db *gorm.DB, metaQuery sessionquery.GetSessionMetaByUserHandler, detailCache cache.SessionDetailCache) sessionquery.ListSessionToolsHandler {
	return sessionquery.NewListSessionToolsHandler(db, metaQuery, detailCache)
}
```

注：`gorm.DB`、`apikey.APIKeyRepository` 已在 container.go 顶部 import；如果未 import，根据编译错误补全。

- [ ] **Step 5: 全量编译**

Run: `make build` 或 `go build ./...`
Expected: 无错误

- [ ] **Step 6: 全量测试**

Run: `go test -count=1 ./...`
Expected: 全 PASS

- [ ] **Step 7: lint**

Run: `make lint`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/bootstrap/container.go
git commit -m "feat(session-detail-perf): bootstrap 注册 SessionDetailCache 与 3 个新 query handler"
```

---

## Task 12: E2E 测试

**Files:**
- Create: `test/e2e/session_detail_perf/session_detail_perf_test.go`

E2E 约束（CODEBUDDY.md §6）：必须读 `BASE_URL` + `API_KEY`，二者任一为空 `t.Skip`；HTTP client 显式超时；不内联大 JSON。

**注意：** 本项目 E2E 用 `API_KEY` 进 LLM 代理路由（`/api/openai/v1/...`），但本次新增的接口走 `jwtAuth`。我们需要 JWT token，可以参考 `test/e2e/session_share/session_share_test.go` 里的 `mustE2EEnv` helper：它读取 `JWT_TOKEN` + `SESSION_ID`。

- [ ] **Step 1: 看现有 e2e helper 模式**

```bash
cat test/e2e/session_share/session_share_test.go | head -80
```

记下 `mustE2EEnv` / HTTP client 超时设置等模板。

- [ ] **Step 2: 创建 `test/e2e/session_detail_perf/session_detail_perf_test.go`**

```go
// Package session_detail_perf_test 验证 session 详情接口性能优化的端到端行为
package session_detail_perf_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const httpTimeout = 30 * time.Second

func mustEnv(t *testing.T) (baseURL, jwtToken string, sessionID uint) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	sessIDStr := os.Getenv("SESSION_ID")
	if baseURL == "" || jwtToken == "" || sessIDStr == "" {
		t.Skip("BASE_URL, JWT_TOKEN and SESSION_ID are required for e2e test")
	}
	var id uint
	if _, err := fmt.Sscanf(sessIDStr, "%d", &id); err != nil || id == 0 {
		t.Skip("SESSION_ID must be a positive integer")
	}
	return baseURL, jwtToken, id
}

func newClient() *http.Client {
	return &http.Client{Timeout: httpTimeout}
}

func doGetJSON(t *testing.T, client *http.Client, url, jwt string, out any) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	rsp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer rsp.Body.Close()
	if out != nil {
		if err := sonic.ConfigDefault.NewDecoder(rsp.Body).Decode(out); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return rsp.StatusCode
}

func TestSessionDetailPerf_GetMetadata_Success(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/metadata?sessionId=%d", baseURL, sessID)
	var rsp struct {
		Error   *struct{ Code int } `json:"error"`
		Session *struct {
			ID           uint   `json:"id"`
			APIKeyName   string `json:"apiKeyName"`
			MessageCount int    `json:"messageCount"`
			ToolCount    int    `json:"toolCount"`
			ShareID      string `json:"shareID"`
		} `json:"session"`
	}
	status := doGetJSON(t, client, url, jwt, &rsp)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if rsp.Error != nil {
		t.Fatalf("biz error: %+v", rsp.Error)
	}
	if rsp.Session == nil {
		t.Fatalf("session is nil")
	}
	if rsp.Session.ID != sessID {
		t.Errorf("session.id = %d, want %d", rsp.Session.ID, sessID)
	}
	if rsp.Session.MessageCount < 0 {
		t.Errorf("messageCount must be >= 0")
	}
}

func TestSessionDetailPerf_ListMessages_Pagination(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/message/list?sessionId=%d&offset=0&limit=10", baseURL, sessID)
	var rsp struct {
		Error    *struct{ Code int } `json:"error"`
		Messages []map[string]any    `json:"messages"`
		PageInfo *struct {
			Offset int   `json:"offset"`
			Limit  int   `json:"limit"`
			Total  int64 `json:"total"`
		} `json:"pageInfo"`
	}
	status := doGetJSON(t, client, url, jwt, &rsp)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if rsp.Error != nil {
		t.Fatalf("biz error: %+v", rsp.Error)
	}
	if rsp.PageInfo == nil {
		t.Fatalf("pageInfo is nil")
	}
	if rsp.PageInfo.Offset != 0 || rsp.PageInfo.Limit != 10 {
		t.Errorf("pageInfo.offset=%d limit=%d, want 0/10", rsp.PageInfo.Offset, rsp.PageInfo.Limit)
	}
	if int64(len(rsp.Messages)) > rsp.PageInfo.Total {
		t.Errorf("messages len %d > total %d", len(rsp.Messages), rsp.PageInfo.Total)
	}
}

func TestSessionDetailPerf_ListMessages_LimitRejected(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/message/list?sessionId=%d&offset=0&limit=999", baseURL, sessID)
	status := doGetJSON(t, client, url, jwt, nil)
	if status == http.StatusOK {
		t.Errorf("expected 4xx for limit=999, got 200")
	}
}

func TestSessionDetailPerf_ListTools_Pagination(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/tool/list?sessionId=%d&offset=0&limit=10", baseURL, sessID)
	var rsp struct {
		Error    *struct{ Code int } `json:"error"`
		Tools    []map[string]any    `json:"tools"`
		PageInfo *struct {
			Offset int   `json:"offset"`
			Limit  int   `json:"limit"`
			Total  int64 `json:"total"`
		} `json:"pageInfo"`
	}
	status := doGetJSON(t, client, url, jwt, &rsp)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if rsp.Error != nil {
		t.Fatalf("biz error: %+v", rsp.Error)
	}
	if rsp.PageInfo == nil {
		t.Fatalf("pageInfo is nil")
	}
}

func TestSessionDetailPerf_CacheConsistency(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/metadata?sessionId=%d", baseURL, sessID)
	var first, second struct {
		Session *struct {
			ID           uint   `json:"id"`
			MessageCount int    `json:"messageCount"`
			ToolCount    int    `json:"toolCount"`
			APIKeyName   string `json:"apiKeyName"`
		} `json:"session"`
	}
	if status := doGetJSON(t, client, url, jwt, &first); status != http.StatusOK {
		t.Fatalf("first status = %d", status)
	}
	if status := doGetJSON(t, client, url, jwt, &second); status != http.StatusOK {
		t.Fatalf("second status = %d", status)
	}
	if first.Session == nil || second.Session == nil {
		t.Fatalf("nil session")
	}
	if first.Session.MessageCount != second.Session.MessageCount {
		t.Errorf("messageCount inconsistent: first=%d second=%d", first.Session.MessageCount, second.Session.MessageCount)
	}
	if first.Session.APIKeyName != second.Session.APIKeyName {
		t.Errorf("apiKeyName inconsistent")
	}
}
```

- [ ] **Step 3: 本地无 BASE_URL 时验证测试可 skip 不报错**

Run: `go test -v -count=1 ./test/e2e/session_detail_perf/`
Expected: 5 个测试全部 SKIP（因为没设置 BASE_URL/JWT_TOKEN/SESSION_ID），exit code 0

- [ ] **Step 4: Commit**

```bash
git add test/e2e/session_detail_perf/
git commit -m "test(session-detail-perf): 新增 E2E 测试覆盖 metadata/message-list/tool-list 接口"
```

注意：真正的 E2E 验证要在部署到测试环境之后执行，参见 spec §6 Step 5。

---

## Task 13: 前端 — 扩展类型 + API client

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api-client.ts`

- [ ] **Step 1: 在 `web/src/lib/types.ts` 末尾追加新类型**

定位到 Session 块（约第 78-143 行的 `// ─── Session ───`），在 `GetSessionRsp` 之后追加：

```ts
// ─── Session Detail Perf (新增) ────────────────────────────────────────────────

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

export interface GetSessionMetadataRsp extends CommonRsp {
  session?: SessionMetadata;
}

export interface OffsetPageInfo {
  offset: number;
  limit: number;
  total: number;
}

export interface ListSessionMessagesRsp extends CommonRsp {
  messages?: MessageItem[];
  pageInfo?: OffsetPageInfo;
}

export interface ListSessionToolsRsp extends CommonRsp {
  tools?: ToolItem[];
  pageInfo?: OffsetPageInfo;
}
```

- [ ] **Step 2: 在 `web/src/lib/api-client.ts` 顶部 import 块追加 5 个类型**

定位到第 1-26 行的 import 块，在原有 `ListSessionsRsp, GetSessionRsp,` 之后追加：

```ts
GetSessionMetadataRsp,
ListSessionMessagesRsp,
ListSessionToolsRsp,
```

- [ ] **Step 3: 在 `// ─── Session (JWT auth) ───` 块（约 156-171 行）末尾追加 3 个方法**

在 `getSession` 方法之后插入：

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

- [ ] **Step 4: 类型检查**

```bash
cd web
npx tsc --noEmit
npm run lint
```

Expected: 无错误

- [ ] **Step 5: Commit**

```bash
cd ..  # 回到 worktree 根
git add web/src/lib/types.ts web/src/lib/api-client.ts
git commit -m "feat(session-detail-perf): 前端新增 SessionMetadata/OffsetPageInfo 类型与 3 个 API 方法"
```

---

## Task 14: 前端 — 新增 useInfiniteList hook

**Files:**
- Create: `web/src/hooks/use-infinite-list.ts`

- [ ] **Step 1: 创建文件**

```ts
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export interface UseInfiniteListOptions<T> {
  fetcher: (offset: number, limit: number) => Promise<{ items: T[]; total: number }>;
  pageSize: number;
  enabled: boolean;
}

export interface UseInfiniteListResult<T> {
  items: T[];
  total: number;
  loading: boolean;
  hasMore: boolean;
  loadMore: () => Promise<void>;
  reset: () => void;
}

/**
 * 通用"向下滚动加载更多"hook，复用于 messages / tools 两个列表。
 *
 * 行为契约：
 * - enabled=false 时不发起任何请求
 * - reset() 清空 state 并重新从 offset=0 拉首页
 * - loadMore() 内部用 inFlight ref 保证并发安全
 * - 请求失败不抛错，console.warn 后保持现状（与项目现有 try/catch 静默风格一致）
 */
export function useInfiniteList<T>(
  opts: UseInfiniteListOptions<T>
): UseInfiniteListResult<T> {
  const { fetcher, pageSize, enabled } = opts;
  const [items, setItems] = useState<T[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const inFlightRef = useRef(false);
  const offsetRef = useRef(0);

  const loadMore = useCallback(async () => {
    if (!enabled) return;
    if (inFlightRef.current) return;
    if (offsetRef.current > 0 && offsetRef.current >= total) return;

    inFlightRef.current = true;
    setLoading(true);
    try {
      const { items: newItems, total: newTotal } = await fetcher(
        offsetRef.current,
        pageSize
      );
      setItems((prev) => [...prev, ...newItems]);
      setTotal(newTotal);
      offsetRef.current += newItems.length;
    } catch (e) {
      console.warn("[useInfiniteList] load failed", e);
    } finally {
      setLoading(false);
      inFlightRef.current = false;
    }
  }, [enabled, fetcher, pageSize, total]);

  const reset = useCallback(() => {
    setItems([]);
    setTotal(0);
    offsetRef.current = 0;
  }, []);

  // enabled 切换为 true 或刚 reset 后自动拉首页
  /* eslint-disable react-hooks/set-state-in-effect -- intentional initial fetch on enable */
  useEffect(() => {
    if (enabled && items.length === 0 && offsetRef.current === 0) {
      void loadMore();
    }
  }, [enabled, items.length, loadMore]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const hasMore = total === 0 ? items.length === 0 && enabled : offsetRef.current < total;

  return { items, total, loading, hasMore, loadMore, reset };
}
```

- [ ] **Step 2: 类型 + lint 检查**

```bash
cd web
npx tsc --noEmit
npm run lint
cd ..
```
Expected: 无错误

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/use-infinite-list.ts
git commit -m "feat(session-detail-perf): 新增 useInfiniteList hook（通用向下滚动加载）"
```

---

## Task 15: 前端 — 改造 session-detail-client.tsx

**Files:**
- Modify: `web/src/components/session-detail/session-detail-client.tsx`

**改造原则（保留以避免视觉回归）：**
- mobile sticky header / desktop 双栏 / share dialog / "end of conversation" 文案 全部沿用
- 仅替换数据来源：`session?.messages` → `messagesList.items`；`session?.tools` → `toolsList.items`
- 在 messages 列表底部加 sentinel；tools 列表底部同理（仅在 `hasMore` 时挂载）

- [ ] **Step 1: 修改 import 块**

在文件顶部 import 中新增：

```ts
import { useInfiniteList } from "@/hooks/use-infinite-list";
import type {
  SessionMetadata,
  MessageItem,
  ToolItem,
  UnifiedTool,
} from "@/lib/types";
```

并删除原来的 `import type { SessionDetail, ToolItem, UnifiedTool } from "@/lib/types";`（用上面这行替代，注意保留 ToolItem/UnifiedTool）。

- [ ] **Step 2: 替换主组件 state 与数据获取逻辑**

替换 `SessionDetailClient` 函数体顶部（约第 415-465 行），从 `const [session, setSession] = useState<SessionDetail | null>(null);` 到 `const toolResultsByID = useMemo(...)` 这一整段。

替换为：

```tsx
const [metadata, setMetadata] = useState<SessionMetadata | null>(null);
const [loading, setLoading] = useState(true);
const [toolsPanelOpen, setToolsPanelOpen] = useState(true);
const [toolsSheetOpen, setToolsSheetOpen] = useState(false);
const [shareOpen, setShareOpen] = useState(false);
const [headerCompact, setHeaderCompact] = useState(false);
const headerSentinelRef = useRef<HTMLDivElement | null>(null);
const messagesSentinelRef = useRef<HTMLDivElement | null>(null);
const toolsSentinelRef = useRef<HTMLDivElement | null>(null);

const enabled = !!sessionId && !Number.isNaN(sessionId) && metadata !== null;

const messagesList = useInfiniteList<MessageItem>({
  fetcher: useCallback(
    async (offset, limit) => {
      const rsp = await api.listSessionMessages(sessionId, offset, limit);
      return { items: rsp.messages ?? [], total: Number(rsp.pageInfo?.total ?? 0) };
    },
    [sessionId]
  ),
  pageSize: 20,
  enabled,
});

const toolsList = useInfiniteList<ToolItem>({
  fetcher: useCallback(
    async (offset, limit) => {
      const rsp = await api.listSessionTools(sessionId, offset, limit);
      return { items: rsp.tools ?? [], total: Number(rsp.pageInfo?.total ?? 0) };
    },
    [sessionId]
  ),
  pageSize: 50,
  enabled,
});

// header IO（保持不变）
/* eslint-disable react-hooks/set-state-in-effect */
useEffect(() => {
  if (!isMobile) {
    setHeaderCompact(false);
    return;
  }
  const sentinel = headerSentinelRef.current;
  if (!sentinel) return;
  const io = new IntersectionObserver(
    ([entry]) => setHeaderCompact(!entry.isIntersecting),
    { threshold: 0, rootMargin: "0px" },
  );
  io.observe(sentinel);
  return () => io.disconnect();
}, [isMobile, loading, metadata]);
/* eslint-enable react-hooks/set-state-in-effect */

// 拉 metadata
const fetchMetadata = useCallback(async () => {
  if (!sessionId || Number.isNaN(sessionId)) return;
  setLoading(true);
  try {
    const rsp = await api.getSessionMetadata(sessionId);
    if (rsp.session) setMetadata(rsp.session);
  } catch (e) {
    console.warn("[SessionDetail] fetchMetadata failed", e);
  } finally {
    setLoading(false);
  }
}, [sessionId]);

/* eslint-disable react-hooks/set-state-in-effect */
useEffect(() => {
  void fetchMetadata();
}, [fetchMetadata]);
/* eslint-enable react-hooks/set-state-in-effect */

// messages 滚动加载 sentinel
useEffect(() => {
  const sentinel = messagesSentinelRef.current;
  if (!sentinel || !messagesList.hasMore) return;
  const io = new IntersectionObserver(
    (entries) => {
      if (entries[0]?.isIntersecting) {
        void messagesList.loadMore();
      }
    },
    { rootMargin: "200px" },
  );
  io.observe(sentinel);
  return () => io.disconnect();
}, [messagesList.hasMore, messagesList.loadMore]);

// tools 滚动加载 sentinel
useEffect(() => {
  const sentinel = toolsSentinelRef.current;
  if (!sentinel || !toolsList.hasMore) return;
  const io = new IntersectionObserver(
    (entries) => {
      if (entries[0]?.isIntersecting) {
        void toolsList.loadMore();
      }
    },
    { rootMargin: "200px" },
  );
  io.observe(sentinel);
  return () => io.disconnect();
}, [toolsList.hasMore, toolsList.loadMore]);

const messages = messagesList.items;
const tools = toolsList.items;
const toolResultsByID = useMemo(
  () => buildToolResultsByID(messages),
  [messages],
);
```

- [ ] **Step 3: 替换 session 引用**

全文搜索并替换：
- `session?.messages` → `messages`
- `session?.tools` → `tools`
- `session.id` → `metadata.id`（在 mobile/desktop 布局里）
- `session.apiKeyName` → `metadata.apiKeyName`
- `session.shareID` → `metadata.shareID`
- `session.createdAt` → `metadata.createdAt`
- `setSession(rsp.session)` 已在新 fetchMetadata 里处理，旧的 `fetchSession` 整个函数和它的 useEffect 都删除（已在 Step 2 替换）

注意：`if (!session)` 这个条件判断要改为 `if (!metadata)`。

- [ ] **Step 4: 在 messages 列表末尾、"end of conversation" 之前插入 sentinel**

定位到 mobile 布局（约第 685-697 行）和 desktop 布局（约第 817-830 行）的 messages 渲染块。

mobile 末尾：

```tsx
<div className="space-y-6">
  {messages.map((msg, idx) => (
    <ChatMessage
      key={msg.id}
      message={msg}
      index={idx}
      toolResultsByID={toolResultsByID}
    />
  ))}
  {messagesList.hasMore && (
    <div ref={messagesSentinelRef} className="flex justify-center py-3">
      <Skeleton className="h-4 w-32" />
    </div>
  )}
  {!messagesList.hasMore && messages.length > 0 && (
    <div className="pt-3 pb-1 text-center">
      <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground/50">
        end of conversation
      </span>
    </div>
  )}
</div>
```

desktop 同理（替换原"end of conversation"div 块，加 sentinel）。

- [ ] **Step 5: 在 tools 列表末尾插入 sentinel（mobile 抽屉 + desktop 侧边栏）**

mobile 抽屉（约第 731-734 行）的 `tools.map(...)` 之后追加：

```tsx
{toolsList.hasMore && (
  <div ref={toolsSentinelRef} className="flex justify-center py-3">
    <Skeleton className="h-4 w-24" />
  </div>
)}
```

desktop 侧边栏（约第 849-852 行）同样在 `tools.map(...)` 后加上。

注意：mobile 和 desktop 共享 `toolsSentinelRef` —— 只能有一个 IO 在观察。判断当前 layout 是 mobile 还是 desktop 后，sentinel 只在对应分支挂载即可（已通过 `if (isMobile)` 分支天然分隔）。

- [ ] **Step 6: 修复 `messageCount` 显示**

定位到 `const messageCount = messages.filter(...)` 这一行（约第 552 行），替换为：

```tsx
const messageCount = metadata?.messageCount ?? 0;
```

注意：原计算是"过滤掉 tool 角色后的消息数量"——这个语义无法从 `messageCount`（总数）直接得到。妥协方案：直接用 `metadata.messageCount`（实际等于全量条目数，包含 tool messages）。如果产品强调"非 tool 消息数"，需要新增后端字段——本期不做（YAGNI）。

- [ ] **Step 7: 类型 + lint**

```bash
cd web
npx tsc --noEmit
npm run lint
cd ..
```
Expected: 无错误

- [ ] **Step 8: 本地 dev 验证**

```bash
cd web && npm run dev
```

打开 `http://localhost:3000/web/sessions/detail/?id=<某个真实session id>`，验证：
1. 详情页能打开，messageCount/toolCount 立即显示
2. 滚动到底部能加载更多消息
3. mobile 视图下抽屉里 tools 也能滚动加载（如果 toolCount > 50）
4. 切换不同 session id 列表正确重置

- [ ] **Step 9: Commit**

```bash
git add web/src/components/session-detail/session-detail-client.tsx
git commit -m "feat(session-detail-perf): 改造 session-detail-client 接入 metadata + 滚动分页"
```

---

## Task 16: 全量回归 + 推送分支

- [ ] **Step 1: 后端全量测试**

```bash
make build
make lint
go test -count=1 ./...
```
Expected: 全 PASS

- [ ] **Step 2: 前端全量检查**

```bash
cd web
npx tsc --noEmit
npm run lint
npm run build
cd ..
```
Expected: 全 PASS

- [ ] **Step 3: 检查 commit 列表**

```bash
git log --oneline master..HEAD
```
Expected: 看到本次任务对应的 commit 序列（按 task 顺序）

- [ ] **Step 4: 询问用户是否合并/提 MR**

按 CODEBUDDY.md §10 git 分支规范："开发完成后询问用户是否需要提mr或者直接合并到master，禁止擅自操作"。

不要自动 push 或 merge。等用户决策。

---

## 自检清单（计划完成后）

- [x] 每个任务都有可独立 commit 的边界
- [x] 每个 step 都有 expected 输出
- [x] 没有"TBD"/"TODO"/"按需"等占位
- [x] 后端类型签名（`GetSessionMetaByUserHandler`、`SessionMetaView` 等）在使用任务里与定义任务一致
- [x] 前端类型（`SessionMetadata`、`OffsetPageInfo`、`ListSessionMessagesRsp`）与后端 DTO 字段命名一致
- [x] Spec §3.3.1 的 8 步流程在 Task 6 实现里逐步对应
- [x] Spec §3.7 的所有前端章节都有对应任务（13/14/15）
- [x] 与 Spec 的"不在本期范围"清单一致（不做缓存预热、不做 stampede 保护、不做 schema 重构、share 公开页不动）

## 已知风险与备注

1. **Task 9 编译时**：handler 单包可能因 SessionDependencies 字段不匹配 container.go 而无法整体编译，仅 Task 11 完成后才能 `go build ./...` 全绿。任务顺序已考虑到这点。
2. **Task 12 E2E**：本地无 BASE_URL/JWT_TOKEN/SESSION_ID 时所有用例 SKIP（exit 0）。真正验证在部署到测试环境之后做（spec §6 Step 5）。
3. **Task 15 messageCount 语义变化**：原计算是"过滤 tool 后的消息数"，新值是"全量条目数"。如果产品对此敏感，需要单独提需求扩字段。

