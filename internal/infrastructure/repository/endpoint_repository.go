package repository

import (
	"context"
	"errors"
	"math/rand"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
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
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// endpointRepository EndpointRepository 的 GORM 实现
type endpointRepository struct {
	endpointDAO *dao.EndpointDAO
	modelDAO    *dao.ModelDAO
	db          *gorm.DB
}

// NewEndpointRepository 构造 EndpointRepository
func NewEndpointRepository(db *gorm.DB) llmproxy.EndpointRepository {
	return &endpointRepository{endpointDAO: dao.GetEndpointDAO(), modelDAO: dao.GetModelDAO(), db: db}
}

// FindByID 按 ID 查询端点（scopeUserID>0 时限定在该用户名下）
func (r *endpointRepository) FindByID(ctx context.Context, id, scopeUserID uint) (*aggregate.Endpoint, error) {
	db := r.db.WithContext(ctx)
	ep, err := r.endpointDAO.Get(db, &dbmodel.Endpoint{ID: id, UserID: scopeUserID}, constant.EndpointRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find endpoint by id")
	}
	return toEndpointAggregate(ep)
}

// BatchFindByIDs 按 ID 集合一次性查询端点，返回以 ID 索引的 map；ids 为空时返回空 map 且不打 SQL。
func (r *endpointRepository) BatchFindByIDs(ctx context.Context, ids []uint) (map[uint]*aggregate.Endpoint, error) {
	out := make(map[uint]*aggregate.Endpoint, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	db := r.db.WithContext(ctx)
	records, err := r.endpointDAO.BatchGetByField(db, constant.FieldID, ids, constant.EndpointRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch find endpoints by ids")
	}
	for _, rec := range records {
		ep, convErr := toEndpointAggregate(rec)
		if convErr != nil {
			return nil, convErr
		}
		out[rec.ID] = ep
	}
	return out, nil
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
	ep.SetUserID(m.UserID)
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

// Create 创建端点（写入归属 ownerUserID）
func (r *endpointRepository) Create(ctx context.Context, ep *aggregate.Endpoint, ownerUserID uint) (uint, error) {
	db := r.db.WithContext(ctx)
	m := toEndpointModel(ep)
	m.UserID = ownerUserID
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

// Delete 删除端点（软删除；scopeUserID>0 时限定在该用户名下）
func (r *endpointRepository) Delete(ctx context.Context, id, scopeUserID uint) error {
	db := r.db.WithContext(ctx)
	if err := r.endpointDAO.Delete(db, &dbmodel.Endpoint{ID: id, UserID: scopeUserID}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete endpoint")
	}
	return nil
}

// DeleteCascade 级联删除端点及其关联模型（事务保护；scopeUserID>0 时限定在该用户名下）
func (r *endpointRepository) DeleteCascade(ctx context.Context, id, scopeUserID uint) error {
	db := r.db.WithContext(ctx)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := r.modelDAO.BatchDeleteByField(tx, constant.FieldEndpointID, []uint{id}); err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "cascade delete models by endpoint id")
		}
		if err := r.endpointDAO.Delete(tx, &dbmodel.Endpoint{ID: id, UserID: scopeUserID}); err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "delete endpoint")
		}
		return nil
	})
}

// List 列出所有端点
func (r *endpointRepository) List(ctx context.Context) ([]*aggregate.Endpoint, error) {
	db := r.db.WithContext(ctx)
	var models []*dbmodel.Endpoint
	if err := db.Find(&models).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list endpoints")
	}
	return util.MapErr(models, func(m *dbmodel.Endpoint, _ int) (*aggregate.Endpoint, error) {
		return toEndpointAggregate(m)
	})
}

// Paginate 分页查询端点列表
//
//	@author centonhuang
//	@update 2026-05-27 10:00:00
func (r *endpointRepository) Paginate(ctx context.Context, param model.CommonParam, scopeUserID uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	records, pageInfo, err := r.endpointDAO.Paginate(
		db,
		&dbmodel.Endpoint{UserID: scopeUserID},
		constant.EndpointRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldName}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate endpoints")
	}
	out, convErr := util.MapErr(records, func(m *dbmodel.Endpoint, _ int) (*aggregate.Endpoint, error) {
		return toEndpointAggregate(m)
	})
	if convErr != nil {
		return nil, nil, convErr
	}
	return out, pageInfo, nil
}

// FindIDsByScope 按租户范围返回全部可见 endpoint ID 列表（id 升序）
//
// scopeUserID==0（admin 视角）不过滤。
func (r *endpointRepository) FindIDsByScope(ctx context.Context, scopeUserID uint) ([]uint, error) {
	db := r.db.WithContext(ctx)
	q := db.Model(&dbmodel.Endpoint{}).Where(constant.DBConditionDeletedAtZero)
	if scopeUserID > 0 {
		q = q.Where(constant.FieldUserID, scopeUserID)
	}
	var ids []uint
	if err := q.Order(constant.FieldID).Pluck(constant.FieldID, &ids).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find endpoint ids by scope")
	}
	return ids, nil
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

// FindByAlias 按 alias 查询指定用户的所有关联模型记录（网关解析专用，userID 必传真实值）
func (r *modelRepository) FindByAlias(ctx context.Context, alias vo.EndpointAlias, userID uint) ([]*aggregate.Model, error) {
	db := r.db.WithContext(ctx)
	models, err := r.dao.BatchGet(db, &dbmodel.Model{Alias: alias.String(), Enabled: true, UserID: userID}, constant.ModelRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find models by alias")
	}
	return util.MapErr(models, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
}

func toModelAggregate(m *dbmodel.Model) (*aggregate.Model, error) {
	model, err := aggregate.CreateModel(m.ID, vo.EndpointAlias(m.Alias), m.UpstreamModel, m.EndpointID, m.Enabled, m.ContextLength, m.MaxOutputTokens, m.Capabilities)
	if err != nil {
		return nil, err
	}
	model.SetUserID(m.UserID)
	model.SetModelID(m.ModelID)
	model.SetTimestamps(m.CreatedAt, m.UpdatedAt)
	return model, nil
}

func toModelDBModel(m *aggregate.Model) *dbmodel.Model {
	return &dbmodel.Model{
		ID:              m.AggregateID(),
		Alias:           m.Alias().String(),
		ModelID:         m.ModelID(),
		UpstreamModel:   m.UpstreamModel(),
		EndpointID:      m.EndpointID(),
		Enabled:         m.Enabled(),
		ContextLength:   m.ContextLength(),
		MaxOutputTokens: m.MaxOutputTokens(),
		Capabilities:    m.Capabilities(),
	}
}

// FindByID 按 ID 查询模型（scopeUserID>0 时限定在该用户名下）
func (r *modelRepository) FindByID(ctx context.Context, id, scopeUserID uint) (*aggregate.Model, error) {
	db := r.db.WithContext(ctx)
	m, err := r.dao.Get(db, &dbmodel.Model{ID: id, UserID: scopeUserID}, constant.ModelRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find model by id")
	}
	return toModelAggregate(m)
}

// Create 创建模型（写入归属 ownerUserID）
func (r *modelRepository) Create(ctx context.Context, m *aggregate.Model, ownerUserID uint) (uint, error) {
	db := r.db.WithContext(ctx)
	mdl := toModelDBModel(m)
	mdl.UserID = ownerUserID
	if err := db.Create(mdl).Error; err != nil {
		return 0, ierr.Wrap(ierr.ErrDBCreate, err, "create model")
	}
	return mdl.ID, nil
}

// Update 更新模型（仅更新非零值字段）
func (r *modelRepository) Update(ctx context.Context, m *aggregate.Model) error {
	db := r.db.WithContext(ctx)
	// GORM 的 Updates(map) 不经过 field serializer，capabilities 需手动序列化为 JSON 字符串
	capJSON, _ := sonic.Marshal(m.Capabilities()) //nolint:errcheck // []string 序列化不会失败，且值已经聚合校验
	updates := map[string]any{
		constant.FieldModelAlias:           m.Alias().String(),
		constant.FieldModelID:              m.ModelID(),
		constant.FieldModelUpstreamModel:   m.UpstreamModel(),
		constant.FieldModelEndpointID:      m.EndpointID(),
		constant.FieldModelEnabled:         m.Enabled(),
		constant.FieldModelContextLength:   m.ContextLength(),
		constant.FieldModelMaxOutputTokens: m.MaxOutputTokens(),
		constant.FieldModelCapabilities:    string(capJSON),
	}
	if err := db.Model(&dbmodel.Model{}).Where(constant.WhereIDEquals, m.AggregateID()).Updates(updates).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update model")
	}
	return nil
}

// Delete 删除模型（软删除；scopeUserID>0 时限定在该用户名下）
func (r *modelRepository) Delete(ctx context.Context, id, scopeUserID uint) error {
	db := r.db.WithContext(ctx)
	if err := r.dao.Delete(db, &dbmodel.Model{ID: id, UserID: scopeUserID}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete model")
	}
	return nil
}

// DeleteByEndpointID 按 endpointID 批量删除模型
func (r *modelRepository) DeleteByEndpointID(ctx context.Context, endpointID uint) error {
	db := r.db.WithContext(ctx)
	if err := r.dao.BatchDeleteByField(db, constant.FieldEndpointID, []uint{endpointID}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete models by endpoint id")
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
	return util.MapErr(models, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
}

// Paginate 分页查询模型列表
//
//	@author centonhuang
//	@update 2026-05-27 10:00:00
func (r *modelRepository) Paginate(ctx context.Context, param model.CommonParam, scopeUserID uint) ([]*aggregate.Model, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Model{UserID: scopeUserID},
		constant.ModelRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldAlias, constant.FieldModelUpstreamModel}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate models")
	}
	out, convErr := util.MapErr(records, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
	if convErr != nil {
		return nil, nil, convErr
	}
	return out, pageInfo, nil
}

// PaginateWithFilter 带筛选的模型分页查询（Web 平铺模型列表专用）
//
//	@receiver r *modelRepository
//	@param ctx context.Context
//	@param param model.CommonParam
//	@param filter llmproxy.ModelListFilter
//	@param scopeUserID *uint nil 表示 admin 全量视角，非 nil 精确匹配 user_id
//	@return []*aggregate.Model
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-08-28 10:00:00
//
// capabilities 是 text 列 + serializer:json（非 PG 原生 jsonb），且 enum.InputModalities
// 是封闭枚举，故 LIKE '%"image"%' 在 PG 与 sqlite 上语法与行为一致，无需分库写法。
// 代价是不走索引；models 表千行量级可接受。
//
// 排序列走显式白名单：不能用 util.SafeSortField 代替（它只校验字符集，
// api_key 之类敏感列同样放行），白名单外取值回退默认列而非报错。
func (r *modelRepository) PaginateWithFilter(ctx context.Context, param model.CommonParam, filter llmproxy.ModelListFilter, scopeUserID *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	if scopeUserID != nil {
		db = db.Where(constant.FieldUserID, *scopeUserID)
	}
	switch filter.Status {
	case constant.ModelStatusEnabled:
		db = db.Where(constant.WhereModelEnabledEquals, true)
	case constant.ModelStatusDisabled:
		db = db.Where(constant.WhereModelEnabledEquals, false)
	}
	if filter.EndpointID != 0 {
		db = db.Where(constant.WhereEndpointIDEquals, filter.EndpointID)
	}
	// 未知能力值视为不过滤，避免前端拼错参数导致整页空白
	if lo.Contains(enum.InputModalities, filter.Capability) {
		db = db.Where(constant.WhereCapabilitiesLike, `%"`+filter.Capability+`"%`)
	}
	if !lo.Contains(constant.ModelListSortFields, param.SortField) {
		param.SortField = constant.ModelListDefaultSortField
		param.Sort = enum.SortDesc
	}

	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Model{},
		constant.ModelRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldAlias, constant.FieldModelID, constant.FieldModelUpstreamModel}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate models with filter")
	}
	out, convErr := util.MapErr(records, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
	if convErr != nil {
		return nil, nil, convErr
	}
	return out, pageInfo, nil
}

// ListByEndpointIDs 按 endpoint ID 集合批量拉取模型聚合（id 升序）
//
// 不做二次 scope 过滤，调用方传入的 endpointIDs 必须已经过 scope 解析。
func (r *modelRepository) ListByEndpointIDs(ctx context.Context, endpointIDs []uint) ([]*aggregate.Model, error) {
	ids := lo.Uniq(lo.Filter(endpointIDs, func(id uint, _ int) bool { return id != 0 }))
	if len(ids) == 0 {
		return []*aggregate.Model{}, nil
	}
	db := r.db.WithContext(ctx)
	records, err := r.dao.BatchGetByField(db, constant.FieldEndpointID, ids, constant.ModelRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list models by endpoint ids")
	}
	out, convErr := util.MapErr(records, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
	if convErr != nil {
		return nil, convErr
	}
	return out, nil
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

// ListAliases 查询指定用户的不重复模型别名（仅已启用的模型；userID=0 不过滤，仅限 admin 内部用途）
func (r *endpointReadRepository) ListAliases(ctx context.Context, userID uint) ([]*llmproxy.ModelAliasProjection, error) {
	db := r.db.WithContext(ctx)
	models, err := r.modelDAO.BatchGet(db, &dbmodel.Model{Enabled: true, UserID: userID}, constant.ModelRepoFieldsAlias)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list model aliases")
	}
	out := lo.Map(lo.UniqBy(models, func(m *dbmodel.Model) string { return m.Alias }), func(m *dbmodel.Model, _ int) *llmproxy.ModelAliasProjection {
		return &llmproxy.ModelAliasProjection{Alias: m.Alias}
	})
	return out, nil
}

// ListEnabledModelDetails 查询所有启用中模型的完整投影（仅已启用、按 alias 去重）
//
// userID 语义：网关路径必传真实用户 ID；0 不过滤（仅限 admin 内部用途）。
func (r *endpointReadRepository) ListEnabledModelDetails(ctx context.Context, userID uint) ([]*llmproxy.ModelDetailProjection, error) {
	db := r.db.WithContext(ctx)
	models, err := r.modelDAO.BatchGet(db, &dbmodel.Model{Enabled: true, UserID: userID}, constant.ModelRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list enabled model details")
	}
	uniq := lo.UniqBy(models, func(m *dbmodel.Model) string { return m.Alias })
	return lo.Map(uniq, func(m *dbmodel.Model, _ int) *llmproxy.ModelDetailProjection {
		return &llmproxy.ModelDetailProjection{
			Alias:           m.Alias,
			UpstreamModel:   m.UpstreamModel,
			ContextLength:   m.ContextLength,
			MaxOutputTokens: m.MaxOutputTokens,
			Capabilities:    m.Capabilities,
		}
	}), nil
}

// FindEndpointByAlias 按 alias 在指定用户的模型集合内随机选满足 matcher 的 endpoint。
// 仅查询已启用的模型。
func (r *endpointReadRepository) FindEndpointByAlias(ctx context.Context, userID uint, alias string, matcher func(*llmproxy.EndpointProjection) bool) (*llmproxy.EndpointProjection, *llmproxy.ModelAliasProjection, error) {
	db := r.db.WithContext(ctx)
	models, err := r.modelDAO.BatchGet(db, &dbmodel.Model{Alias: alias, Enabled: true, UserID: userID}, constant.ModelRepoFieldsFull)
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
			return proj, &llmproxy.ModelAliasProjection{Alias: m.UpstreamModel}, nil
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
