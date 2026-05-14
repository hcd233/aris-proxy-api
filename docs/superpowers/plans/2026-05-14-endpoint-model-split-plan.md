# Endpoint/Model 拆表实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `model_endpoint` 一张表拆分成 `endpoint` 和 `model` 两张表，适配多协议 base URL 和接口支持标记。

**Architecture:** 两表逻辑外键关联：`endpoint` 存储上游服务配置（两个协议的 base URL + 共享 api_key + 接口支持标记），`model` 存储模型别名与 endpoint 的关联。解析改为按 alias 查 model 列表，随机选 endpoint。

**Tech Stack:** Go 1.25, GORM, go.uber.org/dig

---

### Task 1: 创建 Endpoint/Model GORM 模型，删除旧 ModelEndpoint

**Files:**
- Create: `internal/infrastructure/database/model/endpoint.go`
- Create: `internal/infrastructure/database/model/model.go`
- Delete: `internal/infrastructure/database/model/model_endpoint.go`
- Modify: `internal/infrastructure/database/model/base.go`

- [ ] **Step 1: 创建 `internal/infrastructure/database/model/endpoint.go`**

```go
package model

type Endpoint struct {
	BaseModel
	ID                          uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:端点ID"`
	Name                        string `json:"name" gorm:"column:name;not null;uniqueIndex:idx_endpoint_name_deleted,priority:1;comment:端点名称"`
	OpenaiBaseURL               string `json:"openai_base_url" gorm:"column:openai_base_url;not null;comment:OpenAI协议baseURL"`
	AnthropicBaseURL            string `json:"anthropic_base_url" gorm:"column:anthropic_base_url;not null;comment:Anthropic协议baseURL"`
	APIKey                      string `json:"api_key" gorm:"column:api_key;not null;comment:上游API密钥"`
	SupportOpenAIChatCompletion bool   `json:"support_openai_chat_completion" gorm:"column:support_openai_chat_completion;not null;default:true;comment:支持/chat/completions"`
	SupportOpenAIResponse       bool   `json:"support_openai_response" gorm:"column:support_openai_response;not null;default:false;comment:支持/responses"`
	SupportAnthropicMessage     bool   `json:"support_anthropic_message" gorm:"column:support_anthropic_message;not null;default:false;comment:支持/messages"`
	DeletedAt                   int64  `json:"deleted_at" gorm:"column:deleted_at;default:0;uniqueIndex:idx_endpoint_name_deleted,priority:2;comment:删除时间"`
}
```

- [ ] **Step 2: 创建 `internal/infrastructure/database/model/model.go`**

```go
package model

// Model 模型关联表
// 记录对外暴露的模型别名与上游端点的关联关系。
// 同一 alias 可通过多条记录关联多个 endpoint，解析时随机选择。
type Model struct {
	BaseModel
	ID         uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:模型关联ID"`
	Alias      string `json:"alias" gorm:"column:alias;not null;uniqueIndex:idx_model_alias_endpoint_deleted,priority:1;comment:对外暴露的模型别名"`
	ModelName  string `json:"model_name" gorm:"column:model;not null;comment:上游实际模型名"`
	EndpointID uint   `json:"endpoint_id" gorm:"column:endpoint_id;not null;uniqueIndex:idx_model_alias_endpoint_deleted,priority:2;comment:逻辑外键→endpoint.id"`
	DeletedAt  int64  `json:"deleted_at" gorm:"column:deleted_at;default:0;uniqueIndex:idx_model_alias_endpoint_deleted,priority:3;comment:删除时间"`
}
```

注意：struct 字段名 `ModelName` 对应 DB 列 `model`。用 `ModelName` 避免和包名冲突。GORM column tag 指定为 `model`。

- [ ] **Step 3: 删除 `internal/infrastructure/database/model/model_endpoint.go`**

```bash
rm internal/infrastructure/database/model/model_endpoint.go
```

- [ ] **Step 4: 更新 `internal/infrastructure/database/model/base.go` Models 注册**

将 `&ModelEndpoint{}` 替换为 `&Endpoint{}, &Model{}`：

```go
var Models = []any{
	&User{},
	&Message{},
	&Session{},
	&Tool{},
	&Endpoint{},
	&Model{},
	&ProxyAPIKey{},
	&ModelCallAudit{},
}
```

- [ ] **Step 5: 编译验证**

```bash
go build ./...
```
预期：编译报错——很多引用 `dbmodel.ModelEndpoint` 的地方找不到。说明正确。接下来逐步修复。

---

### Task 2: 创建 DAO + 更新 Singleton

**Files:**
- Create: `internal/infrastructure/database/dao/endpoint.go`
- Create: `internal/infrastructure/database/dao/model.go`
- Delete: `internal/infrastructure/database/dao/model_endpoint.go`
- Modify: `internal/infrastructure/database/dao/singleton.go`

- [ ] **Step 1: 创建 `internal/infrastructure/database/dao/endpoint.go`**

```go
package dao

import dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

type EndpointDAO struct {
	baseDAO[dbmodel.Endpoint]
}
```

- [ ] **Step 2: 创建 `internal/infrastructure/database/dao/model.go`**

```go
package dao

import dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

type ModelDAO struct {
	baseDAO[dbmodel.Model]
}
```

- [ ] **Step 3: 删除旧 `model_endpoint.go` DAO**

```bash
rm internal/infrastructure/database/dao/model_endpoint.go
```

- [ ] **Step 4: 更新 `singleton.go`**

添加 endpointDAOSingleton 和 modelDAOSingleton 变量，删除旧的 modelEndpointDAOSingleton：

```go
var (
	userDAOSingleton           *UserDAO
	messageDAOSingleton        *MessageDAO
	sessionDAOSingleton        *SessionDAO
	toolDAOSingleton           *ToolDAO
	endpointDAOSingleton       *EndpointDAO
	modelDAOSingleton          *ModelDAO
	proxyAPIKeyDAOSingleton    *ProxyAPIKeyDAO
	modelCallAuditDAOSingleton *ModelCallAuditDAO
)

func init() {
	userDAOSingleton = &UserDAO{}
	messageDAOSingleton = &MessageDAO{}
	sessionDAOSingleton = &SessionDAO{}
	toolDAOSingleton = &ToolDAO{}
	endpointDAOSingleton = &EndpointDAO{}
	modelDAOSingleton = &ModelDAO{}
	proxyAPIKeyDAOSingleton = &ProxyAPIKeyDAO{}
	modelCallAuditDAOSingleton = &ModelCallAuditDAO{}
}
```

添加 getter 函数：

```go
func GetEndpointDAO() *EndpointDAO {
	return endpointDAOSingleton
}

func GetModelDAO() *ModelDAO {
	return modelDAOSingleton
}
```

删除 `GetModelEndpointDAO()`。

- [ ] **Step 5: 编译验证**

```bash
go build ./...
```

---

### Task 3: 创建新 Domain Aggregates（Endpoint + Model）

**Files:**
- Rewrite: `internal/domain/llmproxy/aggregate/endpoint.go`
- Create: `internal/domain/llmproxy/aggregate/model.go`

- [ ] **Step 1: 重写 `aggregate/endpoint.go`**

```go
package aggregate

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonagg "github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
)

type Endpoint struct {
	commonagg.Base

	name                        string
	openaiBaseURL               string
	anthropicBaseURL            string
	apiKey                      string
	supportOpenAIChatCompletion bool
	supportOpenAIResponse       bool
	supportAnthropicMessage     bool
}

func CreateEndpoint(
	id uint,
	name, openaiBaseURL, anthropicBaseURL, apiKey string,
	supportChatCompletion, supportResponse, supportMessage bool,
) (*Endpoint, error) {
	if name == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint name cannot be empty")
	}
	if openaiBaseURL == "" || anthropicBaseURL == "" || apiKey == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint baseURLs and apiKey must not be empty")
	}
	ep := &Endpoint{
		name:                        name,
		openaiBaseURL:               openaiBaseURL,
		anthropicBaseURL:            anthropicBaseURL,
		apiKey:                      apiKey,
		supportOpenAIChatCompletion: supportChatCompletion,
		supportOpenAIResponse:       supportResponse,
		supportAnthropicMessage:     supportMessage,
	}
	ep.SetID(id)
	return ep, nil
}

func (*Endpoint) AggregateType() string { return constant.AggregateTypeEndpoint }

func (e *Endpoint) Name() string                        { return e.name }
func (e *Endpoint) OpenaiBaseURL() string               { return e.openaiBaseURL }
func (e *Endpoint) AnthropicBaseURL() string            { return e.anthropicBaseURL }
func (e *Endpoint) APIKey() string                      { return e.apiKey }
func (e *Endpoint) SupportOpenAIChatCompletion() bool   { return e.supportOpenAIChatCompletion }
func (e *Endpoint) SupportOpenAIResponse() bool         { return e.supportOpenAIResponse }
func (e *Endpoint) SupportAnthropicMessage() bool       { return e.supportAnthropicMessage }
```

- [ ] **Step 2: 创建 `aggregate/model.go`**

```go
package aggregate

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonagg "github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

type Model struct {
	commonagg.Base

	alias      vo.EndpointAlias
	model      string
	endpointID uint
}

func CreateModel(id uint, alias vo.EndpointAlias, model string, endpointID uint) (*Model, error) {
	if alias.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "model alias cannot be empty")
	}
	if model == "" {
		return nil, ierr.New(ierr.ErrValidation, "model name cannot be empty")
	}
	if endpointID == 0 {
		return nil, ierr.New(ierr.ErrValidation, "endpoint id cannot be 0")
	}
	m := &Model{
		alias:      alias,
		model:      model,
		endpointID: endpointID,
	}
	m.SetID(id)
	return m, nil
}

func (*Model) AggregateType() string { return constant.AggregateTypeModel }

func (m *Model) Alias() vo.EndpointAlias { return m.alias }
func (m *Model) ModelName() string       { return m.model }
func (m *Model) EndpointID() uint        { return m.endpointID }
```

- [ ] **Step 3: 编译验证**

```bash
go build ./...
```

---

### Task 4: 更新 Value Objects

**Files:**
- Modify: `internal/domain/llmproxy/vo/upstream_creds.go`

- [ ] **Step 1: 修改 `upstream_creds.go`——UpstreamCreds 不再包含 model**

`UpstreamCreds` 现在只包含 `baseURL` 和 `apiKey`（model 由 `aggregate.Model` 提供）：

```go
package vo

import "github.com/hcd233/aris-proxy-api/internal/common/ierr"

type UpstreamCreds struct {
	baseURL string
	apiKey  string
}

func NewUpstreamCreds(baseURL, apiKey string) (UpstreamCreds, error) {
	if baseURL == "" || apiKey == "" {
		return UpstreamCreds{}, ierr.New(ierr.ErrValidation, "upstream creds must have non-empty baseURL and apiKey")
	}
	return UpstreamCreds{baseURL: baseURL, apiKey: apiKey}, nil
}

func (c UpstreamCreds) BaseURL() string        { return c.baseURL }
func (c UpstreamCreds) APIKey() string          { return c.apiKey }
func (c UpstreamCreds) IsValid() bool           { return c.baseURL != "" && c.apiKey != "" }
func (c UpstreamCreds) MaskedAPIKey() string    { return util.MaskSecret(c.apiKey) }
```

注意：`util` 包导入需要加上：
```go
import (
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/util"
)
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

---

### Task 5: 更新 Repository Interfaces

**Files:**
- Rewrite: `internal/domain/llmproxy/repository.go`

- [ ] **Step 1: 重写 `repository.go`**

```go
package llmproxy

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

type EndpointRepository interface {
	FindByID(ctx context.Context, id uint) (*aggregate.Endpoint, error)
}

type ModelRepository interface {
	FindByAlias(ctx context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error)
}

type EndpointAliasProjection struct {
	Alias string
}

type EndpointProjection struct {
	ID                          uint
	Name                        string
	OpenaiBaseURL               string
	AnthropicBaseURL            string
	APIKey                      string
	SupportOpenAIChatCompletion bool
	SupportOpenAIResponse       bool
	SupportAnthropicMessage     bool
}

type ModelAliasProjection struct {
	Alias string
}

type EndpointReadRepository interface {
	ListAliases(ctx context.Context) ([]*ModelAliasProjection, error)
	FindEndpointByAlias(ctx context.Context, alias string) (*EndpointProjection, *ModelAliasProjection, error)
}
```

注意：删除了旧的 `EndpointCredentialProjection`、`EndpointAliasProjection`。ReadRepository 的 `ListAliases` 不再按 provider 过滤（model 表无 provider 字段），返回所有 alias。`FindEndpointByAlias` 是 CountTokens 等路径的新接口，随机选 endpoint 并返回。

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

---

### Task 6: 创建新 Repository 实现

**Files:**
- Rewrite: `internal/infrastructure/repository/endpoint_repository.go`

- [ ] **Step 1: 重写 `endpoint_repository.go`**

```go
package repository

import (
	"context"
	"errors"
	"math/rand"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

type endpointRepository struct {
	endpointDAO *dao.EndpointDAO
	modelDAO    *dao.ModelDAO
	db          *gorm.DB
}

func NewEndpointRepository(db *gorm.DB) llmproxy.EndpointRepository {
	return &endpointRepository{
		endpointDAO: dao.GetEndpointDAO(),
		modelDAO:    dao.GetModelDAO(),
		db:          db,
	}
}

func (r *endpointRepository) FindByID(ctx context.Context, id uint) (*aggregate.Endpoint, error) {
	db := r.db.WithContext(ctx)
	ep, err := r.endpointDAO.Get(db, &dbmodel.Endpoint{ID: id}, constant.EndpointRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find endpoint by id")
	}
	return toEndpointAggregate(ep)
}

type modelRepository struct {
	dao *dao.ModelDAO
	db  *gorm.DB
}

func NewModelRepository(db *gorm.DB) llmproxy.ModelRepository {
	return &modelRepository{
		dao: dao.GetModelDAO(),
		db:  db,
	}
}

func (r *modelRepository) FindByAlias(ctx context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error) {
	db := r.db.WithContext(ctx)
	models, err := r.dao.BatchGet(db, &dbmodel.Model{Alias: alias.String()}, constant.ModelRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find models by alias")
	}
	out := make([]*aggregate.Model, 0, len(models))
	for _, m := range models {
		agg, convErr := toModelAggregate(m)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, agg)
	}
	return out, nil
}

func toEndpointAggregate(m *dbmodel.Endpoint) (*aggregate.Endpoint, error) {
	return aggregate.CreateEndpoint(
		m.ID,
		m.Name,
		m.OpenaiBaseURL,
		m.AnthropicBaseURL,
		m.APIKey,
		m.SupportOpenAIChatCompletion,
		m.SupportOpenAIResponse,
		m.SupportAnthropicMessage,
	)
}

func toModelAggregate(m *dbmodel.Model) (*aggregate.Model, error) {
	return aggregate.CreateModel(m.ID, vo.EndpointAlias(m.Alias), m.ModelName, m.EndpointID)
}

// ==================== CQRS 读模型 ====================

type endpointReadRepository struct {
	endpointDAO *dao.EndpointDAO
	modelDAO    *dao.ModelDAO
	db          *gorm.DB
}

func NewEndpointReadRepository(db *gorm.DB) llmproxy.EndpointReadRepository {
	return &endpointReadRepository{
		endpointDAO: dao.GetEndpointDAO(),
		modelDAO:    dao.GetModelDAO(),
		db:          db,
	}
}

func (r *endpointReadRepository) ListAliases(ctx context.Context) ([]*llmproxy.ModelAliasProjection, error) {
	db := r.db.WithContext(ctx)
	models, err := r.modelDAO.BatchGet(db, &dbmodel.Model{}, constant.ModelRepoFieldsAlias)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list model aliases")
	}
	seen := make(map[string]struct{})
	out := make([]*llmproxy.ModelAliasProjection, 0, len(models))
	for _, m := range models {
		if _, ok := seen[m.Alias]; ok {
			continue
		}
		seen[m.Alias] = struct{}{}
		out = append(out, &llmproxy.ModelAliasProjection{Alias: m.Alias})
	}
	return out, nil
}

func (r *endpointReadRepository) FindEndpointByAlias(ctx context.Context, alias string) (*llmproxy.EndpointProjection, *llmproxy.ModelAliasProjection, error) {
	db := r.db.WithContext(ctx)
	models, err := r.modelDAO.BatchGet(db, &dbmodel.Model{Alias: alias}, constant.ModelRepoFieldsFull)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "find models by alias")
	}
	if len(models) == 0 {
		return nil, nil, nil
	}
	m := models[rand.Intn(len(models))]
	ep, err := r.endpointDAO.Get(db, &dbmodel.Endpoint{ID: m.EndpointID}, constant.EndpointRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "find endpoint by id")
	}
	return &llmproxy.EndpointProjection{
		ID:                          ep.ID,
		Name:                        ep.Name,
		OpenaiBaseURL:               ep.OpenaiBaseURL,
		AnthropicBaseURL:            ep.AnthropicBaseURL,
		APIKey:                      ep.APIKey,
		SupportOpenAIChatCompletion: ep.SupportOpenAIChatCompletion,
		SupportOpenAIResponse:       ep.SupportOpenAIResponse,
		SupportAnthropicMessage:     ep.SupportAnthropicMessage,
	}, &llmproxy.ModelAliasProjection{Alias: m.ModelName}, nil
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

---

### Task 7: 重写 EndpointResolver

**Files:**
- Rewrite: `internal/domain/llmproxy/service/resolver.go`

- [ ] **Step 1: 重写 `resolver.go`**

```go
package service

import (
	"context"
	"math/rand"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

type EndpointResolver interface {
	Resolve(ctx context.Context, alias vo.EndpointAlias) (*aggregate.Endpoint, *aggregate.Model, error)
}

type endpointResolver struct {
	endpointRepo llmproxy.EndpointRepository
	modelRepo    llmproxy.ModelRepository
}

func NewEndpointResolver(
	endpointRepo llmproxy.EndpointRepository,
	modelRepo llmproxy.ModelRepository,
) EndpointResolver {
	return &endpointResolver{
		endpointRepo: endpointRepo,
		modelRepo:    modelRepo,
	}
}

func (r *endpointResolver) Resolve(ctx context.Context, alias vo.EndpointAlias) (*aggregate.Endpoint, *aggregate.Model, error) {
	if alias.IsEmpty() {
		return nil, nil, ierr.New(ierr.ErrValidation, "endpoint alias is empty")
	}
	models, err := r.modelRepo.FindByAlias(ctx, alias)
	if err != nil {
		return nil, nil, err
	}
	if len(models) == 0 {
		return nil, nil, ierr.Newf(ierr.ErrDataNotExists, "model %q not found", alias.String())
	}
	m := models[rand.Intn(len(models))]
	ep, err := r.endpointRepo.FindByID(ctx, m.EndpointID())
	if err != nil {
		return nil, nil, err
	}
	if ep == nil {
		return nil, nil, ierr.Newf(ierr.ErrDataNotExists, "endpoint %d not found for model %q", m.EndpointID(), alias.String())
	}
	return ep, m, nil
}
```

---

### Task 8: 更新 Query Usecases（ListModels / CountTokens）

**Files:**
- Rewrite: `internal/application/llmproxy/usecase/query.go`

- [ ] **Step 1: 重写 `query.go`**

```go
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// ListOpenAIModels 不再按 provider filter，返回所有 alias
type ListOpenAIModels interface {
	Handle(ctx context.Context) (*dto.OpenAIListModelsRsp, error)
}

type listOpenAIModels struct {
	readRepo llmproxy.EndpointReadRepository
}

func NewListOpenAIModels(readRepo llmproxy.EndpointReadRepository) ListOpenAIModels {
	return &listOpenAIModels{readRepo: readRepo}
}

func (q *listOpenAIModels) Handle(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	projections, err := q.readRepo.ListAliases(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[OpenAIQuery] Failed to query models", zap.Error(err))
		return &dto.OpenAIListModelsRsp{Object: constant.OpenAIListObject, Data: []*dto.OpenAIModel{}}, nil
	}
	return &dto.OpenAIListModelsRsp{
		Object: constant.OpenAIListObject,
		Data: lo.Map(projections, func(p *llmproxy.ModelAliasProjection, _ int) *dto.OpenAIModel {
			return &dto.OpenAIModel{
				ID:      p.Alias,
				Created: time.Now().Unix(),
				Object:  constant.OpenAIModelObject,
				OwnedBy: constant.OpenAIModelOwnedBy,
			}
		}),
	}, nil
}

type ListAnthropicModels interface {
	Handle(ctx context.Context) (*dto.AnthropicListModelsRsp, error)
}

type listAnthropicModels struct {
	readRepo llmproxy.EndpointReadRepository
}

func NewListAnthropicModels(readRepo llmproxy.EndpointReadRepository) ListAnthropicModels {
	return &listAnthropicModels{readRepo: readRepo}
}

func (q *listAnthropicModels) Handle(ctx context.Context) (*dto.AnthropicListModelsRsp, error) {
	projections, err := q.readRepo.ListAliases(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[AnthropicQuery] Failed to query models", zap.Error(err))
		return &dto.AnthropicListModelsRsp{Data: []*dto.AnthropicModelInfo{}}, nil
	}
	models := lo.Map(projections, func(p *llmproxy.ModelAliasProjection, _ int) *dto.AnthropicModelInfo {
		return &dto.AnthropicModelInfo{
			ID:          p.Alias,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			DisplayName: p.Alias,
			Type:        constant.AnthropicModelType,
		}
	})
	rsp := &dto.AnthropicListModelsRsp{Data: models, HasMore: false}
	if len(models) > 0 {
		rsp.FirstID = models[0].ID
		rsp.LastID = models[len(models)-1].ID
	}
	return rsp, nil
}

// CountTokens 不再按 provider 过滤
type CountTokens interface {
	Handle(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error)
}

type countTokens struct {
	readRepo llmproxy.EndpointReadRepository
	proxy    llmproxy.AnthropicProxy  // 需要改类型或保持现有
}

func NewCountTokens(readRepo llmproxy.EndpointReadRepository, proxy llmproxy.AnthropicProxy) CountTokens {
	return &countTokens{readRepo: readRepo, proxy: proxy}
}

func (q *countTokens) Handle(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
	log := logger.WithCtx(ctx)

	epProj, modelNameProj, err := q.readRepo.FindEndpointByAlias(ctx, req.Body.Model)
	if err != nil {
		log.Warn("[AnthropicQuery] Model lookup error, returning 0", zap.String("model", req.Body.Model), zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}
	if epProj == nil {
		log.Warn("[AnthropicQuery] Model not found, returning 0", zap.String("model", req.Body.Model))
		return &dto.AnthropicTokensCount{}, nil
	}

	if !epProj.SupportAnthropicMessage {
		log.Warn("[AnthropicQuery] Endpoint does not support messages API", zap.String("model", req.Body.Model))
		return &dto.AnthropicTokensCount{}, nil
	}

	upstream := vo.UpstreamEndpoint{
		Model:   modelNameProj.Alias,
		APIKey:  epProj.APIKey,
		BaseURL: epProj.AnthropicBaseURL,
	}
	body := util.MarshalAnthropicCountTokensBodyForModel(req.Body, upstream.Model)

	rsp, err := q.proxy.ForwardCountTokens(ctx, upstream, body)
	if err != nil {
		log.Warn("[AnthropicQuery] Count tokens error, returning 0", zap.Error(err))
		return &dto.AnthropicTokensCount{}, nil
	}
	return rsp, nil
}
```

注意：`llmproxy.AnthropicProxy` 类型在 `infrastructure/transport` 中定义。检查是否在 domain 层有对应接口。如果没有，需在 `repository.go` 添加或保留原有 import。实际编译后按需调整 import。

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

---

### Task 9: 更新 OpenAI Usecase

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai.go`
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
- Modify: `internal/application/llmproxy/usecase/openai_response.go`
- Modify: `internal/application/llmproxy/usecase/common.go`

- [ ] **Step 1: 重构 `openai.go`**

核心变更：
1. `Resolve` 调用改为 `Resolve(ctx, alias)`，不再传 provider
2. `CreateChatCompletion` 中检查 `ep.SupportOpenAIChatCompletion`，不符合则返回错误
3. `CreateResponse` 中检查 `ep.SupportOpenAIResponse`
4. `toTransportEndpoint` 改为从 `aggregate.Model` 和 `aggregate.Endpoint` 构建

```go
package usecase

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

var openAIInternalErrorBody = lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
	Error: &dto.OpenAIError{Message: constant.OpenAIInternalErrorMessage, Type: constant.OpenAIInternalErrorType, Code: constant.OpenAIInternalErrorCode},
}))

type OpenAIUseCase interface {
	ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error)
	CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
	CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error)
}

type openAIUseCase struct {
	resolver       service.EndpointResolver
	modelsQuery    ListOpenAIModels
	openAIProxy    transport.OpenAIProxy
	anthropicProxy transport.AnthropicProxy
	taskSubmitter  TaskSubmitter
}

func NewOpenAIUseCase(
	resolver service.EndpointResolver,
	modelsQuery ListOpenAIModels,
	openAIProxy transport.OpenAIProxy,
	anthropicProxy transport.AnthropicProxy,
	taskSubmitter TaskSubmitter,
) OpenAIUseCase {
	return &openAIUseCase{
		resolver:       resolver,
		modelsQuery:    modelsQuery,
		openAIProxy:    openAIProxy,
		anthropicProxy: anthropicProxy,
		taskSubmitter:  taskSubmitter,
	}
}

func (u *openAIUseCase) ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	return u.modelsQuery.Handle(ctx)
}

func (u *openAIUseCase) CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	ep, m, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model))
	if err != nil {
		log.Error("[OpenAIUseCase] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}
	if !ep.SupportOpenAIChatCompletion() {
		log.Error("[OpenAIUseCase] Endpoint does not support chat completion", zap.String("model", req.Body.Model))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, false) // false = OpenAI protocol
	return u.forwardChatNative(ctx, req, m, ep, upstream, stream), nil
}

func (u *openAIUseCase) CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	model := lo.FromPtr(req.Body.Model)
	ep, m, err := u.resolver.Resolve(ctx, vo.EndpointAlias(model))
	if err != nil {
		log.Error("[OpenAIUseCase] Response API model not found", zap.String("model", model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(model), nil
	}
	if !ep.SupportOpenAIResponse() {
		log.Error("[OpenAIUseCase] Endpoint does not support response API", zap.String("model", model))
		return util.SendOpenAIModelNotFoundError(model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, false)
	return u.forwardResponseNative(ctx, req, m, ep, upstream, stream), nil
}

// toTransportEndpoint 从 Model + Endpoint + 协议构造 UpstreamEndpoint
// isAnthropic=true 取 AnthropicBaseURL, 否则取 OpenaiBaseURL
func toTransportEndpoint(m *aggregate.Model, ep *aggregate.Endpoint, isAnthropic bool) vo.UpstreamEndpoint {
	var baseURL string
	if isAnthropic {
		baseURL = ep.AnthropicBaseURL()
	} else {
		baseURL = ep.OpenaiBaseURL()
	}
	return vo.NewUpstreamEndpointFromCredential(m.ModelName(), ep.APIKey(), baseURL)
}
```

- [ ] **Step 2: 重构 `openai_chat.go`**

关键变更：
1. 所有 `forward*` 方法签名：参数 `ep *aggregate.Endpoint` 改为 `m *aggregate.Model` + `ep *aggregate.Endpoint`
2. 审计 `ModelID` 改为 `m.AggregateID()`
3. `UpstreamProvider` 不再使用 `ep.Provider()`，改为 `enum.ProviderOpenAI`（固定）
4. 移除 `forwardChatViaAnthropic` 及所有 cross-protocol 转发代码

更新所有 `func (u *openAIUseCase)` 方法签名中的 `ep *aggregate.Endpoint` → `m *aggregate.Model, ep *aggregate.Endpoint`：

- `forwardChatNative` → 签名新增 `m *aggregate.Model`
- `forwardChatNativeStream` → 签名新增 `m *aggregate.Model`，审计 `ModelID: m.AggregateID()`, `UpstreamProvider: enum.ProviderOpenAI`
- `forwardChatNativeUnary` → 同上
- 删除 `forwardChatViaAnthropic`, `forwardChatViaAnthropicStream`, `forwardChatViaAnthropicUnary`

示例（`forwardChatNativeStream` 的审计部分变更）：
```go
task := &dto.ModelCallAuditTask{
    Ctx:                 util.CopyContextValues(ctx),
    ModelID:             m.AggregateID(),
    Model:               req.Body.Model,
    UpstreamProvider:    enum.ProviderOpenAI,
    APIProvider:         enum.ProviderOpenAI,
    FirstTokenLatencyMs: firstTokenLatencyMs,
    StreamDurationMs:    streamDurationMs,
}
```

- [ ] **Step 3: 重构 `openai_response.go`**

同上：
- 所有 `forwardResponse*` 方法签名增加 `m *aggregate.Model`
- 审计 `ModelID: m.AggregateID()`, `UpstreamProvider: enum.ProviderOpenAI`
- 删除 `forwardResponseViaAnthropic`, `forwardResponseViaAnthropicStream`, `forwardResponseViaAnthropicUnary`

- [ ] **Step 4: 重构 `common.go`**

```go
package usecase

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func auditFailure(submitter TaskSubmitter, ctx context.Context, m *aggregate.Model, exposedModel string, apiProvider enum.ProviderType, totalMs int64, err error) {
	task := &dto.ModelCallAuditTask{
		Ctx:                 util.CopyContextValues(ctx),
		ModelID:             m.AggregateID(),
		Model:               exposedModel,
		UpstreamProvider:    apiProvider,
		APIProvider:         apiProvider,
		FirstTokenLatencyMs: totalMs,
	}
	task.UpstreamStatusCode, task.ErrorMessage = util.ExtractUpstreamStatusAndError(err)
	_ = submitter.SubmitModelCallAuditTask(task)
}
```

- [ ] **Step 5: 编译验证**

```bash
go build ./...
```

---

### Task 10: 更新 Anthropic Usecase

**Files:**
- Modify: `internal/application/llmproxy/usecase/anthropic.go`

- [ ] **Step 1: 重构 `anthropic.go`**

核心变更同 OpenAI：
1. `CreateMessage` 调用 `resolver.Resolve(ctx, alias)`，检查 `ep.SupportAnthropicMessage()`
2. 所有转发方法签名新增 `m *aggregate.Model`
3. 审计 `ModelID: m.AggregateID()`
4. `UpstreamProvider: enum.ProviderAnthropic`
5. 移除 cross-protocol 转发（`forwardMessageViaOpenAI` 等）

`CreateMessage` 新逻辑：
```go
func (u *anthropicUseCase) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	log := logger.WithCtx(ctx)

	ep, m, err := u.resolver.Resolve(ctx, vo.EndpointAlias(req.Body.Model))
	if err != nil {
		log.Error("[AnthropicUseCase] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}
	if !ep.SupportAnthropicMessage() {
		log.Error("[AnthropicUseCase] Endpoint does not support messages API", zap.String("model", req.Body.Model))
		return util.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}

	stream := req.Body.Stream != nil && *req.Body.Stream
	exposedModel := req.Body.Model
	upstream := toTransportEndpoint(m, ep, true) // true = Anthropic protocol
	return u.forwardMessageNative(ctx, req, m, ep, upstream, exposedModel, stream), nil
}
```

签名变更示例：
```go
func (u *anthropicUseCase) forwardMessageNative(
    ctx context.Context,
    req *dto.AnthropicCreateMessageRequest,
    m *aggregate.Model,
    ep *aggregate.Endpoint,
    upstream vo.UpstreamEndpoint,
    exposedModel string,
    stream bool,
) *huma.StreamResponse
```

所有 `forwardMessage*` 方法签名增加 `m *aggregate.Model`。审计变更：
```go
task := &dto.ModelCallAuditTask{
    Ctx:                 util.CopyContextValues(ctx),
    ModelID:             m.AggregateID(),
    ...
    UpstreamProvider:    enum.ProviderAnthropic,
    APIProvider:         enum.ProviderAnthropic,
    ...
}
```

删除 `forwardMessageViaOpenAI`, `forwardMessageViaOpenAIStream`, `forwardMessageViaOpenAIUnary`。

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

---

### Task 11: 更新 Constants

**Files:**
- Modify: `internal/common/constant/sql.go`
- Modify: `internal/common/constant/database.go`

- [ ] **Step 1: 更新 `sql.go`**

```go
EndpointRepoFieldsFull = []string{FieldID, FieldName, FieldOpenaiBaseURL, FieldAnthropicBaseURL, FieldAPIKey,
    FieldSupportOpenAIChatCompletion, FieldSupportOpenAIResponse, FieldSupportAnthropicMessage}

ModelRepoFieldsFull  = []string{FieldID, FieldAlias, FieldModel, FieldEndpointID}
ModelRepoFieldsAlias = []string{FieldAlias}
```

添加需要的常量：
```go
FieldName                        = "name"           // 已有 FieldName（第19行）, 确认复用
FieldOpenaiBaseURL               = "openai_base_url"
FieldAnthropicBaseURL            = "anthropic_base_url"
FieldSupportOpenAIChatCompletion = "support_openai_chat_completion"
FieldSupportOpenAIResponse       = "support_openai_response"
FieldSupportAnthropicMessage     = "support_anthropic_message"
FieldEndpointID                  = "endpoint_id"
```

- [ ] **Step 2: 更新 `database.go`**

添加：
```go
AggregateTypeModel = "llmproxy.model"
```

删除旧的 `EndpointRepoFieldsAlias`, `EndpointRepoFieldsCredential`（DAO 层面已用 ModelRepoFieldsAlias 等代替）。

---

### Task 12: 更新 Container DI

**Files:**
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: 更新 `container.go`**

`newEndpointResolver` 需要接收 `ModelRepository` 参数：
```go
func newEndpointRepository(db *gorm.DB) llmproxy.EndpointRepository {
	return repository.NewEndpointRepository(db)
}

func newModelRepository(db *gorm.DB) llmproxy.ModelRepository {
	return repository.NewModelRepository(db)
}

func newEndpointResolver(repo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository) llmproxyservice.EndpointResolver {
	return llmproxyservice.NewEndpointResolver(repo, modelRepo)
}
```

在 `provideInfrastructure` 中添加：
```go
if err := container.Provide(newModelRepository); err != nil {
    return err
}
```

删除旧的 `newEndpointReadRepository`（现在用新的 `ListAliases` 接口）。或者保留但修改为新的读仓储实现。建议保留依赖注入注册，修改实现：

```go
func newEndpointReadRepository(db *gorm.DB) llmproxy.EndpointReadRepository {
	return repository.NewEndpointReadRepository(db)
}
```

--- 

### Task 13: 删除旧的 unused 常量引用

**Files:**
- Modify: `internal/common/constant/sql.go`

删除不再使用的常量：
```go
// 删除
EndpointRepoFieldsAlias      = []string{FieldAlias}
EndpointRepoFieldsCredential = []string{FieldModel, FieldAPIKey, FieldBaseURL}
```

已经删除的 `FieldProvider` 从 `EndpointRepoFieldsFull` 中移除。

---

### Task 14: 运行 Lint + Test

- [ ] **Step 1: Lint**

```bash
make lint
```

- [ ] **Step 2: 全量编译**

```bash
go build ./...
```

- [ ] **Step 3: 全量测试（不包含 E2E）**

```bash
go test -count=1 ./test/unit/...
```

如果失败，逐个修复测试文件。

---

## 测试更新指南（不逐一列出代码）

测试文件需要更新的内容：

| 测试文件 | 变更 |
|---|---|
| `test/unit/endpoint_resolver/` | resolver 接口改为 `Resolve(ctx, alias)` 返回 `(endpoint, model, err)`。fixture 用例删除 provider 相关字段。stub repo 改为实现新接口。 |
| `test/unit/proxy_config/` | 适配新 DAO 和新表结构 |
| `test/unit/llmproxy_usecase/` | mock resolver 改为新签名。mock endpoint 改为新 aggregate。mock 实现 `Resolve(ctx, alias)` 返回 `(agg.Endpoint, agg.Model, nil)` |
| `test/unit/unified_response/` | `ModelCallAuditTask` 的 `ModelID` 和 `UpstreamProvider` 语义不变，不需要改 |

---

## 自审检查项

- [x] 每个 task 都有可独立编译验证的步骤
- [x] 删除了 `ModelEndpoint` 旧模型、旧 DAO
- [x] 新 `aggregate.Endpoint` 不再包含 `alias`、`provider`、`UpstreamCreds`
- [x] 新 `aggregate.Model` 包含 `alias`、`modelName`、`endpointID`
- [x] `EndpointResolver.Resolve` 不再区分 primary/fallback provider
- [x] 两个 UseCase 都移除了 cross-protocol 转发代码
- [x] 审计 `UpstreamProvider` 改为固定等于 `APIProvider`
- [x] 表名没有冲突（GORM 自动复数化 `model` → `models`，`endpoint` → `endpoints`）
