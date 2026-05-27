# Session Share 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现会话分享功能，允许已登录用户将会话内容通过 UUID 链接分享给未登录用户查看。

**Architecture:** 使用 Redis 存储分享链接映射（`share:{uuid} → sessionID`）和用户索引（`user_shares:{userID} → SortedSet`），24 小时自动过期。公开访问接口通过 IP 限流（30 次/分钟）保护，管理接口通过 JWT 认证保护。Handler 层编排 ShareCache 和 GetSessionByUserHandler 调用。

**Tech Stack:** Go 1.25.1, Redis (go-redis/v9), Huma v2, Fiber v3, google/uuid

**Design Spec:** `docs/superpowers/specs/2026-05-28-session-share-design.md`

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/common/constant/rediskey.go` | 新增 ShareKeyTemplate、UserSharesKeyTemplate |
| Modify | `internal/common/constant/ratelimit.go` | 新增分享限流常量 |
| Create | `internal/dto/session_share.go` | 分享相关 DTO 定义 |
| Create | `internal/infrastructure/cache/share.go` | 分享缓存操作实现 |
| Modify | `internal/handler/session.go` | 新增分享 Handler 方法 |
| Modify | `internal/router/session.go` | 新增分享路由注册（含公开路由） |
| Modify | `internal/router/router.go` | 注册公开路由 |
| Modify | `internal/bootstrap/container.go` | 注入 ShareCache 依赖 |
| Create | `test/unit/session_share/session_share_test.go` | 单元测试骨架 |

---

### Task 1: 新增常量定义

**Files:**
- Modify: `internal/common/constant/rediskey.go`
- Modify: `internal/common/constant/ratelimit.go`

- [ ] **Step 1: 在 rediskey.go 新增 ShareKeyTemplate 和 UserSharesKeyTemplate**

在 `rediskey.go` 的 const 块末尾新增两行：

```go
// internal/common/constant/rediskey.go
const (
	LockKeyTemplateMiddleware = "%s:%s:%v"
	JWTUserCacheKeyTemplate   = "jwt:user:%d"
	TokenBucketKeyTemplate    = "tb:%s:%s:%v"
	ScannerBanKeyTemplate     = "scanner:ban:%s"
	ScannerStrikeKeyTemplate  = "scanner:strike:%s"
	ShareKeyTemplate          = "share:%s"
	UserSharesKeyTemplate     = "user_shares:%d"
)
```

- [ ] **Step 2: 在 ratelimit.go 新增分享限流常量**

在 `ratelimit.go` 的 const 块末尾新增两行：

```go
// internal/common/constant/ratelimit.go
const (
	PeriodCallProxyLLM = 1 * time.Second
	LimitCallProxyLLM  = 100

	PeriodRefreshToken = 1 * time.Minute
	LimitRefreshToken  = 10

	RateLimitKeyByIP = "ip"

	PeriodGetShareContent = 1 * time.Minute
	LimitGetShareContent  = 30
)
```

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/common/constant/`
Expected: 编译成功，无错误

- [ ] **Step 4: Commit**

```bash
git add internal/common/constant/rediskey.go internal/common/constant/ratelimit.go
git commit -m "feat: add share key template and rate limit constants"
```

---

### Task 2: 新增 DTO 定义

**Files:**
- Create: `internal/dto/session_share.go`

- [ ] **Step 1: 创建 session_share.go**

```go
// Package dto Session Share DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// CreateShareReq 创建分享请求
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type CreateShareReq struct {
	SessionID uint `json:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
}

// CreateShareRsp 创建分享响应
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type CreateShareRsp struct {
	CommonRsp
	ShareID   string    `json:"shareId" doc:"分享ID (UUID)"`
	ExpiresAt time.Time `json:"expiresAt" doc:"过期时间"`
}

// GetShareContentReq 获取分享内容请求
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type GetShareContentReq struct {
	ShareID string `path:"id" required:"true" doc:"分享ID (UUID)"`
}

// GetShareContentRsp 获取分享内容响应
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type GetShareContentRsp struct {
	CommonRsp
	Session *SessionDetail `json:"session,omitempty" doc:"Session详情"`
}

// ListSharesReq 获取分享列表请求
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ListSharesReq struct {
	model.PageParam
}

// ListSharesRsp 获取分享列表响应
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ListSharesRsp struct {
	CommonRsp
	Shares   []*ShareItem    `json:"shares,omitempty" doc:"分享列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ShareItem 分享列表项
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ShareItem struct {
	ShareID   string    `json:"shareId" doc:"分享ID (UUID)"`
	SessionID uint      `json:"sessionId" doc:"Session ID"`
	CreatedAt time.Time `json:"createdAt" doc:"创建时间"`
	ExpiresAt time.Time `json:"expiresAt" doc:"过期时间"`
}

// DeleteShareReq 删除分享请求
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type DeleteShareReq struct {
	ShareID string `path:"id" required:"true" doc:"分享ID (UUID)"`
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/dto/`
Expected: 编译成功，无错误

- [ ] **Step 3: Commit**

```bash
git add internal/dto/session_share.go
git commit -m "feat: add session share DTO definitions"
```

---

### Task 3: 实现 ShareCache

**Files:**
- Create: `internal/infrastructure/cache/share.go`

**Redis 数据结构设计：**
- `share:{uuid}` → `sessionID`（String，TTL 24h）
- `user_shares:{userID}` → Sorted Set（score=timestamp, member=JSON{shareID, sessionID, createdAt}，TTL 24h）

- [ ] **Step 1: 创建 share.go**

```go
// Package cache 分享缓存操作
package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/redis/go-redis/v9"
)

const (
	shareTTL = 24 * time.Hour
)

// shareRecord 分享记录（存储在 Redis Sorted Set 的 member 中）
type shareRecord struct {
	ShareID   string `json:"shareId"`
	SessionID uint   `json:"sessionId"`
	CreatedAt int64  `json:"createdAt"`
}

// ShareCache 分享缓存操作接口
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ShareCache interface {
	CreateShare(ctx context.Context, userID, sessionID uint) (string, time.Time, error)
	GetShareSessionID(ctx context.Context, shareID string) (uint, error)
	DeleteShare(ctx context.Context, shareID string) error
	ListUserShares(ctx context.Context, userID uint, page, pageSize int) ([]*dto.ShareItem, *model.PageInfo, error)
}

type shareCache struct {
	cache *redis.Client
}

// NewShareCache 创建分享缓存操作实例
//
//	@param cache *redis.Client
//	@return ShareCache
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func NewShareCache(cache *redis.Client) ShareCache {
	return &shareCache{cache: cache}
}

// CreateShare 创建分享链接
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param userID uint
//	@param sessionID uint
//	@return string shareID
//	@return time.Time expiresAt
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (s *shareCache) CreateShare(ctx context.Context, userID, sessionID uint) (string, time.Time, error) {
	shareID := uuid.New().String()
	key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
	userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)
	expiresAt := time.Now().Add(shareTTL)

	record := &shareRecord{
		ShareID:   shareID,
		SessionID: sessionID,
		CreatedAt: time.Now().Unix(),
	}
	recordJSON, err := sonic.Marshal(record)
	if err != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, err, "failed to marshal share record")
	}

	pipe := s.cache.Pipeline()
	pipe.Set(ctx, key, sessionID, shareTTL)
	pipe.ZAdd(ctx, userSharesKey, redis.Z{
		Score:  float64(record.CreatedAt),
		Member: string(recordJSON),
	})
	pipe.Expire(ctx, userSharesKey, shareTTL)

	if _, execErr := pipe.Exec(ctx); execErr != nil {
		return "", time.Time{}, ierr.Wrap(ierr.ErrInternal, execErr, "failed to create share")
	}

	return shareID, expiresAt, nil
}

// GetShareSessionID 获取分享链接对应的 sessionID
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param shareID string
//	@return uint
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (s *shareCache) GetShareSessionID(ctx context.Context, shareID string) (uint, error) {
	key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
	val, err := s.cache.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, ierr.New(ierr.ErrDataNotExists, "share link not found or expired")
		}
		return 0, ierr.Wrap(ierr.ErrInternal, err, "failed to get share")
	}

	sessionID, parseErr := strconv.ParseUint(val, constant.DecimalBase, constant.ParseFloat64BitSize)
	if parseErr != nil {
		return 0, ierr.Wrap(ierr.ErrInternal, parseErr, "failed to parse session ID from share")
	}

	return uint(sessionID), nil
}

// DeleteShare 删除分享链接
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param shareID string
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (s *shareCache) DeleteShare(ctx context.Context, shareID string) error {
	key := fmt.Sprintf(constant.ShareKeyTemplate, shareID)
	err := s.cache.Del(ctx, key).Err()
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "failed to delete share")
	}
	return nil
}

// ListUserShares 获取用户的所有分享链接
//
//	@receiver s *shareCache
//	@param ctx context.Context
//	@param userID uint
//	@param page int
//	@param pageSize int
//	@return []*dto.ShareItem
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (s *shareCache) ListUserShares(ctx context.Context, userID uint, page, pageSize int) ([]*dto.ShareItem, *model.PageInfo, error) {
	userSharesKey := fmt.Sprintf(constant.UserSharesKeyTemplate, userID)

	total, err := s.cache.ZCard(ctx, userSharesKey).Result()
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrInternal, err, "failed to count user shares")
	}

	start := int64((page - 1) * pageSize)
	stop := int64(page*pageSize - 1)

	results, zErr := s.cache.ZRevRange(ctx, userSharesKey, start, stop).Result()
	if zErr != nil {
		return nil, nil, ierr.Wrap(ierr.ErrInternal, zErr, "failed to list user shares")
	}

	items := make([]*dto.ShareItem, 0, len(results))
	for _, result := range results {
		var record shareRecord
		if unmarshalErr := sonic.Unmarshal([]byte(result), &record); unmarshalErr != nil {
			continue
		}

		shareKey := fmt.Sprintf(constant.ShareKeyTemplate, record.ShareID)
		exists, existsErr := s.cache.Exists(ctx, shareKey).Result()
		if existsErr != nil || exists == 0 {
			continue
		}

		ttl, ttlErr := s.cache.TTL(ctx, shareKey).Result()
		expiresAt := time.Time{}
		if ttlErr == nil && ttl > 0 {
			expiresAt = time.Now().Add(ttl)
		}

		items = append(items, &dto.ShareItem{
			ShareID:   record.ShareID,
			SessionID: record.SessionID,
			CreatedAt: time.Unix(record.CreatedAt, 0),
			ExpiresAt: expiresAt,
		})
	}

	pageInfo := &model.PageInfo{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}

	return items, pageInfo, nil
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/infrastructure/cache/`
Expected: 编译成功，无错误

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/cache/share.go
git commit -m "feat: implement ShareCache with Redis storage"
```

---

### Task 4: 更新 SessionHandler

**Files:**
- Modify: `internal/handler/session.go`

- [ ] **Step 1: 更新 import 块**

在 `session.go` 的 import 中新增 `"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"`：

```go
import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)
```

- [ ] **Step 2: 更新 SessionHandler 接口**

在 `SessionHandler` interface 末尾新增 4 个方法：

```go
type SessionHandler interface {
	HandleListSessionsByUser(ctx context.Context, req *dto.ListSessionsByUserReq) (*dto.HTTPResponse[*dto.ListSessionsRsp], error)
	HandleGetSessionByUser(ctx context.Context, req *dto.GetSessionByUserReq) (*dto.HTTPResponse[*dto.GetSessionRsp], error)
	HandleCreateShare(ctx context.Context, req *dto.CreateShareReq) (*dto.HTTPResponse[*dto.CreateShareRsp], error)
	HandleGetShareContent(ctx context.Context, req *dto.GetShareContentReq) (*dto.HTTPResponse[*dto.GetShareContentRsp], error)
	HandleListShares(ctx context.Context, req *dto.ListSharesReq) (*dto.HTTPResponse[*dto.ListSharesRsp], error)
	HandleDeleteShare(ctx context.Context, req *dto.DeleteShareReq) (*dto.HTTPResponse[*dto.CommonRsp], error)
}
```

- [ ] **Step 3: 更新 SessionDependencies 和 sessionHandler struct**

在 `SessionDependencies` struct 新增 `ShareCache` 字段，在 `sessionHandler` struct 新增 `shareCache` 字段：

```go
type SessionDependencies struct {
	ListByUser sessionquery.ListSessionsByUserHandler
	GetByUser  sessionquery.GetSessionByUserHandler
	ShareCache cache.ShareCache
}

type sessionHandler struct {
	listByUser sessionquery.ListSessionsByUserHandler
	getByUser  sessionquery.GetSessionByUserHandler
	shareCache cache.ShareCache
}
```

更新 `NewSessionHandler`：

```go
func NewSessionHandler(deps SessionDependencies) SessionHandler {
	return &sessionHandler{
		listByUser: deps.ListByUser,
		getByUser:  deps.GetByUser,
		shareCache: deps.ShareCache,
	}
}
```

- [ ] **Step 4: 实现 HandleCreateShare**

在 `session.go` 文件末尾新增：

```go
// HandleCreateShare 创建分享链接（JWT认证）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.CreateShareReq
//	@return *dto.HTTPResponse[*dto.CreateShareRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (h *sessionHandler) HandleCreateShare(ctx context.Context, req *dto.CreateShareReq) (*dto.HTTPResponse[*dto.CreateShareRsp], error) {
	rsp := &dto.CreateShareRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)
	isAdmin := permission.Level() >= enum.PermissionAdmin.Level()

	view, err := h.getByUser.Handle(ctx, sessionquery.GetSessionByUserQuery{
		UserID:    userID,
		IsAdmin:   isAdmin,
		SessionID: req.SessionID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Create share: verify session failed",
			zap.Uint("sessionID", req.SessionID), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	if view == nil {
		rsp.Error = ierr.ErrDataNotExists.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	shareID, expiresAt, shareErr := h.shareCache.CreateShare(ctx, userID, req.SessionID)
	if shareErr != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Create share failed",
			zap.Uint("sessionID", req.SessionID), zap.Error(shareErr))
		rsp.Error = ierr.ToBizError(shareErr, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.ShareID = shareID
	rsp.ExpiresAt = expiresAt

	logger.WithCtx(ctx).Info("[SessionHandler] Share created",
		zap.String("shareID", shareID),
		zap.Uint("sessionID", req.SessionID),
		zap.Uint("userID", userID))

	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 5: 实现 HandleGetShareContent**

```go
// HandleGetShareContent 获取分享内容（公开接口，IP限流）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.GetShareContentReq
//	@return *dto.HTTPResponse[*dto.GetShareContentRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (h *sessionHandler) HandleGetShareContent(ctx context.Context, req *dto.GetShareContentReq) (*dto.HTTPResponse[*dto.GetShareContentRsp], error) {
	rsp := &dto.GetShareContentRsp{}

	sessionID, err := h.shareCache.GetShareSessionID(ctx, req.ShareID)
	if err != nil {
		logger.WithCtx(ctx).Warn("[SessionHandler] Get share content: share not found",
			zap.String("shareID", req.ShareID), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrDataNotExists.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	view, viewErr := h.getByUser.Handle(ctx, sessionquery.GetSessionByUserQuery{
		UserID:    0,
		IsAdmin:   true,
		SessionID: sessionID,
	})
	if viewErr != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Get share content: fetch session failed",
			zap.Uint("sessionID", sessionID), zap.Error(viewErr))
		rsp.Error = ierr.ToBizError(viewErr, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	if view == nil {
		rsp.Error = ierr.ErrDataNotExists.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	messageItems := lo.Map(view.Messages, func(m *sessionquery.MessageView, _ int) *dto.MessageItem {
		return &dto.MessageItem{
			ID:        m.ID,
			Model:     m.Model,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		}
	})
	toolItems := lo.Map(view.Tools, func(t *sessionquery.ToolView, _ int) *dto.ToolItem {
		return &dto.ToolItem{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		}
	})

	rsp.Session = &dto.SessionDetail{
		ID:         view.ID,
		APIKeyName: view.APIKeyName,
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
		Metadata:   view.Metadata,
		Messages:   messageItems,
		Tools:      toolItems,
	}

	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 6: 实现 HandleListShares**

```go
// HandleListShares 获取当前用户的分享列表（JWT认证）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.ListSharesReq
//	@return *dto.HTTPResponse[*dto.ListSharesRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (h *sessionHandler) HandleListShares(ctx context.Context, req *dto.ListSharesReq) (*dto.HTTPResponse[*dto.ListSharesRsp], error) {
	rsp := &dto.ListSharesRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	shares, pageInfo, err := h.shareCache.ListUserShares(ctx, userID, req.Page, req.PageSize)
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] List shares failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Shares = shares
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 7: 实现 HandleDeleteShare**

```go
// HandleDeleteShare 取消分享（JWT认证）
//
//	@receiver h *sessionHandler
//	@param ctx context.Context
//	@param req *dto.DeleteShareReq
//	@return *dto.HTTPResponse[*dto.CommonRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-28 10:00:00
func (h *sessionHandler) HandleDeleteShare(ctx context.Context, req *dto.DeleteShareReq) (*dto.HTTPResponse[*dto.CommonRsp], error) {
	rsp := &dto.CommonRsp{}

	err := h.shareCache.DeleteShare(ctx, req.ShareID)
	if err != nil {
		logger.WithCtx(ctx).Error("[SessionHandler] Delete share failed",
			zap.String("shareID", req.ShareID), zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	logger.WithCtx(ctx).Info("[SessionHandler] Share deleted",
		zap.String("shareID", req.ShareID))

	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 8: 编译验证**

Run: `go build ./internal/handler/`
Expected: 编译成功，无错误

- [ ] **Step 9: Commit**

```bash
git add internal/handler/session.go
git commit -m "feat: add share handler methods to SessionHandler"
```

---

### Task 5: 更新路由注册

**Files:**
- Modify: `internal/router/session.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: 重写 session.go 路由**

将 `session.go` 完整替换为以下内容（保留原有的 `initSessionJWTRouter` 函数，新增分享相关路由）：

```go
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func initSessionJWTRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, db *gorm.DB, cache *redis.Client) {
	sessionGroup.UseMiddleware(middleware.JwtMiddleware(db, cache))

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessions",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListSessions",
		Description: "Paginate session list for current user (JWT auth)",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listSessions", enum.PermissionUser)},
	}, sessionHandler.HandleListSessionsByUser)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getSession",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "GetSession",
		Description: "Get session detail by session ID (JWT auth)",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getSession", enum.PermissionUser)},
	}, sessionHandler.HandleGetSessionByUser)

	shareGroup := huma.NewGroup(sessionGroup, "/share")
	initSessionShareRouter(shareGroup, sessionHandler)
}

func initSessionShareRouter(shareGroup huma.API, sessionHandler handler.SessionHandler) {
	huma.Register(shareGroup, huma.Operation{
		OperationID: "createShare",
		Method:      http.MethodPost,
		Path:        "/",
		Summary:     "CreateShare",
		Description: "Create a share link for a session",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
	}, sessionHandler.HandleCreateShare)

	huma.Register(shareGroup, huma.Operation{
		OperationID: "listShares",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListShares",
		Description: "List all share links for current user",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
	}, sessionHandler.HandleListShares)

	huma.Register(shareGroup, huma.Operation{
		OperationID: "deleteShare",
		Method:      http.MethodDelete,
		Path:        "/{id}",
		Summary:     "DeleteShare",
		Description: "Delete a share link",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
	}, sessionHandler.HandleDeleteShare)
}

func initSessionPublicRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, cache *redis.Client) {
	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getShareContent",
		Method:      http.MethodGet,
		Path:        "/share/{id}",
		Summary:     "GetShareContent",
		Description: "Get shared session content (public, rate limited)",
		Tags:        []string{"Session"},
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(cache, "getShareContent", "", constant.PeriodGetShareContent, constant.LimitGetShareContent),
		},
	}, sessionHandler.HandleGetShareContent)
}
```

- [ ] **Step 2: 更新 router.go**

在 `RegisterAPIRouter` 函数中，在 `initSessionJWTRouter` 调用之后新增公开路由注册：

```go
// 在现有 sessionJWTGroup 之后新增
sessionPublicGroup := huma.NewGroup(v1Group, "/session")
initSessionPublicRouter(sessionPublicGroup, deps.SessionHandler, deps.Cache)
```

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/router/`
Expected: 编译成功，无错误

- [ ] **Step 4: Commit**

```bash
git add internal/router/session.go internal/router/router.go
git commit -m "feat: add session share route registration"
```

---

### Task 6: 更新依赖注入

**Files:**
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: 在 provideInfrastructure 中注册 ShareCache**

在 `provideInfrastructure` 函数中，在 `newAudioDirCreator` 之后新增一行：

```go
if err := container.Provide(newShareCache); err != nil {
	return err
}
```

- [ ] **Step 2: 在文件末尾新增 newShareCache 构造函数**

```go
func newShareCache(cache *redis.Client) cache.ShareCache {
	return cache.NewShareCache(cache)
}
```

**注意**: 检查 container.go 的 import 中是否已有 `"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"`。如果没有，需要新增。由于 container.go 已有该 import（在 `provideInfrastructure` 中使用了 `cache.InitCache`），所以不需要额外添加。

- [ ] **Step 3: 更新 newSessionDependencies 函数签名**

将 `newSessionDependencies` 函数改为接收 `shareCache` 参数：

```go
func newSessionDependencies(listByUser sessionquery.ListSessionsByUserHandler, getByUser sessionquery.GetSessionByUserHandler, shareCache cache.ShareCache) handler.SessionDependencies {
	return handler.SessionDependencies{ListByUser: listByUser, GetByUser: getByUser, ShareCache: shareCache}
}
```

- [ ] **Step 4: 全量编译验证**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap/container.go
git commit -m "feat: inject ShareCache into dependency container"
```

---

### Task 7: 全量验证

**Files:**
- None

- [ ] **Step 1: 全量编译**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 2: 运行 lint**

Run: `make lint`
Expected: lint 通过，无新增错误

- [ ] **Step 3: 运行现有测试**

Run: `go test -count=1 ./...`
Expected: 所有现有测试通过，无回归

---

### Task 8: 编写单元测试骨架

**Files:**
- Create: `test/unit/session_share/session_share_test.go`

- [ ] **Step 1: 创建测试目录**

```bash
mkdir -p test/unit/session_share
```

- [ ] **Step 2: 编写单元测试骨架**

```go
package session_share

import (
	"testing"
)

// TestCreateShare 测试创建分享
func TestCreateShare(t *testing.T) {
	t.Run("normal_create", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("session_not_found", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("session_not_owned", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})
}

// TestGetShareContent 测试获取分享内容
func TestGetShareContent(t *testing.T) {
	t.Run("normal_get", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("share_not_found", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("share_expired", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})
}

// TestListShares 测试获取分享列表
func TestListShares(t *testing.T) {
	t.Run("normal_list", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("empty_list", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})
}

// TestDeleteShare 测试取消分享
func TestDeleteShare(t *testing.T) {
	t.Run("normal_delete", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})

	t.Run("share_not_found", func(t *testing.T) {
		t.Skip("requires Redis connection, run in integration environment")
	})
}
```

**说明**: 由于分享功能依赖 Redis，完整单元测试需要 mock 或真实 Redis 连接。此处先创建测试骨架，保证编译通过。

- [ ] **Step 3: 运行测试验证编译**

Run: `go test -v -count=1 ./test/unit/session_share/`
Expected: 所有测试 skip，无编译错误

- [ ] **Step 4: Commit**

```bash
git add test/unit/session_share/
git commit -m "test: add session share unit test skeleton"
```
