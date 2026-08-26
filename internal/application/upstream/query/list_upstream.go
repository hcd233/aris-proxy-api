package query

import (
	"context"
	"strings"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/upstream/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityaggregate "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type listUpstreamHandler struct {
	endpointRepo llmproxy.EndpointRepository
	modelRepo    llmproxy.ModelRepository
	userRepo     identity.UserRepository
}

// NewListUpstreamHandler 构造 upstream 分组列表查询处理器
func NewListUpstreamHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository, userRepo identity.UserRepository) port.ListUpstreamHandler {
	return &listUpstreamHandler{endpointRepo: endpointRepo, modelRepo: modelRepo, userRepo: userRepo}
}

// Handle 执行分组列表查询：endpoint 组分页，组内模型随组全量返回。
func (h *listUpstreamHandler) Handle(ctx context.Context, q port.ListUpstreamQuery) ([]*port.UpstreamGroupView, int64, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	scope := q.ScopeUserID
	if scope == 0 && q.Username != "" {
		u, err := h.userRepo.FindByName(ctx, q.Username)
		if err != nil {
			log.Error("[UpstreamQuery] Find user by name failed", zap.Error(err))
			return nil, 0, nil, err
		}
		if u == nil {
			// 用户不存在 → 空结果而非错误
			return []*port.UpstreamGroupView{}, 0,
				&model.PageInfo{Page: q.Page, PageSize: q.PageSize}, nil
		}
		scope = u.AggregateID()
	}

	allIDs, err := h.endpointRepo.FindIDsByScope(ctx, scope)
	if err != nil {
		log.Error("[UpstreamQuery] Find endpoint ids failed", zap.Error(err))
		return nil, 0, nil, err
	}

	epsByID, err := h.endpointRepo.BatchFindByIDs(ctx, allIDs)
	if err != nil {
		log.Error("[UpstreamQuery] Load endpoints failed", zap.Error(err))
		return nil, 0, nil, err
	}

	models, err := h.modelRepo.ListByEndpointIDs(ctx, allIDs)
	if err != nil {
		log.Error("[UpstreamQuery] Load models failed", zap.Error(err))
		return nil, 0, nil, err
	}

	modelsByEp := make(map[uint][]*llmagg.Model, len(allIDs))
	for _, m := range models {
		modelsByEp[m.EndpointID()] = append(modelsByEp[m.EndpointID()], m)
	}

	matchedIDs := allIDs
	if q.Query != "" {
		kw := strings.ToLower(q.Query)
		// keyword 命中 endpoint 名称或其下任一模型字段 → 整组保留
		matchedIDs = lo.Filter(allIDs, func(id uint, _ int) bool {
			if ep := epsByID[id]; ep != nil && strings.Contains(strings.ToLower(ep.Name()), kw) {
				return true
			}
			return lo.SomeBy(modelsByEp[id], func(m *llmagg.Model) bool {
				return strings.Contains(strings.ToLower(m.Alias().String()), kw) ||
					strings.Contains(strings.ToLower(m.ModelID()), kw) ||
					strings.Contains(strings.ToLower(m.UpstreamModel()), kw)
			})
		})
	}

	start := (q.Page - 1) * q.PageSize
	start = min(start, len(matchedIDs))
	end := min(start+q.PageSize, len(matchedIDs))
	pageIDs := matchedIDs[start:end]

	usersByID := h.loadUsers(ctx, epsByID, modelsByEp)

	groups := make([]*port.UpstreamGroupView, 0, len(pageIDs))
	var modelTotal int64
	for _, id := range pageIDs {
		groups = append(groups, h.toGroupView(epsByID[id], modelsByEp[id], usersByID, q.IsDemo))
	}
	// modelTotal 口径跟随当前筛选范围的全量模型数（含非当前页组），不受组内截断影响。
	for _, id := range matchedIDs {
		modelTotal += int64(len(modelsByEp[id]))
	}

	pageInfo := &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: int64(len(matchedIDs))}
	log.Info("[UpstreamQuery] List upstream",
		zap.Int("groups", len(groups)), zap.Int64("modelTotal", modelTotal), zap.Int64("total", pageInfo.Total))
	return groups, modelTotal, pageInfo, nil
}

// loadUsers 一次性拉取本页 endpoint 与 model 归属用户，避免 N+1。
func (h *listUpstreamHandler) loadUsers(ctx context.Context, epsByID map[uint]*llmagg.Endpoint, modelsByEp map[uint][]*llmagg.Model) map[uint]*identityaggregate.User {
	ids := lo.Map(lo.Values(epsByID), func(ep *llmagg.Endpoint, _ int) uint { return ep.UserID() })
	for _, ms := range modelsByEp {
		for _, m := range ms {
			ids = append(ids, m.UserID())
		}
	}
	ids = lo.Uniq(ids)

	out := make(map[uint]*identityaggregate.User, len(ids))
	users, err := h.userRepo.BatchFindByIDs(ctx, ids)
	if err != nil {
		logger.WithCtx(ctx).Warn("[UpstreamQuery] Load users failed", zap.Error(err))
		return out
	}
	return users
}

func (h *listUpstreamHandler) toGroupView(ep *llmagg.Endpoint, models []*llmagg.Model, usersByID map[uint]*identityaggregate.User, isDemo bool) *port.UpstreamGroupView {
	mvs := lo.Map(models, func(m *llmagg.Model, _ int) *port.UpstreamModelView {
		return toModelView(m, usersByID, isDemo)
	})
	truncated := false
	if len(mvs) > constant.UpstreamGroupModelLimit {
		mvs = mvs[:constant.UpstreamGroupModelLimit]
		truncated = true
	}
	return &port.UpstreamGroupView{
		Endpoint:   toEndpointView(ep, usersByID, isDemo),
		Models:     mvs,
		ModelCount: len(mvs),
		Truncated:  truncated,
	}
}

func toEndpointView(ep *llmagg.Endpoint, usersByID map[uint]*identityaggregate.User, isDemo bool) *port.UpstreamEndpointView {
	if ep == nil {
		return nil
	}
	name := ep.Name()
	openaiBaseURL := ep.OpenaiBaseURL()
	anthropicBaseURL := ep.AnthropicBaseURL()
	if isDemo {
		openaiBaseURL = commonutil.MaskSecret(openaiBaseURL)
		anthropicBaseURL = commonutil.MaskSecret(anthropicBaseURL)
	}
	return &port.UpstreamEndpointView{
		ID:                          ep.AggregateID(),
		User:                        toUserView(usersByID[ep.UserID()]),
		Name:                        name,
		OpenaiBaseURL:               openaiBaseURL,
		AnthropicBaseURL:            anthropicBaseURL,
		MaskedAPIKey:                commonutil.MaskSecret(ep.APIKey()),
		SupportOpenAIChatCompletion: ep.SupportOpenAIChatCompletion(),
		SupportOpenAIResponse:       ep.SupportOpenAIResponse(),
		SupportAnthropicMessage:     ep.SupportAnthropicMessage(),
		CreatedAt:                   ep.CreatedAt(),
		UpdatedAt:                   ep.UpdatedAt(),
	}
}

func toModelView(m *llmagg.Model, usersByID map[uint]*identityaggregate.User, isDemo bool) *port.UpstreamModelView {
	upstreamModel := m.UpstreamModel()
	if isDemo {
		upstreamModel = commonutil.MaskSecret(upstreamModel)
	}
	return &port.UpstreamModelView{
		ID:              m.AggregateID(),
		User:            toUserView(usersByID[m.UserID()]),
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
}

func toUserView(u *identityaggregate.User) *port.UpstreamUserView {
	if u == nil {
		return nil
	}
	return &port.UpstreamUserView{
		ID:     u.AggregateID(),
		Name:   string(u.Name()),
		Avatar: string(u.Avatar()),
	}
}
