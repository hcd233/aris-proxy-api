package query

import (
	"context"

	"go.uber.org/zap"

	modelport "github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ListModelsHandler 查询处理器
type ListModelsHandler interface {
	Handle(ctx context.Context, q modelport.ListModelsQuery) ([]*modelport.ModelView, *model.PageInfo, error)
}

type listModelsHandler struct {
	repo         llmproxy.ModelRepository
	endpointRepo llmproxy.EndpointRepository
}

// NewListModelsHandler 构造查询处理器
func NewListModelsHandler(repo llmproxy.ModelRepository, endpointRepo llmproxy.EndpointRepository) ListModelsHandler {
	return &listModelsHandler{repo: repo, endpointRepo: endpointRepo}
}

// Handle 执行列表查询
func (h *listModelsHandler) Handle(ctx context.Context, q modelport.ListModelsQuery) ([]*modelport.ModelView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	models, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam)
	if err != nil {
		log.Error("[ModelQuery] List models failed", zap.Error(err))
		return nil, nil, err
	}

	endpointsByID, err := h.loadEndpoints(ctx, models)
	if err != nil {
		log.Error("[ModelQuery] Load endpoints failed", zap.Error(err))
		return nil, nil, err
	}

	views := make([]*modelport.ModelView, 0, len(models))
	for _, m := range models {
		views = append(views, &modelport.ModelView{
			ID:        m.AggregateID(),
			Alias:     m.Alias().String(),
			ModelName: m.ModelName(),
			Endpoint:  toEndpointView(endpointsByID[m.EndpointID()]),
			CreatedAt: m.CreatedAt(),
			UpdatedAt: m.UpdatedAt(),
		})
	}

	log.Info("[ModelQuery] List models", zap.Int("count", len(views)))
	return views, pageInfo, nil
}

// loadEndpoints 一次性拉取本页所有 model 关联的 endpoint，避免 N+1。
func (h *listModelsHandler) loadEndpoints(ctx context.Context, models []*aggregate.Model) (map[uint]*aggregate.Endpoint, error) {
	seen := make(map[uint]struct{}, len(models))
	ids := make([]uint, 0, len(models))
	for _, m := range models {
		if _, ok := seen[m.EndpointID()]; ok {
			continue
		}
		seen[m.EndpointID()] = struct{}{}
		ids = append(ids, m.EndpointID())
	}
	return h.endpointRepo.BatchFindByIDs(ctx, ids)
}

func toEndpointView(ep *aggregate.Endpoint) *modelport.EndpointView {
	if ep == nil {
		return nil
	}
	return &modelport.EndpointView{
		ID:                          ep.AggregateID(),
		Name:                        ep.Name(),
		OpenaiBaseURL:               ep.OpenaiBaseURL(),
		AnthropicBaseURL:            ep.AnthropicBaseURL(),
		MaskedAPIKey:                commonutil.MaskSecret(ep.APIKey()),
		SupportOpenAIChatCompletion: ep.SupportOpenAIChatCompletion(),
		SupportOpenAIResponse:       ep.SupportOpenAIResponse(),
		SupportAnthropicMessage:     ep.SupportAnthropicMessage(),
		CreatedAt:                   ep.CreatedAt(),
		UpdatedAt:                   ep.UpdatedAt(),
	}
}
