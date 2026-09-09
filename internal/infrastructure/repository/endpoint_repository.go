package repository

import (
	"context"
	"errors"
	"math/rand"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
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

// scopedDB scope 非 nil 时追加 user_id 显式等值条件（含 0=共享池）。
//
// 不能用 struct 条件承载：GORM 会把零值字段（含显式赋值的 0）从 Where 条件中丢弃；
// 也不能写 Where("user_id", v)：无占位符时 v 被静默丢弃。
func scopedDB(db *gorm.DB, scope *uint) *gorm.DB {
	if scope != nil {
		return db.Where(constant.WhereUserIDEquals, *scope)
	}
	return db
}

// FindByID 按 ID 查询端点（scopeUserID 非 nil 时精确匹配 user_id，含共享池 0）
func (r *endpointRepository) FindByID(ctx context.Context, id uint, scopeUserID *uint) (*aggregate.Endpoint, error) {
	db := scopedDB(r.db.WithContext(ctx), scopeUserID)
	ep, err := r.endpointDAO.Get(db, &dbmodel.Endpoint{ID: id}, constant.EndpointRepoFieldsFull)
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

// Delete 删除端点（软删除；scopeUserID 非 nil 时精确匹配 user_id）
func (r *endpointRepository) Delete(ctx context.Context, id uint, scopeUserID *uint) error {
	db := scopedDB(r.db.WithContext(ctx), scopeUserID)
	if err := r.endpointDAO.Delete(db, &dbmodel.Endpoint{ID: id}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete endpoint")
	}
	return nil
}

// DeleteCascade 级联删除端点及其关联模型（事务保护；scopeUserID 非 nil 时精确匹配 user_id）
func (r *endpointRepository) DeleteCascade(ctx context.Context, id uint, scopeUserID *uint) error {
	db := r.db.WithContext(ctx)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := r.modelDAO.BatchDeleteByField(tx, constant.FieldEndpointID, []uint{id}); err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "cascade delete models by endpoint id")
		}
		if err := r.endpointDAO.Delete(scopedDB(tx, scopeUserID), &dbmodel.Endpoint{ID: id}); err != nil {
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
func (r *endpointRepository) Paginate(ctx context.Context, param model.CommonParam, scopeUserID *uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	db := scopedDB(r.db.WithContext(ctx), scopeUserID)
	records, pageInfo, err := r.endpointDAO.Paginate(
		db,
		&dbmodel.Endpoint{},
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
// scopeUserID 为 nil（admin 视角）不过滤；非 nil 精确匹配 user_id（含 0=共享池）。
func (r *endpointRepository) FindIDsByScope(ctx context.Context, scopeUserID *uint) ([]uint, error) {
	db := r.db.WithContext(ctx)
	q := db.Model(&dbmodel.Endpoint{}).Where(constant.DBConditionDeletedAtZero)
	if scopeUserID != nil {
		q = q.Where(constant.WhereUserIDEquals, *scopeUserID)
	}
	var ids []uint
	if err := q.Order(constant.FieldID).Pluck(constant.FieldID, &ids).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find endpoint ids by scope")
	}
	return ids, nil
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

// ListAliases 查询指定用户的不重复模型别名（仅已启用的模型）
//
// userID 必传真实用户 ID；0（认证缺失）防御性返回空列表而非全平台别名——
// struct 零值条件下 GORM 会忽略 user_id 过滤，守卫必须有。
func (r *endpointReadRepository) ListAliases(ctx context.Context, userID uint) ([]*llmproxy.ModelAliasProjection, error) {
	if userID == 0 {
		return []*llmproxy.ModelAliasProjection{}, nil
	}
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
// userID 必传真实用户 ID；0（认证缺失）防御性返回空列表，语义同 ListAliases。
func (r *endpointReadRepository) ListEnabledModelDetails(ctx context.Context, userID uint) ([]*llmproxy.ModelDetailProjection, error) {
	if userID == 0 {
		return []*llmproxy.ModelDetailProjection{}, nil
	}
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
// 仅查询已启用的模型；userID=0（认证缺失）防御性返回空。
func (r *endpointReadRepository) FindEndpointByAlias(ctx context.Context, userID uint, alias string, matcher func(*llmproxy.EndpointProjection) bool) (*llmproxy.EndpointProjection, *llmproxy.ModelAliasProjection, error) {
	if userID == 0 {
		return nil, nil, nil
	}
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
