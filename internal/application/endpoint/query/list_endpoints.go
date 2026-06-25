package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type listEndpointsHandler struct {
	repo llmproxy.EndpointRepository
}

// NewListEndpointsHandler 构造查询处理器
func NewListEndpointsHandler(repo llmproxy.EndpointRepository) port.ListEndpointsHandler {
	return &listEndpointsHandler{repo: repo}
}

// Handle 执行列表查询
func (h *listEndpointsHandler) Handle(ctx context.Context, q port.ListEndpointsQuery) ([]*port.EndpointView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	endpoints, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam)
	if err != nil {
		log.Error("[EndpointQuery] List endpoints failed", zap.Error(err))
		return nil, nil, err
	}

	views := lo.Map(endpoints, func(ep *aggregate.Endpoint, _ int) *port.EndpointView {
		return &port.EndpointView{
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
	})

	log.Info("[EndpointQuery] List endpoints", zap.Int("count", len(views)))
	return views, pageInfo, nil
}
