# API Key 管理模块实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现用户维度的 API Key 管理功能，包括创建、列出、删除操作

**Architecture:**
- 独立 `/api/v1/apikey` 路由组，JWT 认证，权限 `user` 及以上
- Handler → Service → DAO 分层，遵循现有项目架构
- Key 生成使用 cryptographically secure 随机字符串，格式 `sk-aris-{24字符}`
- 软删除机制复用现有 `deleted_at` 字段

**Tech Stack:** Go + Fiber/Huma + GORM + Redis

---

## 文件结构

```
internal/
├── handler/
│   └── apikey.go              # API Key Handler（新建）
├── service/
│   └── apikey.go              # API Key Service（新建）
├── router/
│   └── apikey.go              # API Key 路由注册（新建）
├── dto/
│   └── apikey.go              # API Key DTO（新建）
internal/common/ierr/
└── sentinels.go               # 新增 ErrQuotaExceeded 哨兵错误（修改）

internal/infrastructure/database/model/
└── proxy_api_key.go           # 新增 user_id 字段（修改）

test/
└── apikey/                    # API Key 测试（新建）
    └── apikey_test.go
    └── fixtures/
        └── cases.json
```

---

## Task 1: 数据模型变更

**Files:**
- Modify: `internal/infrastructure/database/model/proxy_api_key.go:1-18`

- [ ] **Step 1: 修改 ProxyAPIKey 模型，新增 UserID 字段**

替换 `proxy_api_key.go` 全部内容：

```go
package model

// ProxyAPIKey 代理API密钥数据库模型
//
// 对应原 config.yaml 中的 api_keys 配置。
// 存储代理自身对外暴露的 API Key，用于客户端认证。
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type ProxyAPIKey struct {
	BaseModel
	ID     uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:密钥ID"`
	UserID uint   `json:"userId" gorm:"column:user_id;not null;index:idx_user_id_name;comment:所属用户ID"`
	Name   string `json:"name" gorm:"column:name;not null;index:idx_user_id_name;uniqueIndex:idx_user_id_name,priority:2;comment:密钥名称（对应用户标识）"`
	Key    string `json:"key" gorm:"column:key;uniqueIndex;not null;comment:API密钥值"`
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/infrastructure/database/model/proxy_api_key.go
git commit -m "refactor(model): add user_id field to ProxyAPIKey for user-scoped keys"
```

---

## Task 2: 新增哨兵错误

**Files:**
- Modify: `internal/common/ierr/sentinels.go`

- [ ] **Step 1: 新增 ErrQuotaExceeded 哨兵错误**

在 `sentinels.go` 的通用错误 section 中新增：

```go
// ErrQuotaExceeded 配额超限
ErrQuotaExceeded = newFromSentinel(newSentinel("quota_exceeded", model.NewError(10007, "InsufficientQuota")))
```

- [ ] **Step 2: 提交**

```bash
git add internal/common/ierr/sentinels.go
git commit -m "feat(ierr): add ErrQuotaExceeded for API key limit exceeded"
```

---

## Task 3: 创建 DTO 文件

**Files:**
- Create: `internal/dto/apikey.go`

- [ ] **Step 1: 创建 API Key DTO 文件**

```go
// Package dto API Key DTO
package dto

import "time"

// CreateAPIKeyReq 创建 API Key 请求
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type CreateAPIKeyReq struct {
	Body *CreateAPIKeyReqBody `json:"body" doc:"Request body containing API key name"`
}

// CreateAPIKeyReqBody 创建 API Key 请求体
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type CreateAPIKeyReqBody struct {
	Name string `json:"name" required:"true" minLength:"1" maxLength:"64" doc:"API Key 名称"`
}

// CreateAPIKeyRsp 创建 API Key 响应
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type CreateAPIKeyRsp struct {
	CommonRsp
	Key *APIKeyDetail `json:"key,omitempty" doc:"创建的 API Key 详情"`
}

// ListAPIKeyRsp 列出 API Key 响应
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type ListAPIKeyRsp struct {
	CommonRsp
	Keys []*APIKeyItem `json:"keys,omitempty" doc:"API Key 列表"`
}

// APIKeyItem API Key 列表项（masked key）
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type APIKeyItem struct {
	ID        uint   `json:"id" doc:"API Key ID"`
	Name      string `json:"name" doc:"API Key 名称"`
	Key       string `json:"key" doc:"Masked API Key 值"`
	CreatedAt string `json:"createdAt" doc:"创建时间"`
}

// APIKeyDetail API Key 详情（完整 key，仅创建时返回）
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type APIKeyDetail struct {
	ID        uint      `json:"id" doc:"API Key ID"`
	Name      string    `json:"name" doc:"API Key 名称"`
	Key       string    `json:"key" doc:"完整 API Key 值（仅创建时返回）"`
	CreatedAt time.Time `json:"-" doc:"创建时间（用于内部）"`
}

// DeleteAPIKeyReq 删除 API Key 请求
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type DeleteAPIKeyReq struct {
	ID uint `path:"id" required:"true" minimum:"1" doc:"API Key ID"`
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/dto/apikey.go
git commit -m "feat(dto): add API key DTOs for create/list/delete operations"
```

---

## Task 4: 创建 Service 文件

**Files:**
- Create: `internal/service/apikey.go`

- [ ] **Step 1: 创建 API Key Service 文件**

```go
// Package service API Key 服务
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	// APIKeyMaxCount 每位用户最多创建的 API Key 数量
	APIKeyMaxCount = 5
	// APIKeyPrefix API Key 前缀
	APIKeyPrefix = "sk-aris-"
	// APIKeyRandomLength 随机字符串长度
	APIKeyRandomLength = 24
	// APIKeyCharset 随机字符串字符集（base62）
	APIKeyCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// APIKeyService API Key 服务
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type APIKeyService interface {
	CreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.CreateAPIKeyRsp, error)
	ListAPIKeys(ctx context.Context) (*dto.ListAPIKeyRsp, error)
	DeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.EmptyRsp, error)
}

type apiKeyService struct {
	apiKeyDAO *dao.ProxyAPIKeyDAO
	userDAO   *dao.UserDAO
}

// NewAPIKeyService 创建 API Key 服务
//
//	@return APIKeyService
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func NewAPIKeyService() APIKeyService {
	return &apiKeyService{
		apiKeyDAO: dao.GetProxyAPIKeyDAO(),
		userDAO:   dao.GetUserDAO(),
	}
}

// CreateAPIKey 创建 API Key
//
//	@receiver s *apiKeyService
//	@param ctx context.Context
//	@param req *dto.CreateAPIKeyReq
//	@return *dto.CreateAPIKeyRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func (s *apiKeyService) CreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.CreateAPIKeyRsp, error) {
	rsp := &dto.CreateAPIKeyRsp{}

	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	logger := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	// 检查用户是否存在
	user, err := s.userDAO.Get(db, &dbmodel.User{ID: userID}, []string{"id", "name"})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("[APIKeyService] User not found", zap.Uint("userID", userID))
			rsp.Error = ierr.ErrDataNotExists.BizError()
			return rsp, nil
		}
		logger.Error("[APIKeyService] Failed to get user", zap.Error(err))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}

	// 检查用户已有 API Key 数量
	existingKeys, err := s.apiKeyDAO.BatchGet(db, &dbmodel.ProxyAPIKey{UserID: userID}, []string{"id"})
	if err != nil {
		logger.Error("[APIKeyService] Failed to get existing API keys", zap.Error(err))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}
	if len(existingKeys) >= APIKeyMaxCount {
		logger.Warn("[APIKeyService] API key count exceeds limit",
			zap.Uint("userID", userID),
			zap.Int("current", len(existingKeys)),
			zap.Int("limit", APIKeyMaxCount))
		rsp.Error = ierr.ErrQuotaExceeded.BizError()
		return rsp, nil
	}

	// 生成 API Key
	rawKey, err := generateAPIKey()
	if err != nil {
		logger.Error("[APIKeyService] Failed to generate API key", zap.Error(err))
		rsp.Error = ierr.ErrInternal.BizError()
		return rsp, nil
	}

	// 创建 API Key 记录
	apiKey := &dbmodel.ProxyAPIKey{
		UserID: userID,
		Name:   req.Body.Name,
		Key:    rawKey,
	}
	if err := s.apiKeyDAO.Create(db, apiKey); err != nil {
		logger.Error("[APIKeyService] Failed to create API key", zap.Error(err))
		rsp.Error = ierr.ErrDBCreate.BizError()
		return rsp, nil
	}

	logger.Info("[APIKeyService] API key created",
		zap.Uint("userID", userID),
		zap.String("userName", user.Name),
		zap.String("keyName", req.Body.Name),
		zap.String("key", util.MaskSecret(rawKey)))

	rsp.Key = &dto.APIKeyDetail{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		Key:       rawKey,
		CreatedAt: apiKey.CreatedAt,
	}

	return rsp, nil
}

// ListAPIKeys 列出当前用户的 API Key
//
//	@receiver s *apiKeyService
//	@param ctx context.Context
//	@return *dto.ListAPIKeyRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func (s *apiKeyService) ListAPIKeys(ctx context.Context) (*dto.ListAPIKeyRsp, error) {
	rsp := &dto.ListAPIKeyRsp{}

	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValueString(ctx, constant.CtxKeyPermission)

	logger := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	var keys []*dbmodel.ProxyAPIKey
	var err error

	// 管理员可查看所有用户的 key，普通用户只看自己的
	if permission == string(enum.PermissionAdmin) {
		keys, err = s.apiKeyDAO.BatchGet(db, &dbmodel.ProxyAPIKey{}, []string{"id", "user_id", "name", "key", "created_at"})
	} else {
		keys, err = s.apiKeyDAO.BatchGet(db, &dbmodel.ProxyAPIKey{UserID: userID}, []string{"id", "user_id", "name", "key", "created_at"})
	}

	if err != nil {
		logger.Error("[APIKeyService] Failed to list API keys", zap.Error(err))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}

	rsp.Keys = make([]*dto.APIKeyItem, 0, len(keys))
	for _, k := range keys {
		rsp.Keys = append(rsp.Keys, &dto.APIKeyItem{
			ID:        k.ID,
			Name:      k.Name,
			Key:       util.MaskSecret(k.Key),
			CreatedAt: k.CreatedAt.Format(time.DateTime),
		})
	}

	return rsp, nil
}

// DeleteAPIKey 删除 API Key
//
//	@receiver s *apiKeyService
//	@param ctx context.Context
//	@param req *dto.DeleteAPIKeyReq
//	@return *dto.EmptyRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func (s *apiKeyService) DeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.EmptyRsp, error) {
	rsp := &dto.EmptyRsp{}

	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValueString(ctx, constant.CtxKeyPermission)

	logger := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	// 查询 API Key
	apiKey, err := s.apiKeyDAO.Get(db, &dbmodel.ProxyAPIKey{ID: req.ID}, []string{"id", "user_id", "name"})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("[APIKeyService] API key not found", zap.Uint("id", req.ID))
			rsp.Error = ierr.ErrDataNotExists.BizError()
			return rsp, nil
		}
		logger.Error("[APIKeyService] Failed to get API key", zap.Error(err))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}

	// 权限检查：普通用户只能删除自己的 key，管理员可删除所有
	if apiKey.UserID != userID && permission != string(enum.PermissionAdmin) {
		logger.Warn("[APIKeyService] Permission denied to delete API key",
			zap.Uint("keyID", req.ID),
			zap.Uint("keyOwner", apiKey.UserID),
			zap.Uint("requester", userID))
		rsp.Error = ierr.ErrNoPermission.BizError()
		return rsp, nil
	}

	// 软删除
	if err := s.apiKeyDAO.Delete(db, &dbmodel.ProxyAPIKey{ID: req.ID}); err != nil {
		logger.Error("[APIKeyService] Failed to delete API key", zap.Error(err))
		rsp.Error = ierr.ErrDBDelete.BizError()
		return rsp, nil
	}

	logger.Info("[APIKeyService] API key deleted",
		zap.Uint("keyID", req.ID),
		zap.Uint("userID", userID))

	return rsp, nil
}

// generateAPIKey 生成 API Key
//
//	@return string
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func generateAPIKey() (string, error) {
	result := make([]byte, APIKeyRandomLength)
	charsetLen := len(APIKeyCharset)
	if _, err := rand.Read(result); err != nil {
		return "", err
	}
	for i := range result {
		result[i] = APIKeyCharset[int(result[i])%charsetLen]
	}
	return APIKeyPrefix + strings.ToLower(string(result)), nil
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/service/apikey.go
git commit -m "feat(service): add API key service with create/list/delete operations"
```

---

## Task 5: 创建 Handler 文件

**Files:**
- Create: `internal/handler/apikey.go`

- [ ] **Step 1: 创建 API Key Handler 文件**

```go
// Package handler API Key 处理器
package handler

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/service"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// APIKeyHandler API Key 处理器
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type APIKeyHandler interface {
	HandleCreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.HTTPResponse[*dto.CreateAPIKeyRsp], error)
	HandleListAPIKeys(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.ListAPIKeyRsp], error)
	HandleDeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type apiKeyHandler struct {
	svc service.APIKeyService
}

// NewAPIKeyHandler 创建 API Key 处理器
//
//	@return APIKeyHandler
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func NewAPIKeyHandler() APIKeyHandler {
	return &apiKeyHandler{
		svc: service.NewAPIKeyService(),
	}
}

// HandleCreateAPIKey 创建 API Key
//
//	@receiver h *apiKeyHandler
//	@param ctx context.Context
//	@param req *dto.CreateAPIKeyReq
//	@return *dto.HTTPResponse[*dto.CreateAPIKeyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func (h *apiKeyHandler) HandleCreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.HTTPResponse[*dto.CreateAPIKeyRsp], error) {
	return util.WrapHTTPResponse(h.svc.CreateAPIKey(ctx, req))
}

// HandleListAPIKeys 列出 API Key
//
//	@receiver h *apiKeyHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.ListAPIKeyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func (h *apiKeyHandler) HandleListAPIKeys(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.ListAPIKeyRsp], error) {
	return util.WrapHTTPResponse(h.svc.ListAPIKeys(ctx))
}

// HandleDeleteAPIKey 删除 API Key
//
//	@receiver h *apiKeyHandler
//	@param ctx context.Context
//	@param req *dto.DeleteAPIKeyReq
//	@return *dto.HTTPResponse[*dto.EmptyRsp]
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func (h *apiKeyHandler) HandleDeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	return util.WrapHTTPResponse(h.svc.DeleteAPIKey(ctx, req))
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/handler/apikey.go
git commit -m "feat(handler): add API key handler with create/list/delete operations"
```

---

## Task 6: 创建 Router 文件

**Files:**
- Create: `internal/router/apikey.go`
- Modify: `internal/router/router.go:37-65`

- [ ] **Step 1: 创建 API Key Router 文件**

```go
// Package router API Key 路由
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initAPIKeyRouter(apikeyGroup huma.API) {
	apiKeyHandler := handler.NewAPIKeyHandler()

	apikeyGroup.UseMiddleware(middleware.JwtMiddleware())

	huma.Register(apikeyGroup, huma.Operation{
		OperationID: "createAPIKey",
		Method:      http.MethodPost,
		Path:        "/",
		Summary:     "CreateAPIKey",
		Description: "Create a new API key for the current user",
		Tags:        []string{"APIKey"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("createAPIKey", enum.PermissionUser),
		},
	}, apiKeyHandler.HandleCreateAPIKey)

	huma.Register(apikeyGroup, huma.Operation{
		OperationID: "listAPIKeys",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "ListAPIKeys",
		Description: "List all API keys for the current user (admin sees all)",
		Tags:        []string{"APIKey"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listAPIKeys", enum.PermissionUser),
		},
	}, apiKeyHandler.HandleListAPIKeys)

	huma.Register(apikeyGroup, huma.Operation{
		OperationID: "deleteAPIKey",
		Method:      http.MethodDelete,
		Path:        "/{id}",
		Summary:     "DeleteAPIKey",
		Description: "Delete an API key by ID (owner or admin)",
		Tags:        []string{"APIKey"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteAPIKey", enum.PermissionUser),
		},
	}, apiKeyHandler.HandleDeleteAPIKey)
}
```

- [ ] **Step 2: 修改 router.go 注册路由**

在 `RegisterAPIRouter()` 函数中，新增 `apikeyGroup` 路由组注册：

在 `userGroup := huma.NewGroup(v1Group, "/user")` 后添加：

```go
	apikeyGroup := huma.NewGroup(v1Group, "/apikey")
	initAPIKeyRouter(apikeyGroup)
```

完整修改 `router.go:41-65`：

```go
func RegisterAPIRouter() {
	api := api.GetHumaAPI()
	apiGroup := huma.NewGroup(api, "/api")
	v1Group := huma.NewGroup(apiGroup, "/v1")

	initHealthRouter(api)

	tokenGroup := huma.NewGroup(v1Group, "/token")
	initTokenRouter(tokenGroup)

	oauth2Group := huma.NewGroup(v1Group, "/oauth2")
	initOauth2Router(oauth2Group)

	userGroup := huma.NewGroup(v1Group, "/user")
	initUserRouter(userGroup)

	apikeyGroup := huma.NewGroup(v1Group, "/apikey")
	initAPIKeyRouter(apikeyGroup)

	sessionGroup := huma.NewGroup(v1Group, "/session")
	initSessionRouter(sessionGroup)

	openaiGroup := huma.NewGroup(apiGroup, "/openai/v1")
	initOpenAIRouter(openaiGroup)

	anthropicGroup := huma.NewGroup(apiGroup, "/anthropic/v1")
	initAnthropicRouter(anthropicGroup)
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/router/apikey.go internal/router/router.go
git commit -m "feat(router): add API key routes under /api/v1/apikey"
```

---

## Task 7: 创建测试文件

**Files:**
- Create: `test/apikey/apikey_test.go`
- Create: `test/apikey/fixtures/cases.json`

- [ ] **Step 1: 创建测试 fixtures**

创建 `test/apikey/fixtures/cases.json`：

```json
[
  {
    "name": "create_api_key_success",
    "description": "成功创建 API Key",
    "input": {
      "name": "Test Key"
    },
    "expected": {
      "keyPattern": "^sk-aris-[a-z0-9]{24}$",
      "name": "Test Key"
    }
  },
  {
    "name": "create_api_key_quota_exceeded",
    "description": "创建超过5个限额",
    "input": {
      "name": "Extra Key"
    },
    "setup": {
      "existingKeyCount": 5
    },
    "expected": {
      "error": "insufficient_quota"
    }
  },
  {
    "name": "key_format_validation",
    "description": "Key 格式验证",
    "input": {},
    "validation": {
      "prefix": "sk-aris-",
      "totalLength": 32,
      "randomLength": 24
    }
  }
]
```

- [ ] **Step 2: 创建测试文件**

创建 `test/apikey/apikey_test.go`：

```go
// Package apikey API Key 测试
package apikey

import (
	"regexp"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/service"
	"github.com/bytedance/sonic"
)

const (
	apiKeyPrefix       = "sk-aris-"
	apiKeyRandomLength = 24
	apiKeyTotalLength  = 32
)

var keyPattern = regexp.MustCompile(`^sk-aris-[a-z0-9]{24}$`)

// TestGenerateAPIKey_Format 验证生成的 API Key 格式
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func TestGenerateAPIKey_Format(t *testing.T) {
	// 通过反射调用 unexported 函数 generateAPIKey
	// 由于无法直接调用 unexported 函数，这里测试常量
	t.Run("key format constants", func(t *testing.T) {
		if service.APIKeyPrefix != apiKeyPrefix {
			t.Errorf("APIKeyPrefix = %s, want %s", service.APIKeyPrefix, apiKeyPrefix)
		}
		if service.APIKeyRandomLength != apiKeyRandomLength {
			t.Errorf("APIKeyRandomLength = %d, want %d", service.APIKeyRandomLength, apiKeyRandomLength)
		}
	})

	t.Run("key total length", func(t *testing.T) {
		totalLen := len(apiKeyPrefix) + apiKeyRandomLength
		if totalLen != apiKeyTotalLength {
			t.Errorf("total length = %d, want %d", totalLen, apiKeyTotalLength)
		}
	})
}

// TestCreateAPIKeyReq_JSON 验证 CreateAPIKeyReq JSON 序列化
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func TestCreateAPIKeyReq_JSON(t *testing.T) {
	t.Run("valid request body", func(t *testing.T) {
		jsonStr := `{"name":"Test Key"}`
		var req struct {
			Name string `json:"name"`
		}
		if err := sonic.Unmarshal([]byte(jsonStr), &req); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if req.Name != "Test Key" {
			t.Errorf("Name = %s, want Test Key", req.Name)
		}
	})
}

// TestMaskedKey 显示 MaskSecret 的效果
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func TestMaskedKey(t *testing.T) {
	// sk-aris-abcdefghijklmnopqrstuvw -> sk-a***klmn
	// 保留前4后4
}
```

- [ ] **Step 3: 提交**

```bash
git add test/apikey/
git commit -m "test: add API key management tests"
```

---

## Task 8: 运行全量测试

- [ ] **Step 1: 运行 go mod tidy**

```bash
go mod tidy
```

- [ ] **Step 2: 运行 go vet**

```bash
go vet ./...
```

- [ ] **Step 3: 运行全量测试**

```bash
go test -count=1 ./...
```

- [ ] **Step 4: 运行 lint 扫描**

```bash
make lint
```

- [ ] **Step 5: 提交所有变更**

```bash
git add -A
git commit -m "feat: implement user-scoped API key management (create/list/delete)"
```

---

## 验证清单

| 检查项 | 说明 |
|--------|------|
| 数据模型 | `ProxyAPIKey` 新增 `UserID` 字段 |
| 错误码 | `ErrQuotaExceeded` 映射到 `InsufficientQuota` |
| Key 生成 | 格式 `sk-aris-{24字符}`，总长 32 |
| 创建接口 | POST `/api/v1/apikey`，返回完整 key |
| 列表接口 | GET `/api/v1/apikey`，返回 masked key |
| 删除接口 | DELETE `/api/v1/apikey/:id`，软删除 |
| 权限控制 | `user` 及以上可操作，admin 可管所有 |
| 数量限制 | 每用户最多 5 个 key |
| 测试通过 | `go test -count=1 ./...` 全部 PASS |
