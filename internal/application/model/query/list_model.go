package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityaggregate "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type listModelHandler struct {
	modelRepo    llmproxy.ModelRepository
	endpointRepo llmproxy.EndpointRepository
	userRepo     identity.UserRepository
}

// NewListModelHandler 构造平铺模型列表查询处理器
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
func NewListModelHandler(modelRepo llmproxy.ModelRepository, endpointRepo llmproxy.EndpointRepository, userRepo identity.UserRepository) port.ListModelHandler {
	return &listModelHandler{modelRepo: modelRepo, endpointRepo: endpointRepo, userRepo: userRepo}
}

// Handle 执行平铺模型列表查询：SQL 侧分页/筛选/排序，端点与用户批量回填。
func (h *listModelHandler) Handle(ctx context.Context, q port.ListModelQuery) ([]*port.ListModelView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	scope := q.ScopeUserID
	if scope == nil && q.Username != "" {
		u, err := h.userRepo.FindByName(ctx, q.Username)
		if err != nil {
			log.Error("[ModelQuery] Find user by name failed", zap.Error(err))
			return nil, nil, err
		}
		if u == nil {
			// 用户不存在 → 空结果而非错误；绝不可让 scope 保持 nil 退化为全量可见
			return []*port.ListModelView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize}, nil
		}
		scope = lo.ToPtr(u.AggregateID())
	}

	filter := llmproxy.ModelListFilter{Status: q.Status, EndpointID: q.EndpointID, Capability: q.Capability}
	models, pageInfo, err := h.modelRepo.PaginateWithFilter(ctx, q.CommonParam, filter, scope)
	if err != nil {
		log.Error("[ModelQuery] Paginate models failed", zap.Error(err))
		return nil, nil, err
	}

	epsByID := h.loadEndpoints(ctx, models)
	usersByID := h.loadUsers(ctx, models)

	views := lo.Map(models, func(m *llmagg.Model, _ int) *port.ListModelView {
		return toListModelView(m, epsByID, usersByID, q.IsDemo)
	})
	log.Info("[ModelQuery] List models", zap.Int("rows", len(views)), zap.Int64("total", pageInfo.Total))
	return views, pageInfo, nil
}

// loadEndpoints 批量拉取本页模型所属端点，避免 N+1
func (h *listModelHandler) loadEndpoints(ctx context.Context, models []*llmagg.Model) map[uint]*llmagg.Endpoint {
	ids := uniqNonZero(lo.Map(models, func(m *llmagg.Model, _ int) uint { return m.EndpointID() }))
	out, err := h.endpointRepo.BatchFindByIDs(ctx, ids)
	if err != nil {
		logger.WithCtx(ctx).Warn("[ModelQuery] Load endpoints failed", zap.Error(err))
		return map[uint]*llmagg.Endpoint{}
	}
	return out
}

// loadUsers 批量拉取归属用户；userID==0（共享池/遗留）不查，避免拼出错误归属
func (h *listModelHandler) loadUsers(ctx context.Context, models []*llmagg.Model) map[uint]*identityaggregate.User {
	ids := uniqNonZero(lo.Map(models, func(m *llmagg.Model, _ int) uint { return m.UserID() }))
	out, err := h.userRepo.BatchFindByIDs(ctx, ids)
	if err != nil {
		logger.WithCtx(ctx).Warn("[ModelQuery] Load users failed", zap.Error(err))
		return map[uint]*identityaggregate.User{}
	}
	return out
}

// uniqNonZero 去重并剔除 0（未归属），结果作为批量查询入参
func uniqNonZero(ids []uint) []uint {
	return lo.Uniq(lo.Filter(ids, func(id uint, _ int) bool { return id != 0 }))
}

func toListModelView(m *llmagg.Model, epsByID map[uint]*llmagg.Endpoint, usersByID map[uint]*identityaggregate.User, isDemo bool) *port.ListModelView {
	upstreamModel := m.UpstreamModel()
	if isDemo {
		upstreamModel = commonutil.MaskSecret(upstreamModel)
	}
	v := &port.ListModelView{
		ID:              m.AggregateID(),
		Alias:           m.Alias().String(),
		ModelID:         m.ModelID(),
		UpstreamModel:   upstreamModel,
		Enabled:         m.Enabled(),
		ContextLength:   m.ContextLength(),
		MaxOutputTokens: m.MaxOutputTokens(),
		Capabilities:    m.Capabilities(),
		CreatedAt:       m.CreatedAt(),
		UpdatedAt:       m.UpdatedAt(),
	}
	if ep := epsByID[m.EndpointID()]; ep != nil {
		v.Endpoint = &port.ListModelEndpointView{ID: ep.AggregateID(), Name: ep.Name()}
	}
	if u := usersByID[m.UserID()]; u != nil {
		v.User = &port.ListModelUserView{ID: u.AggregateID(), Name: string(u.Name()), Avatar: string(u.Avatar())}
	}
	return v
}
