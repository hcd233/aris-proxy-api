package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	modelport "github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type listModelsHandler struct {
	repo         llmproxy.ModelRepository
	endpointRepo llmproxy.EndpointRepository
	userRepo     identity.UserRepository
}

// NewListModelsHandler 构造查询处理器
func NewListModelsHandler(repo llmproxy.ModelRepository, endpointRepo llmproxy.EndpointRepository, userRepo identity.UserRepository) modelport.ListModelsHandler {
	return &listModelsHandler{repo: repo, endpointRepo: endpointRepo, userRepo: userRepo}
}

// Handle 执行列表查询
func (h *listModelsHandler) Handle(ctx context.Context, q modelport.ListModelsQuery) ([]*modelport.ModelView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	scope := q.ScopeUserID
	if scope == 0 && q.Username != "" {
		u, findErr := h.userRepo.FindByName(ctx, q.Username)
		if findErr != nil {
			log.Error("[ModelQuery] Find user by name failed", zap.Error(findErr))
			return nil, nil, findErr
		}
		if u == nil {
			return []*modelport.ModelView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize}, nil
		}
		scope = u.AggregateID()
	}

	models, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam, scope)
	if err != nil {
		log.Error("[ModelQuery] List models failed", zap.Error(err))
		return nil, nil, err
	}

	endpointsByID, err := h.loadEndpoints(ctx, models)
	if err != nil {
		log.Error("[ModelQuery] Load endpoints failed", zap.Error(err))
		return nil, nil, err
	}

	usernamesByID := h.loadUsernames(ctx, models)

	views := lo.Map(models, func(m *aggregate.Model, _ int) *modelport.ModelView {
		upstreamModel := m.UpstreamModel()
		if q.IsDemo {
			upstreamModel = commonutil.MaskSecret(upstreamModel)
		}
		return &modelport.ModelView{
			ID:              m.AggregateID(),
			Username:        usernamesByID[m.UserID()],
			Alias:           m.Alias().String(),
			ModelID:         m.ModelID(),
			UpstreamModel:   upstreamModel,
			Enabled:         m.Enabled(),
			ContextLength:   m.ContextLength(),
			MaxOutputTokens: m.MaxOutputTokens(),
			Capabilities:    m.Capabilities(),
			Endpoint:        toEndpointView(endpointsByID[m.EndpointID()], q.IsDemo),
			CreatedAt:       m.CreatedAt(),
			UpdatedAt:       m.UpdatedAt(),
		}
	})

	log.Info("[ModelQuery] List models", zap.Int("count", len(views)))
	return views, pageInfo, nil
}

// loadUsernames 一次性拉取本页 model 归属用户名，避免 N+1。
func (h *listModelsHandler) loadUsernames(ctx context.Context, models []*aggregate.Model) map[uint]string {
	out := make(map[uint]string, len(models))
	ids := lo.Uniq(lo.Map(models, func(m *aggregate.Model, _ int) uint { return m.UserID() }))
	if len(ids) == 0 {
		return out
	}
	users, err := h.userRepo.BatchFindByIDs(ctx, ids)
	if err != nil {
		logger.WithCtx(ctx).Warn("[ModelQuery] Load usernames failed", zap.Error(err))
		return out
	}
	for id, u := range users {
		out[id] = string(u.Name())
	}
	return out
}

// loadEndpoints 一次性拉取本页所有 model 关联的 endpoint，避免 N+1。
func (h *listModelsHandler) loadEndpoints(ctx context.Context, models []*aggregate.Model) (map[uint]*aggregate.Endpoint, error) {
	ids := lo.Uniq(lo.Map(models, func(m *aggregate.Model, _ int) uint { return m.EndpointID() }))
	return h.endpointRepo.BatchFindByIDs(ctx, ids)
}

func toEndpointView(ep *aggregate.Endpoint, isDemo bool) *modelport.EndpointView {
	if ep == nil {
		return nil
	}
	name := ep.Name()
	openaiBaseURL := ep.OpenaiBaseURL()
	anthropicBaseURL := ep.AnthropicBaseURL()
	if isDemo {
		name = commonutil.MaskSecret(name)
		openaiBaseURL = commonutil.MaskSecret(openaiBaseURL)
		anthropicBaseURL = commonutil.MaskSecret(anthropicBaseURL)
	}
	return &modelport.EndpointView{
		ID:                          ep.AggregateID(),
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
