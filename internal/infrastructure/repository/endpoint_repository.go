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

// endpointRepository EndpointRepository 的 GORM 实现
type endpointRepository struct {
	endpointDAO *dao.EndpointDAO
	db          *gorm.DB
}

// NewEndpointRepository 构造 EndpointRepository
func NewEndpointRepository(db *gorm.DB) llmproxy.EndpointRepository {
	return &endpointRepository{endpointDAO: dao.GetEndpointDAO(), db: db}
}

// FindByID 按 ID 查询端点
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

// modelRepository ModelRepository 的 GORM 实现
type modelRepository struct {
	dao *dao.ModelDAO
	db  *gorm.DB
}

// NewModelRepository 构造 ModelRepository
func NewModelRepository(db *gorm.DB) llmproxy.ModelRepository {
	return &modelRepository{dao: dao.GetModelDAO(), db: db}
}

// FindByAlias 按 alias 查询所有关联的模型记录
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

func toModelAggregate(m *dbmodel.Model) (*aggregate.Model, error) {
	return aggregate.CreateModel(m.ID, vo.EndpointAlias(m.Alias), m.ModelName, m.EndpointID)
}

// ==================== CQRS 读模型实现 ====================

type endpointReadRepository struct {
	endpointDAO *dao.EndpointDAO
	modelDAO    *dao.ModelDAO
	db          *gorm.DB
}

// NewEndpointReadRepository 构造 EndpointReadRepository
func NewEndpointReadRepository(db *gorm.DB) llmproxy.EndpointReadRepository {
	return &endpointReadRepository{
		endpointDAO: dao.GetEndpointDAO(),
		modelDAO:    dao.GetModelDAO(),
		db:          db,
	}
}

// ListAliases 查询所有不重复的模型别名
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

// FindEndpointByAlias 按 alias 随机选 endpoint，返回端点信息 + 上游模型名
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
