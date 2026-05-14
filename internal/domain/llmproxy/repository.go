package llmproxy

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// EndpointRepository Endpoint 聚合根仓储接口
type EndpointRepository interface {
	FindByID(ctx context.Context, id uint) (*aggregate.Endpoint, error)
}

// ModelRepository Model 聚合根仓储接口
type ModelRepository interface {
	FindByAlias(ctx context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error)
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
