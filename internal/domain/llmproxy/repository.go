package llmproxy

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// EndpointRepository Endpoint 聚合根仓储接口
type EndpointRepository interface {
	FindByID(ctx context.Context, id uint) (*aggregate.Endpoint, error)
	BatchFindByIDs(ctx context.Context, ids []uint) (map[uint]*aggregate.Endpoint, error)
	Create(ctx context.Context, endpoint *aggregate.Endpoint) (uint, error)
	Update(ctx context.Context, endpoint *aggregate.Endpoint) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context) ([]*aggregate.Endpoint, error)
	Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Endpoint, *model.PageInfo, error)
}

// ModelRepository Model 聚合根仓储接口
type ModelRepository interface {
	FindByAlias(ctx context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error)
	FindByID(ctx context.Context, id uint) (*aggregate.Model, error)
	Create(ctx context.Context, model *aggregate.Model) (uint, error)
	Update(ctx context.Context, model *aggregate.Model) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context) ([]*aggregate.Model, error)
	Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Model, *model.PageInfo, error)
}

// ==================== CQRS 读模型 ====================

// ModelAliasProjection 模型别名只读投影
type ModelAliasProjection struct {
	Alias string
}

// EndpointProjection 端点只读投影
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

// EndpointReadRepository CQRS 读模型仓储接口
type EndpointReadRepository interface {
	ListAliases(ctx context.Context) ([]*ModelAliasProjection, error)
	FindEndpointByAlias(ctx context.Context, alias string, matcher func(*EndpointProjection) bool) (*EndpointProjection, *ModelAliasProjection, error)
}
