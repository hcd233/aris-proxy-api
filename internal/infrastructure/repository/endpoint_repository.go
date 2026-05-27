package repository

import (
	"context"
	"errors"
	"math/rand"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
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
	ep, err := aggregate.CreateEndpoint(
		m.ID,
		m.Name,
		m.OpenaiBaseURL,
		m.AnthropicBaseURL,
		m.APIKey,
		m.SupportOpenAIChatCompletion,
		m.SupportOpenAIResponse,
		m.SupportAnthropicMessage,
	)
	if err != nil {
		return nil, err
	}
	ep.SetTimestamps(m.CreatedAt, m.UpdatedAt)
	return ep, nil
}

func toEndpointModel(ep *aggregate.Endpoint) *dbmodel.Endpoint {
	return &dbmodel.Endpoint{
		ID:                          ep.AggregateID(),
		Name:                        ep.Name(),
		OpenaiBaseURL:               ep.OpenaiBaseURL(),
		AnthropicBaseURL:            ep.AnthropicBaseURL(),
		APIKey:                      ep.APIKey(),
		SupportOpenAIChatCompletion: ep.SupportOpenAIChatCompletion(),
		SupportOpenAIResponse:       ep.SupportOpenAIResponse(),
		SupportAnthropicMessage:     ep.SupportAnthropicMessage(),
	}
}

// Create 创建端点
func (r *endpointRepository) Create(ctx context.Context, ep *aggregate.Endpoint) (uint, error) {
	db := r.db.WithContext(ctx)
	m := toEndpointModel(ep)
	if err := db.Create(m).Error; err != nil {
		return 0, ierr.Wrap(ierr.ErrDBCreate, err, "create endpoint")
	}
	return m.ID, nil
}

// Update 更新端点（仅更新非零值字段）
func (r *endpointRepository) Update(ctx context.Context, ep *aggregate.Endpoint) error {
	db := r.db.WithContext(ctx)
	updates := map[string]any{
		constant.FieldEndpointName:                        ep.Name(),
		constant.FieldEndpointOpenaiBaseURL:               ep.OpenaiBaseURL(),
		constant.FieldEndpointAnthropicBaseURL:            ep.AnthropicBaseURL(),
		constant.FieldEndpointAPIKey:                      ep.APIKey(),
		constant.FieldEndpointSupportOpenAIChatCompletion: ep.SupportOpenAIChatCompletion(),
		constant.FieldEndpointSupportOpenAIResponse:       ep.SupportOpenAIResponse(),
		constant.FieldEndpointSupportAnthropicMessage:     ep.SupportAnthropicMessage(),
	}
	if err := db.Model(&dbmodel.Endpoint{}).Where(constant.WhereIDEquals, ep.AggregateID()).Updates(updates).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update endpoint")
	}
	return nil
}

// Delete 删除端点
func (r *endpointRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	if err := db.Delete(&dbmodel.Endpoint{}, id).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete endpoint")
	}
	return nil
}

// List 列出所有端点
func (r *endpointRepository) List(ctx context.Context) ([]*aggregate.Endpoint, error) {
	db := r.db.WithContext(ctx)
	var models []*dbmodel.Endpoint
	if err := db.Find(&models).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list endpoints")
	}
	result := make([]*aggregate.Endpoint, 0, len(models))
	for _, m := range models {
		ep, err := toEndpointAggregate(m)
		if err != nil {
			return nil, err
		}
		result = append(result, ep)
	}
	return result, nil
}

// Paginate 分页查询端点列表
func (r *endpointRepository) Paginate(ctx context.Context, param llmproxy.PageParam) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	records, pageInfo, err := r.endpointDAO.Paginate(
		db,
		&dbmodel.Endpoint{},
		constant.EndpointRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldName}},
			SortParam:  dao.SortParam{Sort: enum.Sort(param.Sort), SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate endpoints")
	}
	out := make([]*aggregate.Endpoint, 0, len(records))
	for _, m := range records {
		ep, convErr := toEndpointAggregate(m)
		if convErr != nil {
			return nil, nil, convErr
		}
		out = append(out, ep)
	}
	return out, pageInfo, nil
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
	model, err := aggregate.CreateModel(m.ID, vo.EndpointAlias(m.Alias), m.ModelName, m.EndpointID)
	if err != nil {
		return nil, err
	}
	model.SetTimestamps(m.CreatedAt, m.UpdatedAt)
	return model, nil
}

func toModelDBModel(m *aggregate.Model) *dbmodel.Model {
	return &dbmodel.Model{
		ID:         m.AggregateID(),
		Alias:      m.Alias().String(),
		ModelName:  m.ModelName(),
		EndpointID: m.EndpointID(),
	}
}

// FindByID 按 ID 查询模型
func (r *modelRepository) FindByID(ctx context.Context, id uint) (*aggregate.Model, error) {
	db := r.db.WithContext(ctx)
	m, err := r.dao.Get(db, &dbmodel.Model{ID: id}, constant.ModelRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find model by id")
	}
	return toModelAggregate(m)
}

// Create 创建模型
func (r *modelRepository) Create(ctx context.Context, m *aggregate.Model) (uint, error) {
	db := r.db.WithContext(ctx)
	mdl := toModelDBModel(m)
	if err := db.Create(mdl).Error; err != nil {
		return 0, ierr.Wrap(ierr.ErrDBCreate, err, "create model")
	}
	return mdl.ID, nil
}

// Update 更新模型（仅更新非零值字段）
func (r *modelRepository) Update(ctx context.Context, m *aggregate.Model) error {
	db := r.db.WithContext(ctx)
	updates := map[string]any{
		constant.FieldModelAlias:      m.Alias().String(),
		constant.FieldModelModelName:  m.ModelName(),
		constant.FieldModelEndpointID: m.EndpointID(),
	}
	if err := db.Model(&dbmodel.Model{}).Where(constant.WhereIDEquals, m.AggregateID()).Updates(updates).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update model")
	}
	return nil
}

// Delete 删除模型
func (r *modelRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	if err := db.Delete(&dbmodel.Model{}, id).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete model")
	}
	return nil
}

// List 列出所有模型
func (r *modelRepository) List(ctx context.Context) ([]*aggregate.Model, error) {
	db := r.db.WithContext(ctx)
	var models []*dbmodel.Model
	if err := db.Find(&models).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list models")
	}
	result := make([]*aggregate.Model, 0, len(models))
	for _, m := range models {
		agg, err := toModelAggregate(m)
		if err != nil {
			return nil, err
		}
		result = append(result, agg)
	}
	return result, nil
}

// Paginate 分页查询模型列表
func (r *modelRepository) Paginate(ctx context.Context, param llmproxy.PageParam) ([]*aggregate.Model, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Model{},
		constant.ModelRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldAlias, constant.FieldModelModelName}},
			SortParam:  dao.SortParam{Sort: enum.Sort(param.Sort), SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate models")
	}
	out := make([]*aggregate.Model, 0, len(records))
	for _, m := range records {
		agg, convErr := toModelAggregate(m)
		if convErr != nil {
			return nil, nil, convErr
		}
		out = append(out, agg)
	}
	return out, pageInfo, nil
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

// FindEndpointByAlias 按 alias 随机选满足 matcher 的 endpoint，返回端点信息 + 上游模型名。
func (r *endpointReadRepository) FindEndpointByAlias(ctx context.Context, alias string, matcher func(*llmproxy.EndpointProjection) bool) (*llmproxy.EndpointProjection, *llmproxy.ModelAliasProjection, error) {
	db := r.db.WithContext(ctx)
	models, err := r.modelDAO.BatchGet(db, &dbmodel.Model{Alias: alias}, constant.ModelRepoFieldsFull)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "find models by alias")
	}
	if len(models) == 0 {
		return nil, nil, nil
	}
	for _, idx := range rand.Perm(len(models)) {
		m := models[idx]
		ep, getErr := r.endpointDAO.Get(db, &dbmodel.Endpoint{ID: m.EndpointID}, constant.EndpointRepoFieldsFull)
		if getErr != nil {
			if errors.Is(getErr, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, nil, ierr.Wrap(ierr.ErrDBQuery, getErr, "find endpoint by id")
		}
		proj := toEndpointProjection(ep)
		if matcher == nil || matcher(proj) {
			return proj, &llmproxy.ModelAliasProjection{Alias: m.ModelName}, nil
		}
	}
	return nil, nil, nil
}

func toEndpointProjection(ep *dbmodel.Endpoint) *llmproxy.EndpointProjection {
	return &llmproxy.EndpointProjection{
		ID:                          ep.ID,
		Name:                        ep.Name,
		OpenaiBaseURL:               ep.OpenaiBaseURL,
		AnthropicBaseURL:            ep.AnthropicBaseURL,
		APIKey:                      ep.APIKey,
		SupportOpenAIChatCompletion: ep.SupportOpenAIChatCompletion,
		SupportOpenAIResponse:       ep.SupportOpenAIResponse,
		SupportAnthropicMessage:     ep.SupportAnthropicMessage,
	}
}
