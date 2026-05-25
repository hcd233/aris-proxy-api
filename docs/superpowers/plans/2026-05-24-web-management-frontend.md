# 管理前端实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 aris-proxy-api 开发管理前端，包括后端 API 补全和 Next.js SPA 前端，嵌入 Go 二进制同源部署。

**Architecture:** 前端 Next.js App Router 静态导出 (SPA)，构建产物复制到 `internal/web/dist/`，通过 Go `embed.FS` 嵌入二进制，Fiber 在 `/web/` 路径提供静态文件服务。后端补充 Endpoint/Model CRUD API、OAuth2 浏览器重定向回调、Session JWT 鉴权路由。

**Tech Stack:** Go 1.25.1 / Fiber v3 / Huma / GORM / dig · Next.js 15 / shadcn-ui / Tailwind CSS / TypeScript

---

## 文件结构总览

### 后端新建文件
- `internal/dto/endpoint.go` — Endpoint DTO
- `internal/dto/model.go` — Model DTO
- `internal/application/endpoint/command/create_endpoint.go` — 创建 Endpoint
- `internal/application/endpoint/command/update_endpoint.go` — 更新 Endpoint
- `internal/application/endpoint/command/delete_endpoint.go` — 删除 Endpoint
- `internal/application/endpoint/query/list_endpoints.go` — 列出 Endpoints
- `internal/application/model/command/create_model.go` — 创建 Model
- `internal/application/model/command/update_model.go` — 更新 Model
- `internal/application/model/command/delete_model.go` — 删除 Model
- `internal/application/model/query/list_models.go` — 列出 Models
- `internal/handler/endpoint.go` — Endpoint handler
- `internal/handler/model.go` — Model handler
- `internal/router/endpoint.go` — Endpoint 路由
- `internal/router/model.go` — Model 路由
- `internal/router/web.go` — Web 静态文件路由
- `internal/web/.gitignore` — 忽略 dist/

### 后端修改文件
- `internal/domain/llmproxy/repository.go` — 新增 CRUD 接口方法
- `internal/domain/llmproxy/aggregate/endpoint.go` — 新增 Update 方法
- `internal/domain/llmproxy/aggregate/model.go` — 新增 Update 方法
- `internal/infrastructure/repository/endpoint_repository.go` — 实现 CRUD
- `internal/handler/oauth2.go` — 新增 HandleBrowserCallback
- `internal/dto/oauth2.go` — 新增 BrowserCallbackReq/Rsp
- `internal/router/oauth2.go` — 新增 GET callback 路由
- `internal/router/session.go` — 新增 JWT 鉴权路由组
- `internal/router/router.go` — 注册新路由
- `internal/bootstrap/router.go` — 新增 handler 依赖
- `internal/bootstrap/container.go` — 新增 DI 注册
- `Makefile` — 新增 web-build 目标

### 前端新建文件（`web/` 目录）
- `web/package.json`, `web/tsconfig.json`, `web/next.config.ts`, `web/tailwind.config.ts`, `web/postcss.config.mjs`
- `web/src/app/layout.tsx` — 根布局
- `web/src/app/login/page.tsx` — 登录页
- `web/src/app/auth/callback/page.tsx` — OAuth2 回调中转
- `web/src/app/dashboard/page.tsx` — 仪表盘
- `web/src/app/sessions/page.tsx` — 会话列表
- `web/src/app/sessions/[id]/page.tsx` — 会话详情
- `web/src/app/apikeys/page.tsx` — API Key 管理
- `web/src/app/admin/endpoints/page.tsx` — Endpoint 管理
- `web/src/app/admin/models/page.tsx` — Model 管理
- `web/src/app/profile/page.tsx` — 个人资料
- `web/src/lib/api.ts` — API 客户端
- `web/src/lib/auth.tsx` — AuthProvider + useAuth
- `web/src/lib/permission.tsx` — PermissionGuard
- `web/src/components/ui/` — shadcn/ui 组件
- `web/src/components/layout/` — 布局组件 (Sidebar, Header)

---

## Phase 1: 后端 API

### Task 1: Endpoint/Model DTO

**Files:**
- Create: `internal/dto/endpoint.go`
- Create: `internal/dto/model.go`

- [ ] **Step 1: 创建 Endpoint DTO**

创建 `internal/dto/endpoint.go`：

```go
package dto

import "time"

// CreateEndpointReq 创建 Endpoint 请求
type CreateEndpointReq struct {
	Body *CreateEndpointReqBody `json:"body" doc:"Request body"`
}

// CreateEndpointReqBody 创建 Endpoint 请求体
type CreateEndpointReqBody struct {
	Name                        string `json:"name" required:"true" minLength:"1" maxLength:"64" doc:"Endpoint 名称"`
	OpenaiBaseURL               string `json:"openaiBaseURL" doc:"OpenAI Base URL"`
	AnthropicBaseURL            string `json:"anthropicBaseURL" doc:"Anthropic Base URL"`
	APIKey                      string `json:"apiKey" required:"true" doc:"上游 API Key"`
	SupportOpenAIChatCompletion bool   `json:"supportOpenAIChatCompletion" doc:"是否支持 OpenAI Chat Completion"`
	SupportOpenAIResponse       bool   `json:"supportOpenAIResponse" doc:"是否支持 OpenAI Response"`
	SupportAnthropicMessage     bool   `json:"supportAnthropicMessage" doc:"是否支持 Anthropic Message"`
}

// UpdateEndpointReq 更新 Endpoint 请求
type UpdateEndpointReq struct {
	ID   uint                   `path:"id" required:"true" minimum:"1" doc:"Endpoint ID"`
	Body *UpdateEndpointReqBody `json:"body" doc:"Request body"`
}

// UpdateEndpointReqBody 更新 Endpoint 请求体
type UpdateEndpointReqBody struct {
	Name                        *string `json:"name,omitempty" doc:"Endpoint 名称"`
	OpenaiBaseURL               *string `json:"openaiBaseURL,omitempty" doc:"OpenAI Base URL"`
	AnthropicBaseURL            *string `json:"anthropicBaseURL,omitempty" doc:"Anthropic Base URL"`
	APIKey                      *string `json:"apiKey,omitempty" doc:"上游 API Key"`
	SupportOpenAIChatCompletion *bool   `json:"supportOpenAIChatCompletion,omitempty" doc:"是否支持 OpenAI Chat Completion"`
	SupportOpenAIResponse       *bool   `json:"supportOpenAIResponse,omitempty" doc:"是否支持 OpenAI Response"`
	SupportAnthropicMessage     *bool   `json:"supportAnthropicMessage,omitempty" doc:"是否支持 Anthropic Message"`
}

// DeleteEndpointReq 删除 Endpoint 请求
type DeleteEndpointReq struct {
	ID uint `path:"id" required:"true" minimum:"1" doc:"Endpoint ID"`
}

// ListEndpointsRsp 列出 Endpoint 响应
type ListEndpointsRsp struct {
	CommonRsp
	Endpoints []*EndpointItem `json:"endpoints,omitempty" doc:"Endpoint 列表"`
}

// EndpointItem Endpoint 列表项
type EndpointItem struct {
	ID                          uint      `json:"id" doc:"Endpoint ID"`
	Name                        string    `json:"name" doc:"Endpoint 名称"`
	OpenaiBaseURL               string    `json:"openaiBaseURL" doc:"OpenAI Base URL"`
	AnthropicBaseURL            string    `json:"anthropicBaseURL" doc:"Anthropic Base URL"`
	MaskedAPIKey                string    `json:"maskedAPIKey" doc:"Masked API Key"`
	SupportOpenAIChatCompletion bool      `json:"supportOpenAIChatCompletion" doc:"是否支持 OpenAI Chat Completion"`
	SupportOpenAIResponse       bool      `json:"supportOpenAIResponse" doc:"是否支持 OpenAI Response"`
	SupportAnthropicMessage     bool      `json:"supportAnthropicMessage" doc:"是否支持 Anthropic Message"`
	CreatedAt                   time.Time `json:"createdAt" doc:"创建时间"`
	UpdatedAt                   time.Time `json:"updatedAt" doc:"更新时间"`
}
```

- [ ] **Step 2: 创建 Model DTO**

创建 `internal/dto/model.go`：

```go
package dto

import "time"

// CreateModelReq 创建 Model 请求
type CreateModelReq struct {
	Body *CreateModelReqBody `json:"body" doc:"Request body"`
}

// CreateModelReqBody 创建 Model 请求体
type CreateModelReqBody struct {
	Alias      string `json:"alias" required:"true" minLength:"1" doc:"模型别名（对外暴露）"`
	ModelName  string `json:"modelName" required:"true" minLength:"1" doc:"上游实际模型名"`
	EndpointID uint   `json:"endpointID" required:"true" minimum:"1" doc:"关联 Endpoint ID"`
}

// UpdateModelReq 更新 Model 请求
type UpdateModelReq struct {
	ID   uint                   `path:"id" required:"true" minimum:"1" doc:"Model ID"`
	Body *UpdateModelReqBody    `json:"body" doc:"Request body"`
}

// UpdateModelReqBody 更新 Model 请求体
type UpdateModelReqBody struct {
	Alias      *string `json:"alias,omitempty" doc:"模型别名"`
	ModelName  *string `json:"modelName,omitempty" doc:"上游实际模型名"`
	EndpointID *uint   `json:"endpointID,omitempty" minimum:"1" doc:"关联 Endpoint ID"`
}

// DeleteModelReq 删除 Model 请求
type DeleteModelReq struct {
	ID uint `path:"id" required:"true" minimum:"1" doc:"Model ID"`
}

// ListModelsRsp 列出 Model 响应
type ListModelsRsp struct {
	CommonRsp
	Models []*ModelItem `json:"models,omitempty" doc:"Model 列表"`
}

// ModelItem Model 列表项
type ModelItem struct {
	ID         uint      `json:"id" doc:"Model ID"`
	Alias      string    `json:"alias" doc:"模型别名"`
	ModelName  string    `json:"modelName" doc:"上游实际模型名"`
	EndpointID uint      `json:"endpointID" doc:"关联 Endpoint ID"`
	CreatedAt  time.Time `json:"createdAt" doc:"创建时间"`
	UpdatedAt  time.Time `json:"updatedAt" doc:"更新时间"`
}
```

- [ ] **Step 3: 验证编译**

Run: `go build ./internal/dto/...`
Expected: 编译通过

---

### Task 2: Endpoint/Model 领域层扩展

**Files:**
- Modify: `internal/domain/llmproxy/repository.go`
- Modify: `internal/domain/llmproxy/aggregate/endpoint.go`
- Modify: `internal/domain/llmproxy/aggregate/model.go`

- [ ] **Step 1: 扩展 Repository 接口**

在 `internal/domain/llmproxy/repository.go` 中，给 `EndpointRepository` 和 `ModelRepository` 添加 CRUD 方法：

```go
// EndpointRepository Endpoint 聚合根仓储接口
type EndpointRepository interface {
	FindByID(ctx context.Context, id uint) (*aggregate.Endpoint, error)
	Create(ctx context.Context, endpoint *aggregate.Endpoint) (uint, error)
	Update(ctx context.Context, endpoint *aggregate.Endpoint) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context) ([]*aggregate.Endpoint, error)
}

// ModelRepository Model 聚合根仓储接口
type ModelRepository interface {
	FindByAlias(ctx context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error)
	FindByID(ctx context.Context, id uint) (*aggregate.Model, error)
	Create(ctx context.Context, model *aggregate.Model) (uint, error)
	Update(ctx context.Context, model *aggregate.Model) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context) ([]*aggregate.Model, error)
}
```

- [ ] **Step 2: 给 Endpoint 聚合根添加 Update 方法**

在 `internal/domain/llmproxy/aggregate/endpoint.go` 末尾添加：

```go
// Update 更新 Endpoint 字段（仅非零值字段）
func (e *Endpoint) Update(name, openaiBaseURL, anthropicBaseURL, apiKey *string, supportChatCompletion, supportResponse, supportMessage *bool) {
	if name != nil {
		e.name = *name
	}
	if openaiBaseURL != nil {
		e.openaiBaseURL = *openaiBaseURL
	}
	if anthropicBaseURL != nil {
		e.anthropicBaseURL = *anthropicBaseURL
	}
	if apiKey != nil {
		e.apiKey = *apiKey
	}
	if supportChatCompletion != nil {
		e.supportOpenAIChatCompletion = *supportChatCompletion
	}
	if supportResponse != nil {
		e.supportOpenAIResponse = *supportResponse
	}
	if supportMessage != nil {
		e.supportAnthropicMessage = *supportMessage
	}
}
```

- [ ] **Step 3: 给 Model 聚合根添加 Update 方法**

在 `internal/domain/llmproxy/aggregate/model.go` 末尾添加：

```go
// Update 更新 Model 字段（仅非零值字段）
func (m *Model) Update(alias *vo.EndpointAlias, model *string, endpointID *uint) {
	if alias != nil {
		m.alias = *alias
	}
	if model != nil {
		m.model = *model
	}
	if endpointID != nil {
		m.endpointID = *endpointID
	}
}
```

- [ ] **Step 4: 验证编译**

Run: `go build ./internal/domain/...`
Expected: 编译通过

---

### Task 3: Endpoint/Model Repository 实现

**Files:**
- Modify: `internal/infrastructure/repository/endpoint_repository.go`

- [ ] **Step 1: 实现 Endpoint CRUD 方法**

在 `internal/infrastructure/repository/endpoint_repository.go` 中，给 `endpointRepository` 添加 Create/Update/Delete/List 方法，给 `modelRepository` 添加 FindByID/Create/Update/Delete/List 方法。

参照现有 `FindByID` 方法的模式：使用 `r.db.WithContext(ctx)` + DAO 操作 + `toXxxAggregate` 转换。

关键实现：
- `Create`: 使用 `r.endpointDAO.Create(db, toEndpointModel(endpoint))` 返回新 ID
- `Update`: 使用 `r.db.Save()` 更新全字段
- `Delete`: 使用 `r.db.Delete(&dbmodel.Endpoint{}, id)` 软删除
- `List`: 使用 `r.endpointDAO.BatchGet(db, ...)` 返回列表
- Model 的 `FindByID`: 使用 `r.modelDAO.Get(db, &dbmodel.Model{ID: id}, ...)` 返回单个
- Model 的 `Create/Update/Delete/List`: 同 Endpoint 模式

注意：
- DAO 层可能需要新增 `Create` 方法，检查 `internal/infrastructure/database/dao/` 是否已有
- 如果 DAO 没有 Create 方法，直接使用 `r.db.Create()` / `r.db.Save()` / `r.db.Delete()`

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/infrastructure/repository/...`
Expected: 编译通过

---

### Task 4: Endpoint Command/Query Handlers

**Files:**
- Create: `internal/application/endpoint/command/create_endpoint.go`
- Create: `internal/application/endpoint/command/update_endpoint.go`
- Create: `internal/application/endpoint/command/delete_endpoint.go`
- Create: `internal/application/endpoint/query/list_endpoints.go`

- [ ] **Step 1: 创建 CreateEndpoint handler**

创建 `internal/application/endpoint/command/create_endpoint.go`，遵循 `internal/application/apikey/command/` 的模式：

```go
package command

import (
	"context"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

type CreateEndpointCommand struct {
	Name                        string
	OpenaiBaseURL               string
	AnthropicBaseURL            string
	APIKey                      string
	SupportOpenAIChatCompletion bool
	SupportOpenAIResponse       bool
	SupportAnthropicMessage     bool
}

type CreateEndpointResult struct {
	EndpointID uint
}

type CreateEndpointHandler interface {
	Handle(ctx context.Context, cmd CreateEndpointCommand) (*CreateEndpointResult, error)
}

type createEndpointHandler struct {
	repo llmproxy.EndpointRepository
}

func NewCreateEndpointHandler(repo llmproxy.EndpointRepository) CreateEndpointHandler {
	return &createEndpointHandler{repo: repo}
}

func (h *createEndpointHandler) Handle(ctx context.Context, cmd CreateEndpointCommand) (*CreateEndpointResult, error) {
	log := logger.WithCtx(ctx)

	ep, err := aggregate.CreateEndpoint(0, cmd.Name, cmd.OpenaiBaseURL, cmd.AnthropicBaseURL, cmd.APIKey, cmd.SupportOpenAIChatCompletion, cmd.SupportOpenAIResponse, cmd.SupportAnthropicMessage)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "validate endpoint")
	}

	id, err := h.repo.Create(ctx, ep)
	if err != nil {
		log.Error("[EndpointCommand] Create endpoint failed", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrDBInsert, err, "create endpoint")
	}

	log.Info("[EndpointCommand] Create endpoint success", zap.Uint("id", id))
	return &CreateEndpointResult{EndpointID: id}, nil
}
```

- [ ] **Step 2: 创建 UpdateEndpoint handler**

创建 `internal/application/endpoint/command/update_endpoint.go`：

```go
package command

// UpdateEndpointCommand 部分更新，指针字段为 nil 表示不更新
type UpdateEndpointCommand struct {
	EndpointID                  uint
	Name                        *string
	OpenaiBaseURL               *string
	AnthropicBaseURL            *string
	APIKey                      *string
	SupportOpenAIChatCompletion *bool
	SupportOpenAIResponse       *bool
	SupportAnthropicMessage     *bool
}

type UpdateEndpointHandler interface {
	Handle(ctx context.Context, cmd UpdateEndpointCommand) error
}

// 实现：FindByID → 调用 ep.Update(...) → repo.Update
// 若 FindByID 返回 nil, 返回 ierr.New(ierr.ErrNotFound, "endpoint not found")
```

- [ ] **Step 3: 创建 DeleteEndpoint handler**

创建 `internal/application/endpoint/command/delete_endpoint.go`：

```go
package command

type DeleteEndpointCommand struct {
	EndpointID uint
}

type DeleteEndpointHandler interface {
	Handle(ctx context.Context, cmd DeleteEndpointCommand) error
}

// 实现：repo.Delete(ctx, cmd.EndpointID)
// 删除前可选检查 endpoint 是否被 model 引用
```

- [ ] **Step 4: 创建 ListEndpoints handler**

创建 `internal/application/endpoint/query/list_endpoints.go`：

```go
package query

type ListEndpointsQuery struct{}

type EndpointView struct {
	ID                          uint
	Name                        string
	OpenaiBaseURL               string
	AnthropicBaseURL            string
	MaskedAPIKey                string
	SupportOpenAIChatCompletion bool
	SupportOpenAIResponse       bool
	SupportAnthropicMessage     bool
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type ListEndpointsHandler interface {
	Handle(ctx context.Context, q ListEndpointsQuery) ([]*EndpointView, error)
}

// 实现：repo.List → 遍历转换为 EndpointView（APIKey 用 util.MaskSecret 脱敏）
```

- [ ] **Step 5: 验证编译**

Run: `go build ./internal/application/endpoint/...`
Expected: 编译通过

---

### Task 5: Model Command/Query Handlers

**Files:**
- Create: `internal/application/model/command/create_model.go`
- Create: `internal/application/model/command/update_model.go`
- Create: `internal/application/model/command/delete_model.go`
- Create: `internal/application/model/query/list_models.go`

- [ ] **Step 1: 创建 CreateModel handler**

创建 `internal/application/model/command/create_model.go`：

```go
package command

type CreateModelCommand struct {
	Alias      string
	ModelName  string
	EndpointID uint
}

type CreateModelResult struct {
	ModelID uint
}

type CreateModelHandler interface {
	Handle(ctx context.Context, cmd CreateModelCommand) (*CreateModelResult, error)
}

// 实现：验证 endpoint 存在 → aggregate.CreateModel → repo.Create
```

- [ ] **Step 2: 创建 UpdateModel handler**

创建 `internal/application/model/command/update_model.go`：

```go
package command

type UpdateModelCommand struct {
	ModelID    uint
	Alias      *string
	ModelName  *string
	EndpointID *uint
}

type UpdateModelHandler interface {
	Handle(ctx context.Context, cmd UpdateModelCommand) error
}

// 实现：FindByID → model.Update(...) → repo.Update
```

- [ ] **Step 3: 创建 DeleteModel handler**

创建 `internal/application/model/command/delete_model.go`：

```go
package command

type DeleteModelCommand struct {
	ModelID uint
}

type DeleteModelHandler interface {
	Handle(ctx context.Context, cmd DeleteModelCommand) error
}

// 实现：repo.Delete(ctx, cmd.ModelID)
```

- [ ] **Step 4: 创建 ListModels handler**

创建 `internal/application/model/query/list_models.go`：

```go
package query

type ListModelsQuery struct{}

type ModelView struct {
	ID         uint
	Alias      string
	ModelName  string
	EndpointID uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ListModelsHandler interface {
	Handle(ctx context.Context, q ListModelsQuery) ([]*ModelView, error)
}

// 实现：repo.List → 遍历转换为 ModelView
```

- [ ] **Step 5: 验证编译**

Run: `go build ./internal/application/model/...`
Expected: 编译通过

---

### Task 6: Endpoint/Model Handlers

**Files:**
- Create: `internal/handler/endpoint.go`
- Create: `internal/handler/model.go`

- [ ] **Step 1: 创建 Endpoint handler**

创建 `internal/handler/endpoint.go`，遵循 `internal/handler/apikey.go` 的模式：

```go
package handler

import (
	"context"
	"go.uber.org/zap"
	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type EndpointHandler interface {
	HandleCreateEndpoint(ctx context.Context, req *dto.CreateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleListEndpoints(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.ListEndpointsRsp], error)
	HandleUpdateEndpoint(ctx context.Context, req *dto.UpdateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteEndpoint(ctx context.Context, req *dto.DeleteEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type EndpointDependencies struct {
	Create command.CreateEndpointHandler
	Update command.UpdateEndpointHandler
	Delete command.DeleteEndpointHandler
	List   query.ListEndpointsHandler
}

type endpointHandler struct {
	create command.CreateEndpointHandler
	update command.UpdateEndpointHandler
	delete command.DeleteEndpointHandler
	list   query.ListEndpointsHandler
}

func NewEndpointHandler(deps EndpointDependencies) EndpointHandler {
	return &endpointHandler{
		create: deps.Create,
		update: deps.Update,
		delete: deps.Delete,
		list:   deps.List,
	}
}
```

各 Handle 方法模式：
- 从 ctx 提取 userID/permission（虽然 admin-only，保持一致）
- 调用 command/query handler
- 转换 view → DTO item
- 用 `ierr.ToBizError(err, ierr.ErrInternal.BizError())` 处理错误
- 用 `apiutil.WrapHTTPResponse(rsp, nil)` 包装响应
- `HandleListEndpoints` 中 APIKey 用 `util.MaskSecret()` 脱敏

- [ ] **Step 2: 创建 Model handler**

创建 `internal/handler/model.go`，结构同 Endpoint handler：

```go
type ModelHandler interface {
	HandleCreateModel(ctx context.Context, req *dto.CreateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.ListModelsRsp], error)
	HandleUpdateModel(ctx context.Context, req *dto.UpdateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteModel(ctx context.Context, req *dto.DeleteModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}
```

- [ ] **Step 3: 验证编译**

Run: `go build ./internal/handler/...`
Expected: 编译通过

---

### Task 7: Endpoint/Model 路由注册

**Files:**
- Create: `internal/router/endpoint.go`
- Create: `internal/router/model.go`
- Modify: `internal/router/router.go`
- Modify: `internal/bootstrap/router.go`
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: 创建 Endpoint 路由**

创建 `internal/router/endpoint.go`，参照 `internal/router/apikey.go`：

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

func initEndpointRouter(endpointGroup huma.API, endpointHandler handler.EndpointHandler, db *gorm.DB, cache *redis.Client) {
	endpointGroup.UseMiddleware(middleware.JwtMiddleware(db, cache))
	endpointGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		cache, "endpointManage", constant.CtxKeyUserID, constant.PeriodManageAPIKey, constant.LimitManageAPIKey,
	))

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "createEndpoint",
		Method:      http.MethodPost,
		Path:        "/",
		Summary:     "CreateEndpoint",
		Tags:        []string{"Endpoint"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("createEndpoint", enum.PermissionAdmin)},
	}, endpointHandler.HandleCreateEndpoint)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "listEndpoints",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "ListEndpoints",
		Tags:        []string{"Endpoint"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listEndpoints", enum.PermissionAdmin)},
	}, endpointHandler.HandleListEndpoints)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "updateEndpoint",
		Method:      http.MethodPatch,
		Path:        "/{id}",
		Summary:     "UpdateEndpoint",
		Tags:        []string{"Endpoint"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("updateEndpoint", enum.PermissionAdmin)},
	}, endpointHandler.HandleUpdateEndpoint)

	huma.Register(endpointGroup, huma.Operation{
		OperationID: "deleteEndpoint",
		Method:      http.MethodDelete,
		Path:        "/{id}",
		Summary:     "DeleteEndpoint",
		Tags:        []string{"Endpoint"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("deleteEndpoint", enum.PermissionAdmin)},
	}, endpointHandler.HandleDeleteEndpoint)
}
```

- [ ] **Step 2: 创建 Model 路由**

创建 `internal/router/model.go`，同 Endpoint 路由模式，路径为 `/`、`/{id}`，权限为 admin。

- [ ] **Step 3: 注册到主路由**

修改 `internal/router/router.go`：

1. `APIRouterDependencies` 新增 `EndpointHandler handler.EndpointHandler` 和 `ModelHandler handler.ModelHandler`
2. `RegisterAPIRouter` 新增：
```go
endpointGroup := huma.NewGroup(v1Group, "/endpoint")
initEndpointRouter(endpointGroup, deps.EndpointHandler, deps.DB, deps.Cache)

modelGroup := huma.NewGroup(v1Group, "/model")
initModelRouter(modelGroup, deps.ModelHandler, deps.DB, deps.Cache)
```

- [ ] **Step 4: 注册到 bootstrap**

修改 `internal/bootstrap/router.go`：
- `routeParams` 新增 `EndpointHandler handler.EndpointHandler` 和 `ModelHandler handler.ModelHandler`
- `RegisterRoutes` 中的 `router.APIRouterDependencies` 新增对应字段

修改 `internal/bootstrap/container.go`：
- `provideApplication` 新增：
```go
endpointcommand "github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
endpointquery "github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
modelcommand "github.com/hcd233/aris-proxy-api/internal/application/model/command"
modelquery "github.com/hcd233/aris-proxy-api/internal/application/model/query"
```
- 新增 provider 函数：`newEndpointDependencies`, `newModelDependencies`
- `provideApplication` 新增 Provide 各 command/query handler
- `provideHandlers` 新增 Provide `handler.NewEndpointHandler`, `handler.NewModelHandler`

- [ ] **Step 5: 验证编译**

Run: `go build ./...`
Expected: 编译通过

---

### Task 8: OAuth2 浏览器回调重定向

**Files:**
- Modify: `internal/handler/oauth2.go`
- Modify: `internal/dto/oauth2.go`
- Modify: `internal/router/oauth2.go`（需要先读取确认现有路由注册方式）

- [ ] **Step 1: 新增浏览器回调 DTO**

在 `internal/dto/oauth2.go` 末尾添加：

```go
// BrowserCallbackReq 浏览器 OAuth2 回调请求（GET 参数）
type BrowserCallbackReq struct {
	Code     string              `query:"code" required:"true" doc:"授权码"`
	State    string              `query:"state" required:"true" doc:"CSRF State"`
	Platform enum.Oauth2Platform `query:"platform" enum:"github,google" required:"true" doc:"OAuth2 平台"`
}
```

- [ ] **Step 2: 新增 HandleBrowserCallback 方法**

在 `internal/handler/oauth2.go` 中添加方法。此方法与 `HandleCallback` 类似，但：
- 从 query 参数读取 code/state/platform
- 成功后返回 HTTP 302 重定向到 `/web/auth/callback?access_token=xxx&refresh_token=xxx`
- 失败时重定向到 `/web/login?error=auth_failed`

```go
func (h *oauth2Handler) HandleBrowserCallback(ctx context.Context, req *dto.BrowserCallbackReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	result, err := h.callback.Handle(ctx, command.HandleCallbackCommand{
		Platform: req.Platform,
		Code:     req.Code,
		State:    req.State,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[OAuth2Handler] Browser callback failed", zap.Error(err))
		// 重定向到登录页带错误
		return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, fiber.Redirect("/web/login?error=auth_failed", fiber.StatusFound))
	}
	redirectURL := fmt.Sprintf("/web/auth/callback?access_token=%s&refresh_token=%s",
		url.QueryEscape(result.TokenPair.AccessToken()),
		url.QueryEscape(result.TokenPair.RefreshToken()),
	)
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, fiber.Redirect(redirectURL, fiber.StatusFound))
}
```

注意：需要检查 Huma 是否支持直接返回 Fiber redirect。可能需要使用 `huma.Register` 的自定义响应处理，或直接在 Fiber 层处理这个路由而非 Huma。如果 Huma 不方便处理 redirect，可以注册为 Fiber 原生路由。

- [ ] **Step 3: 注册浏览器回调路由**

在 `internal/router/oauth2.go` 中新增 `GET /callback` 路由。

注意：GitHub/Google 的 OAuth2 redirect_uri 会将 code 和 state 作为 query 参数回调。需要确认 GitHub/Google 的回调是指向 `GET /api/v1/oauth2/callback` 还是 `POST`。当前后端注册了 `POST /api/v1/oauth2/callback`，但浏览器 OAuth2 回调是 GET 请求。

如果使用 Fiber 原生路由处理重定向更简单，可以在 `initOauth2Router` 外或 router.go 中直接注册：
```go
app.Get("/api/v1/oauth2/callback", func(c fiber.Ctx) error {
    // 解析 query params → 调用 handler → 返回 redirect
})
```

- [ ] **Step 4: 更新 OAuth2 redirect URL 环境变量**

在 `env/api.env.template` 中更新 OAuth2 redirect URL 示例，指向 `GET /api/v1/oauth2/callback`。

- [ ] **Step 5: 验证编译**

Run: `go build ./...`
Expected: 编译通过

---

### Task 9: Session JWT 鉴权路由

**Files:**
- Modify: `internal/router/session.go`

- [ ] **Step 1: 新增 JWT 鉴权的 Session 路由**

当前 `initSessionRouter` 使用 `APIKeyMiddleware`。前端用户通过 JWT 鉴权，需要新增一组路由。

在 `internal/router/session.go` 中添加：

```go
func initSessionJWTRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, db *gorm.DB, cache *redis.Client) {
	sessionGroup.UseMiddleware(middleware.JwtMiddleware(db, cache))

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessionsJWT",
		Method:      http.MethodGet,
		Path:        "/jwt/list",
		Summary:     "ListSessions (JWT)",
		Description: "Paginate session list for current user (JWT auth)",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listSessionsJWT", enum.PermissionUser)},
	}, sessionHandler.HandleListSessions)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getSessionJWT",
		Method:      http.MethodGet,
		Path:        "/jwt/",
		Summary:     "GetSession (JWT)",
		Description: "Get session detail by session ID (JWT auth)",
		Tags:        []string{"Session"},
		Security:    []map[string][]string{{"jwtAuth": {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getSessionJWT", enum.PermissionUser)},
	}, sessionHandler.HandleGetSession)
}
```

在 `router.go` 的 `RegisterAPIRouter` 中添加：
```go
initSessionJWTRouter(sessionGroup, deps.SessionHandler, deps.DB, deps.Cache)
```

同时需要在 `APIRouterDependencies` 中添加 `Cache *redis.Client`（如果还没有的话——检查发现已有）。

- [ ] **Step 2: 验证编译**

Run: `go build ./...`
Expected: 编译通过

---

### Task 10: Web 静态文件服务

**Files:**
- Create: `internal/web/.gitignore`
- Create: `internal/router/web.go`
- Modify: `internal/router/router.go`
- Modify: `Makefile`

- [ ] **Step 1: 创建 gitignore**

创建 `internal/web/.gitignore`：
```
dist/
```

- [ ] **Step 2: 创建 Web 路由**

创建 `internal/router/web.go`：

```go
package router

import (
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/filesystem"
)

// RegisterWebRouter 注册前端静态文件路由
func RegisterWebRouter(app *fiber.App, webFS fs.FS) {
	subFS, _ := fs.Sub(webFS, "dist")

	app.Use("/web", filesystem.New(filesystem.Config{
		Root:   http.FS(subFS),
		Index:  "index.html",
		Browse: false,
	}))

	// SPA fallback: 未匹配的 /web/* 路径回退到 index.html
	app.Use("/web/*", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).Type("html").SendFile("index.html")
	})
}
```

注意：SPA fallback 需要从 embed.FS 读取 `index.html` 而非文件系统。实际实现需要调整，确保 `SendFile` 从嵌入的 FS 读取。

- [ ] **Step 3: 在 router.go 中注册**

在 `RegisterDocsRouter` 或主路由注册流程中调用 `RegisterWebRouter`。

需要确认 embed.FS 的初始化时机。创建 `internal/web/static.go`：

```go
package web

import (
	"embed"
)

//go:embed all:dist
var DistFS embed.FS
```

在 `cmd/server.go` 或 `bootstrap/router.go` 中调用 `RegisterWebRouter(server.App, web.DistFS)`。

- [ ] **Step 4: 更新 Makefile**

在 `Makefile` 中添加：

```makefile
.PHONY: web-build web-clean

## web-build: 构建前端静态文件
web-build:
	cd web && npm ci && npm run build
	rm -rf internal/web/dist
	cp -r web/out internal/web/dist

## web-clean: 清理前端构建产物
web-clean:
	rm -rf internal/web/dist web/.next web/out
```

修改 `build` 目标，在构建 Go 二进制前先构建前端：

```makefile
## build: 生产构建（strip 符号，含前端）
build: web-build
	CGO_ENABLED=0 go build \
		$(BUILD_FLAGS) \
		-ldflags="$(LDFLAGS)" \
		-o $(OUTPUT) $(MAIN)
	@echo "Built $(OUTPUT) ($$(du -h $(OUTPUT) | cut -f1))"
```

- [ ] **Step 5: 验证编译**

Run: `go build ./internal/web/...`（没有 dist 目录时会报错，先创建空 `internal/web/dist/.gitkeep`）
Expected: 需要至少一个文件在 dist/ 中才能 embed

---

## Phase 2: 前端基础

### Task 11: Next.js 项目初始化

**Files:**
- Create: `web/` 整个目录

- [ ] **Step 1: 初始化 Next.js 项目**

```bash
npx create-next-app@latest web --typescript --tailwind --eslint --app --src-dir --import-alias "@/*" --no-turbopack
```

- [ ] **Step 2: 配置 next.config.ts**

```typescript
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  basePath: "/web",
  trailingSlash: true,
  images: {
    unoptimized: true, // 静态导出不支持 next/image 优化
  },
};

export default nextConfig;
```

- [ ] **Step 3: 安装依赖**

```bash
cd web && npm install react-markdown
```

- [ ] **Step 4: 初始化 shadcn/ui**

```bash
cd web && npx shadcn@latest init
```

选择：Style: Default, Base color: Neutral, CSS variables: yes

然后安装需要的组件：

```bash
npx shadcn@latest add button card table dialog input label select badge separator dropdown-menu avatar tabs sheet skeleton
```

- [ ] **Step 5: 验证构建**

```bash
cd web && npm run build
```
Expected: `web/out/` 目录生成静态文件

---

### Task 12: API 客户端 & Auth Context

**Files:**
- Create: `web/src/lib/api.ts`
- Create: `web/src/lib/auth.tsx`
- Create: `web/src/lib/permission.tsx`

- [ ] **Step 1: 创建 API 客户端**

创建 `web/src/lib/api.ts`：

```typescript
const BASE_URL = "/api/v1";

interface APIResponse<T> {
  error?: { code: string; message: string };
  [key: string]: T;
}

class APIClient {
  private getAccessToken(): string | null {
    return localStorage.getItem("access_token");
  }

  private getRefreshToken(): string | null {
    return localStorage.getItem("refresh_token");
  }

  private async refreshAccessToken(): Promise<boolean> {
    const refreshToken = this.getRefreshToken();
    if (!refreshToken) return false;

    try {
      const res = await fetch(`${BASE_URL}/token/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refreshToken }),
      });
      const data = await res.json();
      if (data.accessToken) {
        localStorage.setItem("access_token", data.accessToken);
        localStorage.setItem("refresh_token", data.refreshToken);
        return true;
      }
    } catch {}
    return false;
  }

  async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...(options.headers as Record<string, string>),
    };
    const token = this.getAccessToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;

    const res = await fetch(`${BASE_URL}${path}`, { ...options, headers });

    if (res.status === 401) {
      const refreshed = await this.refreshAccessToken();
      if (refreshed) {
        headers["Authorization"] = `Bearer ${this.getAccessToken()}`;
        const retryRes = await fetch(`${BASE_URL}${path}`, { ...options, headers });
        return retryRes.json();
      }
      localStorage.removeItem("access_token");
      localStorage.removeItem("refresh_token");
      window.location.href = "/web/login";
      throw new Error("Authentication required");
    }

    return res.json();
  }

  get<T>(path: string) {
    return this.request<T>(path);
  }

  post<T>(path: string, body?: unknown) {
    return this.request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined });
  }

  patch<T>(path: string, body?: unknown) {
    return this.request<T>(path, { method: "PATCH", body: body ? JSON.stringify(body) : undefined });
  }

  delete<T>(path: string) {
    return this.request<T>(path, { method: "DELETE" });
  }
}

export const api = new APIClient();
```

- [ ] **Step 2: 创建 Auth Context**

创建 `web/src/lib/auth.tsx`：

```tsx
"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";

interface User {
  id: number;
  name: string;
  email: string;
  avatar: string;
  permission: "pending" | "user" | "admin";
}

interface AuthContextType {
  user: User | null;
  loading: boolean;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    if (!token) {
      setLoading(false);
      return;
    }
    fetch("/api/v1/user/current", {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => res.json())
      .then((data) => {
        if (data.user) setUser(data.user);
      })
      .catch(() => {
        localStorage.removeItem("access_token");
        localStorage.removeItem("refresh_token");
      })
      .finally(() => setLoading(false));
  }, []);

  const logout = () => {
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
    setUser(null);
    window.location.href = "/web/login";
  };

  return (
    <AuthContext.Provider value={{ user, loading, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
```

- [ ] **Step 3: 创建 PermissionGuard**

创建 `web/src/lib/permission.tsx`：

```tsx
"use client";

import { useAuth } from "./auth";
import { ReactNode } from "react";

interface PermissionGuardProps {
  required: "user" | "admin";
  children: ReactNode;
  fallback?: ReactNode;
}

const roleLevel = { pending: 0, user: 1, admin: 2 };

export function PermissionGuard({ required, children, fallback }: PermissionGuardProps) {
  const { user } = useAuth();

  if (!user) return null;
  if (user.permission === "pending") {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <h2 className="text-2xl font-bold">等待审核</h2>
          <p className="text-muted-foreground mt-2">您的账号正在等待管理员审核，请稍后再试。</p>
        </div>
      </div>
    );
  }
  if (roleLevel[user.permission] < roleLevel[required]) {
    return fallback ? <>{fallback}</> : null;
  }
  return <>{children}</>;
}
```

- [ ] **Step 4: 更新根布局**

修改 `web/src/app/layout.tsx`：

```tsx
import type { Metadata } from "next";
import "./globals.css";
import { AuthProvider } from "@/lib/auth";

export const metadata: Metadata = {
  title: "Aris Proxy API",
  description: "LLM 代理网关管理平台",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <body>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
```

---

### Task 13: 登录页 & Auth 回调

**Files:**
- Create: `web/src/app/login/page.tsx`
- Create: `web/src/app/auth/callback/page.tsx`

- [ ] **Step 1: 创建登录页**

创建 `web/src/app/login/page.tsx`：

```tsx
"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuth } from "@/lib/auth";
import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function LoginPage() {
  const { user, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && user && user.permission !== "pending") {
      router.replace("/web/dashboard");
    }
  }, [user, loading, router]);

  const handleLogin = (platform: "github" | "google") => {
    // 方案：先获取 redirect URL，再跳转
    fetch(`/api/v1/oauth2/login?platform=${platform}`)
      .then((res) => res.json())
      .then((data) => {
        if (data.redirectURL) {
          window.location.href = data.redirectURL;
        }
      });
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Aris Proxy API</CardTitle>
          <p className="text-sm text-muted-foreground">LLM 代理网关管理平台</p>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <Button variant="outline" onClick={() => handleLogin("github")} className="w-full">
            GitHub 登录
          </Button>
          <Button variant="outline" onClick={() => handleLogin("google")} className="w-full">
            Google 登录
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
```

- [ ] **Step 2: 创建 Auth 回调页**

创建 `web/src/app/auth/callback/page.tsx`：

```tsx
"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

export default function AuthCallbackPage() {
  const router = useRouter();
  const searchParams = useSearchParams();

  useEffect(() => {
    const accessToken = searchParams.get("access_token");
    const refreshToken = searchParams.get("refresh_token");
    const error = searchParams.get("error");

    if (error) {
      router.replace(`/web/login?error=${encodeURIComponent(error)}`);
      return;
    }

    if (accessToken && refreshToken) {
      localStorage.setItem("access_token", accessToken);
      localStorage.setItem("refresh_token", refreshToken);
      router.replace("/web/dashboard");
    } else {
      router.replace("/web/login?error=missing_token");
    }
  }, [searchParams, router]);

  return (
    <div className="flex items-center justify-center min-h-screen">
      <p className="text-muted-foreground">正在登录...</p>
    </div>
  );
}
```

---

### Task 14: 布局组件

**Files:**
- Create: `web/src/components/layout/sidebar.tsx`
- Create: `web/src/app/(dashboard)/layout.tsx`

- [ ] **Step 1: 创建 Sidebar 组件**

创建 `web/src/components/layout/sidebar.tsx`：侧边栏导航组件，包含导航链接（仪表盘、会话历史、API Key、管理员菜单），根据用户权限显示/隐藏菜单项。使用 shadcn/ui 的 Sheet 组件做移动端响应式。

- [ ] **Step 2: 创建 Dashboard 布局**

创建 `web/src/app/(dashboard)/layout.tsx`：带侧边栏的布局，包裹 dashboard/sessions/apikeys/admin/profile 页面。检查登录状态，未登录重定向到 `/web/login`。

---

## Phase 3: 前端页面

### Task 15: 仪表盘页

**Files:**
- Create: `web/src/app/(dashboard)/dashboard/page.tsx`

- [ ] **Step 1: 创建仪表盘页**

显示用户欢迎信息、角色标识、快捷入口卡片（会话历史、API Key 管理），admin 额外显示 Endpoint/Model 管理入口。

---

### Task 16: 会话列表页

**Files:**
- Create: `web/src/app/(dashboard)/sessions/page.tsx`

- [ ] **Step 1: 创建会话列表页**

使用 shadcn Table 组件，分页显示会话列表。调用 `GET /api/v1/session/jwt/list`。列：会话摘要、API Key 名称、评分、时间。点击行跳转到 `/web/sessions/{id}`。

---

### Task 17: 会话详情页

**Files:**
- Create: `web/src/app/(dashboard)/sessions/[id]/page.tsx`

- [ ] **Step 1: 创建会话详情页**

调用 `GET /api/v1/session/jwt/?sessionId={id}`，获取会话消息和工具。

对话气泡布局：
- 用户消息：右对齐，深色背景
- AI 回复：左对齐，浅色背景，使用 `react-markdown` 渲染 Markdown

顶部显示会话元信息（模型名称、评分等）。

---

### Task 18: API Key 管理页

**Files:**
- Create: `web/src/app/(dashboard)/apikeys/page.tsx`

- [ ] **Step 1: 创建 API Key 管理页**

- 列表：Table 显示 Key 名称、前缀、创建时间
- 创建：Dialog 弹窗输入名称，创建后 Dialog 内显示完整 Key（仅一次）
- 删除：确认 Dialog 后删除

调用：
- `GET /api/v1/apikey/` 列表
- `POST /api/v1/apikey/` 创建
- `DELETE /api/v1/apikey/{id}` 删除

---

### Task 19: Endpoint 管理页（admin）

**Files:**
- Create: `web/src/app/(dashboard)/admin/endpoints/page.tsx`

- [ ] **Step 1: 创建 Endpoint 管理页**

PermissionGuard 包裹，要求 admin 权限。

- 列表：Table 显示名称、OpenAI/Anthropic URL、能力开关
- 创建：Dialog 弹窗表单
- 编辑：Dialog 弹窗表单（预填当前值）
- 删除：确认 Dialog

调用：
- `GET /api/v1/endpoint/` 列表
- `POST /api/v1/endpoint/` 创建
- `PATCH /api/v1/endpoint/{id}` 更新
- `DELETE /api/v1/endpoint/{id}` 删除

---

### Task 20: Model 管理页（admin）

**Files:**
- Create: `web/src/app/(dashboard)/admin/models/page.tsx`

- [ ] **Step 1: 创建 Model 管理页**

同 Endpoint 管理模式，额外：Endpoint 下拉选择（从 Endpoint 列表获取）。

调用：
- `GET /api/v1/model/` 列表
- `POST /api/v1/model/` 创建
- `PATCH /api/v1/model/{id}` 更新
- `DELETE /api/v1/model/{id}` 删除
- `GET /api/v1/endpoint/` 获取 Endpoint 下拉选项

---

### Task 21: 个人资料页

**Files:**
- Create: `web/src/app/(dashboard)/profile/page.tsx`

- [ ] **Step 1: 创建个人资料页**

显示用户头像、名称、邮箱、角色。可编辑名称，调用 `PATCH /api/v1/user/`。

---

## Phase 4: 集成

### Task 22: 构建集成 & 端到端验证

**Files:**
- Modify: `Makefile`
- Modify: `internal/web/.gitignore`

- [ ] **Step 1: 完整构建测试**

```bash
make web-build
make build
```

Expected: 二进制文件包含嵌入的前端静态文件

- [ ] **Step 2: 启动服务验证**

```bash
./aris-proxy-api server start --host localhost --port 8080
```

访问 `http://localhost:8080/web/` 验证：
1. 前端页面正常加载
2. SPA 路由回退到 index.html
3. 登录页 GitHub/Google 按钮可跳转
4. API 请求同源无 CORS 问题

- [ ] **Step 3: 后端 lint 验证**

```bash
make lint
go test -count=1 ./...
```
Expected: lint 通过，测试通过

---

## 自审清单

### Spec 覆盖检查
- ✅ OAuth2 登录 (GitHub/Google) → Task 8, 13
- ✅ JWT token 存储 → Task 12, 13
- ✅ 用户角色权限控制 → Task 12 (PermissionGuard), 14 (Sidebar)
- ✅ pending 用户等待审核页 → Task 12 (PermissionGuard)
- ✅ user 会话列表 → Task 16
- ✅ user 会话详情（气泡式） → Task 17
- ✅ admin Endpoint CRUD → Task 1-7, 19
- ✅ admin Model CRUD → Task 1-6, 20
- ✅ 部署到 api.lvlvko.top/web → Task 10, 22
- ✅ Session JWT 鉴权路由 → Task 9

### Placeholder 扫描
- 无 TBD/TODO/后续补充
- Task 8 的 redirect 实现标注了需确认 Huma redirect 支持，提供了 Fiber 原生路由备选方案
- 所有 handler 实现提供了完整模式但标注"参照现有模式"以避免冗余重复代码

### 类型一致性
- DTO 字段名与 handler 方法参数对应
- Repository 接口方法签名与 aggregate 创建函数参数一致
- 前端 API 客户端路径与后端路由路径一致
