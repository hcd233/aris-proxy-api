package llmproxy

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// EndpointRepository Endpoint 聚合根仓储接口
//
// scopeUserID 语义（多租户隔离）：>0 时查询/删除限定在该用户名下；==0（admin 视角）不过滤。
type EndpointRepository interface {
	FindByID(ctx context.Context, id, scopeUserID uint) (*aggregate.Endpoint, error)
	BatchFindByIDs(ctx context.Context, ids []uint) (map[uint]*aggregate.Endpoint, error)
	Create(ctx context.Context, endpoint *aggregate.Endpoint, ownerUserID uint) (uint, error)
	Update(ctx context.Context, endpoint *aggregate.Endpoint) error // 归属校验由调用方 FindByID 完成
	Delete(ctx context.Context, id, scopeUserID uint) error
	DeleteCascade(ctx context.Context, id, scopeUserID uint) error
	List(ctx context.Context) ([]*aggregate.Endpoint, error)
	Paginate(ctx context.Context, param model.CommonParam, scopeUserID uint) ([]*aggregate.Endpoint, *model.PageInfo, error)
}

// ModelRepository Model 聚合根仓储接口
//
// scopeUserID 语义同 EndpointRepository；FindByAlias 为网关解析专用，userID 必传真实值。
type ModelRepository interface {
	FindByAlias(ctx context.Context, alias vo.EndpointAlias, userID uint) ([]*aggregate.Model, error)
	FindByID(ctx context.Context, id, scopeUserID uint) (*aggregate.Model, error)
	Create(ctx context.Context, model *aggregate.Model, ownerUserID uint) (uint, error)
	Update(ctx context.Context, model *aggregate.Model) error
	Delete(ctx context.Context, id, scopeUserID uint) error
	DeleteByEndpointID(ctx context.Context, endpointID uint) error // 级联删除前调用方已校验端点归属
	List(ctx context.Context) ([]*aggregate.Model, error)
	Paginate(ctx context.Context, param model.CommonParam, scopeUserID uint) ([]*aggregate.Model, *model.PageInfo, error)
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
//
// userID 语义：网关路径必传真实用户 ID；0 不过滤（仅限 admin 内部用途）。
type EndpointReadRepository interface {
	ListAliases(ctx context.Context, userID uint) ([]*ModelAliasProjection, error)
	FindEndpointByAlias(ctx context.Context, userID uint, alias string, matcher func(*EndpointProjection) bool) (*EndpointProjection, *ModelAliasProjection, error)
}
