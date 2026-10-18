package repository

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/samber/lo"
	"github.com/samber/mo"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// endpointRepository EndpointRepository 的 GORM 实现
type endpointRepository struct {
	endpointDAO *dao.EndpointDAO
	modelDAO    *dao.ModelDAO
	db          *gorm.DB
	cache       *cache.ReadCache // nil = 不带读缓存失效
}

// NewEndpointRepository 构造 EndpointRepository
func NewEndpointRepository(db *gorm.DB) llmproxy.EndpointRepository {
	return &endpointRepository{endpointDAO: dao.GetEndpointDAO(), modelDAO: dao.GetModelDAO(), db: db}
}

// NewCachedEndpointRepository 构造带读缓存失效钩子的 EndpointRepository
//
// 写路径（Create/Update/Delete/DeleteCascade）成功后清空读缓存，
// 保证读缓存不残留脏数据。
func NewCachedEndpointRepository(db *gorm.DB, readCache *cache.ReadCache) llmproxy.EndpointRepository {
	return &endpointRepository{endpointDAO: dao.GetEndpointDAO(), modelDAO: dao.GetModelDAO(), db: db, cache: readCache}
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
	agg := toEndpointAggregate(ep)
	if err := r.fillOAuthStatus(ctx, []*aggregate.Endpoint{agg}); err != nil {
		return nil, err
	}
	return agg, nil
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
	aggs := make([]*aggregate.Endpoint, 0, len(records))
	for _, rec := range records {
		aggs = append(aggs, toEndpointAggregate(rec))
	}
	if err := r.fillOAuthStatus(ctx, aggs); err != nil {
		return nil, err
	}
	out = make(map[uint]*aggregate.Endpoint, len(aggs))
	for _, agg := range aggs {
		out[agg.AggregateID()] = agg
	}
	return out, nil
}

// fillOAuthStatus 为 OAuth 类型端点批量填充凭据状态投影（一次 IN 查询；无 OAuth 端点不打 SQL）。
//
// 缺凭据行的 OAuth 端点保守标记为 reauth_required（配置不完整不可用，fail-closed）——
// 只跳过填充会让解析层把零值当"非 OAuth"放行，失效端点仍会被选中。
func (r *endpointRepository) fillOAuthStatus(ctx context.Context, eps []*aggregate.Endpoint) error {
	oauthIDs := make([]uint, 0, len(eps))
	for _, ep := range eps {
		if ep.AuthType().IsOAuth() {
			oauthIDs = append(oauthIDs, ep.AggregateID())
		}
	}
	if len(oauthIDs) == 0 {
		return nil
	}
	var records []*dbmodel.EndpointOAuthCredential
	if err := r.db.WithContext(ctx).
		Where(constant.WhereEndpointIDIn, oauthIDs).
		Where(constant.DBConditionDeletedAtZero).
		Find(&records).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBQuery, err, "fill oauth credential status")
	}
	statusByEndpoint := make(map[uint]enum.OAuthCredentialStatus, len(records))
	for _, record := range records {
		statusByEndpoint[record.EndpointID] = enum.OAuthCredentialStatus(record.Status)
	}
	for _, ep := range eps {
		if !ep.AuthType().IsOAuth() {
			continue
		}
		status, ok := statusByEndpoint[ep.AggregateID()]
		if !ok {
			status = enum.OAuthCredentialStatusReauthRequired
		}
		ep.SetOAuthStatus(status)
	}
	return nil
}

// toEndpointAggregate 从 DB 记录恢复聚合根。
//
// 走不校验的 RehydrateEndpoint：恢复失败的代价是整表读取失败（upstream 列表、
// 代理路由、PATCH 修复入口全挂），而校验型工厂只该守在写边界上。
// 见 aggregate.RehydrateEndpoint 的注释。
func toEndpointAggregate(m *dbmodel.Endpoint) *aggregate.Endpoint {
	ep := aggregate.RehydrateEndpoint(
		m.ID,
		m.Name,
		m.OpenaiBaseURL,
		m.AnthropicBaseURL,
		m.APIKey,
		m.SupportOpenAIChatCompletion,
		m.SupportOpenAIResponse,
		m.SupportAnthropicMessage,
		m.SupportOpenAIDecision,
		enum.EndpointAuthType(m.AuthType),
	)
	ep.SetUserID(m.UserID)
	ep.SetTimestamps(m.CreatedAt, m.UpdatedAt)
	return ep
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
		SupportOpenAIDecision:       ep.SupportOpenAIDecision(),
		AuthType:                    string(ep.AuthType()),
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
	r.cache.InvalidateAll(ctx)
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
		constant.FieldEndpointSupportOpenAIDecision:       ep.SupportOpenAIDecision(),
	}
	if err := db.Model(&dbmodel.Endpoint{}).Where(constant.WhereIDEquals, ep.AggregateID()).Updates(updates).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update endpoint")
	}
	r.cache.InvalidateAll(ctx)
	return nil
}

// Delete 删除端点（软删除；scopeUserID 非 nil 时精确匹配 user_id）
func (r *endpointRepository) Delete(ctx context.Context, id uint, scopeUserID *uint) error {
	db := scopedDB(r.db.WithContext(ctx), scopeUserID)
	if err := r.endpointDAO.Delete(db, &dbmodel.Endpoint{ID: id}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete endpoint")
	}
	r.cache.InvalidateAll(ctx)
	return nil
}

// DeleteCascade 级联删除端点及其关联模型（事务保护；scopeUserID 非 nil 时精确匹配 user_id）
func (r *endpointRepository) DeleteCascade(ctx context.Context, id uint, scopeUserID *uint) error {
	db := r.db.WithContext(ctx)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := r.modelDAO.BatchDeleteByField(tx, constant.FieldEndpointID, []uint{id}); err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "cascade delete models by endpoint id")
		}
		if err := r.endpointDAO.Delete(scopedDB(tx, scopeUserID), &dbmodel.Endpoint{ID: id}); err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "delete endpoint")
		}
		return nil
	}); err != nil {
		return err
	}
	r.cache.InvalidateAll(ctx)
	return nil
}

// List 列出所有端点
func (r *endpointRepository) List(ctx context.Context) ([]*aggregate.Endpoint, error) {
	db := r.db.WithContext(ctx)
	var models []*dbmodel.Endpoint
	if err := db.Find(&models).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list endpoints")
	}
	return lo.Map(models, func(m *dbmodel.Endpoint, _ int) *aggregate.Endpoint {
		return toEndpointAggregate(m)
	}), nil
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
	out := lo.Map(records, func(m *dbmodel.Endpoint, _ int) *aggregate.Endpoint {
		return toEndpointAggregate(m)
	})
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
	cache       *cache.ReadCache // nil = 不带读缓存
}

// NewEndpointReadRepository 构造 EndpointReadRepository
func NewEndpointReadRepository(db *gorm.DB) llmproxy.EndpointReadRepository {
	return &endpointReadRepository{
		endpointDAO: dao.GetEndpointDAO(),
		modelDAO:    dao.GetModelDAO(),
		db:          db,
	}
}

// NewCachedEndpointReadRepository 构造带 Redis 读缓存的 EndpointReadRepository
//
// 缓存只覆盖无凭据载荷（别名列表、模型详情列表）；FindEndpointByAlias 的投影
// 含上游 API Key，按项目约定不写入 Redis。
func NewCachedEndpointReadRepository(db *gorm.DB, readCache *cache.ReadCache) llmproxy.EndpointReadRepository {
	return &endpointReadRepository{
		endpointDAO: dao.GetEndpointDAO(),
		modelDAO:    dao.GetModelDAO(),
		db:          db,
		cache:       readCache,
	}
}

// ListAliases 查询指定用户的不重复模型别名（仅已启用的模型）
//
// userID 必传真实用户 ID；0（认证缺失）防御性返回空列表而非全平台别名——
// struct 零值条件下 GORM 会忽略 user_id 过滤，守卫必须有。
//
// 结果走 Redis 读缓存（cache:rd:alias:<userID>）：空结果写空值标记防穿透，
// TTL 叠加随机抖动防雪崩，同 key 并发回源由 singleflight 合并；
// 失效由 model/endpoint 写路径的 InvalidateAll 承担。
func (r *endpointReadRepository) ListAliases(ctx context.Context, userID uint) ([]*llmproxy.ModelAliasProjection, error) {
	if userID == 0 {
		return []*llmproxy.ModelAliasProjection{}, nil
	}
	key := fmt.Sprintf(constant.ReadCacheAliasKeyTemplate, userID)
	cached, err := cache.GetOrLoad(ctx, r.cache, key, func(ctx context.Context) (mo.Option[[]*llmproxy.ModelAliasProjection], error) {
		return r.loadAliases(ctx, userID)
	})
	if err != nil {
		return nil, err
	}
	return cached.OrEmpty(), nil
}

// loadAliases 回源查询模型别名；空结果返回 mo.None（由缓存层写空值标记防穿透）
func (r *endpointReadRepository) loadAliases(ctx context.Context, userID uint) (mo.Option[[]*llmproxy.ModelAliasProjection], error) {
	db := r.db.WithContext(ctx)
	models, err := r.modelDAO.BatchGet(db, &dbmodel.Model{Enabled: true, UserID: userID}, constant.ModelRepoFieldsAlias)
	if err != nil {
		return mo.None[[]*llmproxy.ModelAliasProjection](), ierr.Wrap(ierr.ErrDBQuery, err, "list model aliases")
	}
	out := lo.Map(lo.UniqBy(models, func(m *dbmodel.Model) string { return m.Alias }), func(m *dbmodel.Model, _ int) *llmproxy.ModelAliasProjection {
		return &llmproxy.ModelAliasProjection{Alias: m.Alias}
	})
	if len(out) == 0 {
		return mo.None[[]*llmproxy.ModelAliasProjection](), nil
	}
	return mo.Some(out), nil
}

// ListEnabledModelDetails 查询所有启用中模型的完整投影（仅已启用、按 alias 去重）
//
// userID 必传真实用户 ID；0（认证缺失）防御性返回空列表，语义同 ListAliases。
// 缓存语义同 ListAliases（cache:rd:detail:<userID>）。
func (r *endpointReadRepository) ListEnabledModelDetails(ctx context.Context, userID uint) ([]*llmproxy.ModelDetailProjection, error) {
	if userID == 0 {
		return []*llmproxy.ModelDetailProjection{}, nil
	}
	key := fmt.Sprintf(constant.ReadCacheDetailKeyTemplate, userID)
	cached, err := cache.GetOrLoad(ctx, r.cache, key, func(ctx context.Context) (mo.Option[[]*llmproxy.ModelDetailProjection], error) {
		return r.loadModelDetails(ctx, userID)
	})
	if err != nil {
		return nil, err
	}
	return cached.OrEmpty(), nil
}

// loadModelDetails 回源查询启用中模型详情；空结果返回 mo.None（写空值标记防穿透）
func (r *endpointReadRepository) loadModelDetails(ctx context.Context, userID uint) (mo.Option[[]*llmproxy.ModelDetailProjection], error) {
	db := r.db.WithContext(ctx)
	models, err := r.modelDAO.BatchGet(db, &dbmodel.Model{Enabled: true, UserID: userID}, constant.ModelRepoFieldsFull)
	if err != nil {
		return mo.None[[]*llmproxy.ModelDetailProjection](), ierr.Wrap(ierr.ErrDBQuery, err, "list enabled model details")
	}
	uniq := lo.UniqBy(models, func(m *dbmodel.Model) string { return m.Alias })
	out := lo.Map(uniq, func(m *dbmodel.Model, _ int) *llmproxy.ModelDetailProjection {
		return &llmproxy.ModelDetailProjection{
			Alias:           m.Alias,
			UpstreamModel:   m.UpstreamModel,
			ContextLength:   m.ContextLength,
			MaxOutputTokens: m.MaxOutputTokens,
			Capabilities:    m.Capabilities,
			Pricing:         pricingFromDB(m.ID, m.PricingRules, m.PricingCurrency),
		}
	})
	if len(out) == 0 {
		return mo.None[[]*llmproxy.ModelDetailProjection](), nil
	}
	return mo.Some(out), nil
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

// CreateWithOAuthCredential 在单事务内创建 OAuth 端点与其凭据（llmproxy.OAuthEndpointRepository）。
//
// 端点与凭据必须原子落库：分两次写会留下「OAuth 端点无凭据行」的中间态，
// 该状态下解析层按 reauth_required 剔除、用户端表现为端点不可用。
func (r *endpointRepository) CreateWithOAuthCredential(ctx context.Context, ep *aggregate.Endpoint, cred *aggregate.OAuthCredential, ownerUserID uint) (uint, error) {
	db := r.db.WithContext(ctx)
	var endpointID uint
	if err := db.Transaction(func(tx *gorm.DB) error {
		record := toEndpointModel(ep)
		record.UserID = ownerUserID
		if err := tx.Create(record).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBCreate, err, "create oauth endpoint")
		}
		endpointID = record.ID

		cred.SetEndpointID(endpointID)
		credRecord, err := buildOAuthCredentialModel(cred)
		if err != nil {
			return err
		}
		if err := tx.Create(credRecord).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBCreate, err, "create oauth credential")
		}
		return nil
	}); err != nil {
		return 0, err
	}
	r.cache.InvalidateAll(ctx)
	return endpointID, nil
}

var _ llmproxy.OAuthEndpointRepository = (*endpointRepository)(nil)

// NewOAuthEndpointRepository 构造订阅端点原子创建仓储。
//
// 与 NewEndpointRepository 是同一实现的独立接口视图（fx 按接口类型分别注入）。
func NewOAuthEndpointRepository(db *gorm.DB) llmproxy.OAuthEndpointRepository {
	return &endpointRepository{endpointDAO: dao.GetEndpointDAO(), modelDAO: dao.GetModelDAO(), db: db}
}
