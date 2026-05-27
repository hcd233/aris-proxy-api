# Session Share 功能设计文档

**日期**: 2026-05-28
**作者**: Sisyphus
**状态**: 已批准

---

## 1. 需求概述

用户可以将一个会话内容分享给没有登录的人查看。

**核心功能**：
- 用户在会话详情页点击分享按钮，生成一个 UUID，存储到 Redis（24小时过期）
- 用户访问 `web/sessions/share/{uuid}` 时，调用接口获取会话内容
- 分享链接无鉴权，需要基于 IP 限流

**管理功能**：
- 用户可以查看自己创建的所有分享链接
- 用户可以取消已创建的分享链接
- 独立管理页面

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Frontend  │────▶│   Backend   │────▶│    Redis    │
│  (web app)  │◀────│   (API)     │◀────│   (cache)   │
└─────────────┘     └─────────────┘     └─────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ PostgreSQL  │
                    │ (database)  │
                    └─────────────┘
```

### 2.2 数据流

1. **创建分享**：前端调用 API → 后端生成 UUID → 存储到 Redis → 返回 UUID
2. **访问分享**：用户访问链接 → 前端调用 API → 后端从 Redis 获取 session ID → 从数据库获取会话内容 → 返回给前端
3. **管理分享**：前端调用 API → 后端查询用户的分享列表 → 返回给前端

### 2.3 限流策略

- 基于 IP 的令牌桶限流
- 每个 IP 每分钟最多 30 次请求
- 使用现有的 `TokenBucketRateLimiterMiddleware`

---

## 3. 数据结构设计

### 3.1 Redis 数据结构

```go
// Redis Key 格式
const (
    ShareKeyTemplate = "share:%s"  // share:{uuid} -> session_id (uint)
)

// 存储内容
// Key: share:{uuid}
// Value: session_id (uint 的字符串形式)
// TTL: 24 小时
```

### 3.2 DTO 设计

```go
// CreateShareReq 创建分享请求
type CreateShareReq struct {
    SessionID uint `json:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
}

// CreateShareRsp 创建分享响应
type CreateShareRsp struct {
    CommonRsp
    ShareID   string    `json:"shareId" doc:"分享ID (UUID)"`
    ExpiresAt time.Time `json:"expiresAt" doc:"过期时间"`
}

// GetShareContentReq 获取分享内容请求
type GetShareContentReq struct {
    ShareID string `path:"id" required:"true" doc:"分享ID (UUID)"`
}

// GetShareContentRsp 获取分享内容响应
type GetShareContentRsp struct {
    CommonRsp
    Session *SessionDetail `json:"session,omitempty" doc:"Session详情"`
}

// ListSharesReq 获取分享列表请求
type ListSharesReq struct {
    model.PageParam
}

// ListSharesRsp 获取分享列表响应
type ListSharesRsp struct {
    CommonRsp
    Shares   []*ShareItem   `json:"shares,omitempty" doc:"分享列表"`
    PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ShareItem 分享列表项
type ShareItem struct {
    ShareID   string    `json:"shareId" doc:"分享ID (UUID)"`
    SessionID uint      `json:"sessionId" doc:"Session ID"`
    ShareURL  string    `json:"shareUrl" doc:"分享链接"`
    CreatedAt time.Time `json:"createdAt" doc:"创建时间"`
    ExpiresAt time.Time `json:"expiresAt" doc:"过期时间"`
}

// DeleteShareReq 删除分享请求
type DeleteShareReq struct {
    ShareID string `path:"id" required:"true" doc:"分享ID (UUID)"`
}
```

### 3.3 Redis 操作封装

```go
// ShareCache 分享缓存操作接口
type ShareCache interface {
    CreateShare(ctx context.Context, sessionID uint) (string, error)
    GetShareSessionID(ctx context.Context, shareID string) (uint, error)
    DeleteShare(ctx context.Context, shareID string) error
    ListUserShares(ctx context.Context, userID uint) ([]*ShareItem, error)
}
```

---

## 4. API 设计

### 4.1 API 端点

| 操作 | Method | Path | 认证 | 限流 | 说明 |
|------|--------|------|------|------|------|
| 创建分享 | POST | `/api/v1/session/share/` | JWT | 否 | 创建分享链接 |
| 获取分享列表 | GET | `/api/v1/session/share/list` | JWT | 否 | 获取用户的所有分享 |
| 取消分享 | DELETE | `/api/v1/session/share/{id}` | JWT | 否 | 删除分享链接 |
| 获取分享内容 | GET | `/api/v1/session/share/{id}` | 否 | IP 限流 | 公开接口 |

### 4.2 路由注册

```go
// internal/router/router.go
func RegisterAPIRouter(humaAPI huma.API, deps APIRouterDependencies) {
    // ... 现有代码 ...
    
    sessionJWTGroup := huma.NewGroup(v1Group, "/session")
    initSessionJWTRouter(sessionJWTGroup, deps.SessionHandler, deps.DB, deps.Cache)
    
    // 新增：公开的 session 路由（无认证）
    sessionPublicGroup := huma.NewGroup(v1Group, "/session")
    initSessionPublicRouter(sessionPublicGroup, deps.SessionHandler, deps.Cache)
    
    // ... 其他路由 ...
}
```

```go
// internal/router/session.go

// initSessionJWTRouter 初始化 Session JWT 路由
func initSessionJWTRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, db *gorm.DB, cache *redis.Client) {
    sessionGroup.UseMiddleware(middleware.JwtMiddleware(db, cache))
    
    // ... 现有路由 ...
    
    // 分享相关路由（JWT 认证）
    shareGroup := huma.NewGroup(sessionGroup, "/share")
    initSessionShareRouter(shareGroup, sessionHandler)
}

// initSessionShareRouter 初始化 Session 分享路由（JWT 认证）
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

// initSessionPublicRouter 初始化 Session 公开路由（无认证）
func initSessionPublicRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, cache *redis.Client) {
    // 分享内容获取（公开，带限流）
    huma.Register(sessionGroup, huma.Operation{
        OperationID: "getShareContent",
        Method:      http.MethodGet,
        Path:        "/share/{id}",
        Summary:     "GetShareContent",
        Description: "Get shared session content (public, rate limited)",
        Tags:        []string{"Session"},
        Middlewares: huma.Middlewares{
            middleware.TokenBucketRateLimiterMiddleware(cache, "getShareContent", "", 1*time.Minute, 30),
        },
    }, sessionHandler.HandleGetShareContent)
}
```

### 4.3 Handler 接口

```go
// SessionHandler Session处理器
type SessionHandler interface {
    // ... 现有方法 ...
    
    // 分享相关
    HandleCreateShare(ctx context.Context, req *dto.CreateShareReq) (*dto.HTTPResponse[*dto.CreateShareRsp], error)
    HandleGetShareContent(ctx context.Context, req *dto.GetShareContentReq) (*dto.HTTPResponse[*dto.GetShareContentRsp], error)
    HandleListShares(ctx context.Context, req *dto.ListSharesReq) (*dto.HTTPResponse[*dto.ListSharesRsp], error)
    HandleDeleteShare(ctx context.Context, req *dto.DeleteShareReq) (*dto.HTTPResponse[*dto.CommonRsp], error)
}
```

---

## 5. 流程设计

### 5.1 创建分享流程

```
用户点击分享按钮
       │
       ▼
前端调用 POST /api/v1/session/share/
       │
       ▼
后端验证：
  1. 从 JWT 获取 userID
  2. 验证 session 是否属于该用户
  3. 生成 UUID
  4. 存储到 Redis：share:{uuid} -> session_id (TTL 24h)
  5. 返回 UUID
       │
       ▼
前端拼接分享链接：web/sessions/share/{uuid}
       │
       ▼
前端显示分享链接
```

### 5.2 访问分享流程

```
用户访问 web/sessions/share/{uuid}
       │
       ▼
前端调用 GET /api/v1/session/share/{uuid}
       │
       ▼
IP 限流检查（30次/分钟）
       │
       ▼
后端处理：
  1. 从 Redis 获取 session_id
  2. 如果不存在，返回 404
  3. 从数据库获取 session 详情
  4. 返回会话内容
       │
       ▼
前端渲染会话内容
```

### 5.3 取消分享流程

```
用户点击取消分享
       │
       ▼
前端调用 DELETE /api/v1/session/share/{uuid}
       │
       ▼
后端验证：
  1. 从 JWT 获取 userID
  2. 从 Redis 获取 session_id
  3. 验证 session 是否属于该用户
  4. 从 Redis 删除分享链接
  5. 返回成功
       │
       ▼
前端刷新分享列表
```

### 5.4 获取分享列表流程

```
用户访问分享管理页面
       │
       ▼
前端调用 GET /api/v1/session/share/list
       │
       ▼
后端处理：
  1. 从 JWT 获取 userID
  2. 查询该用户的所有分享链接
  3. 返回分享列表
       │
       ▼
前端显示分享列表
```

---

## 6. 错误处理

### 6.1 错误场景

| 场景 | 错误码 | 说明 |
|------|--------|------|
| Session 不存在 | 404 | 分享的 session 不存在 |
| Session 不属于用户 | 403 | 用户尝试分享他人的 session |
| 分享链接不存在 | 404 | 访问的分享链接不存在或已过期 |
| 分享链接已过期 | 404 | 分享链接已过期（Redis TTL 过期） |
| 限流 | 429 | IP 限流触发 |
| 参数错误 | 400 | 请求参数验证失败 |

### 6.2 错误处理实现

```go
// 错误定义
var (
    ErrSessionNotFound = ierr.New(ierr.ErrNotFound, "session not found")
    ErrSessionNotOwned = ierr.New(ierr.ErrForbidden, "session not owned by user")
    ErrShareNotFound   = ierr.New(ierr.ErrNotFound, "share link not found or expired")
    ErrShareExpired    = ierr.New(ierr.ErrNotFound, "share link expired")
)

// Handler 错误处理示例
func (h *sessionHandler) HandleGetShareContent(ctx context.Context, req *dto.GetShareContentReq) (*dto.HTTPResponse[*dto.GetShareContentRsp], error) {
    rsp := &dto.GetShareContentRsp{}
    
    sessionID, err := h.shareCache.GetShareSessionID(ctx, req.ShareID)
    if err != nil {
        if errors.Is(err, redis.Nil) {
            rsp.Error = ierr.ToBizError(ErrShareNotFound, ierr.ErrNotFound.BizError())
            return apiutil.WrapHTTPResponse(rsp, nil)
        }
        logger.WithCtx(ctx).Error("[SessionHandler] Get share session ID failed", zap.Error(err))
        rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
        return apiutil.WrapHTTPResponse(rsp, nil)
    }
    
    // ... 获取 session 详情 ...
}
```

### 6.3 限流错误

```go
// 使用现有的限流中间件，自动返回 429 错误
// 响应头包含：
// - X-RateLimit-Limit: 桶容量
// - X-RateLimit-Remaining: 剩余令牌数
// - Retry-After: 恢复 1 个令牌所需的秒数
```

---

## 7. 测试策略

### 7.1 单元测试

```go
// test/unit/session_share/session_share_test.go
package session_share

import (
    "testing"
    "context"
    "time"
    
    "github.com/hcd233/aris-proxy-api/internal/dto"
    "github.com/hcd233/aris-proxy-api/internal/handler"
    "github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
)

// TestCreateShare 测试创建分享
func TestCreateShare(t *testing.T) {
    // 1. 测试正常创建
    // 2. 测试 session 不存在
    // 3. 测试 session 不属于用户
}

// TestGetShareContent 测试获取分享内容
func TestGetShareContent(t *testing.T) {
    // 1. 测试正常获取
    // 2. 测试分享链接不存在
    // 3. 测试分享链接已过期
}

// TestListShares 测试获取分享列表
func TestListShares(t *testing.T) {
    // 1. 测试正常获取列表
    // 2. 测试空列表
}

// TestDeleteShare 测试取消分享
func TestDeleteShare(t *testing.T) {
    // 1. 测试正常删除
    // 2. 测试分享链接不存在
    // 3. 测试 session 不属于用户
}
```

### 7.2 测试覆盖率目标

- 单元测试：覆盖所有 Handler 方法的正常和异常路径

---

## 8. 实现计划

### 8.1 需要新增的文件

1. **DTO**: `internal/dto/session_share.go`
2. **Cache**: `internal/infrastructure/cache/share.go`
3. **Handler**: 更新 `internal/handler/session.go`
4. **Router**: 更新 `internal/router/session.go` 和 `internal/router/router.go`
5. **Constant**: 更新 `internal/common/constant/rediskey.go`
6. **Test**: `test/unit/session_share/session_share_test.go`

### 8.2 实现顺序

1. 新增 Redis Key 常量
2. 新增 DTO 定义
3. 实现 ShareCache
4. 更新 SessionHandler 接口和实现
5. 更新路由注册
6. 编写单元测试

---

## 9. 附录

### 9.1 相关文件

- `internal/handler/session.go` - Session Handler
- `internal/router/session.go` - Session 路由
- `internal/router/router.go` - 路由注册
- `internal/infrastructure/cache/redis.go` - Redis 客户端
- `internal/middleware/rate.go` - 限流中间件
- `internal/common/constant/rediskey.go` - Redis Key 常量

### 9.2 依赖项

- `github.com/google/uuid` - UUID 生成
- `github.com/redis/go-redis/v9` - Redis 客户端
- `github.com/danielgtaylor/huma/v2` - API 框架
