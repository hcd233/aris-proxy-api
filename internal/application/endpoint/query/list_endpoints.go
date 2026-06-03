package query

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ListEndpointsQuery 列出 Endpoints 查询命令
type ListEndpointsQuery struct {
	model.CommonParam
}

// EndpointView Endpoint 只读投影
type EndpointView struct {
	ID                          uint
	Name                        string
	OpenaiBaseURL               string
	AnthropicBaseURL            string
	MaskedAPIKey                string
	SupportOpenAIChatCompletion bool
	SupportOpenAIResponse       bool
	SupportAnthropicMessage     bool
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

// ListEndpointsHandler 查询处理器
type ListEndpointsHandler interface {
	Handle(ctx context.Context, q ListEndpointsQuery) ([]*EndpointView, *model.PageInfo, error)
}

type listEndpointsHandler struct {
	repo llmproxy.EndpointRepository
}

// NewListEndpointsHandler 构造查询处理器
func NewListEndpointsHandler(repo llmproxy.EndpointRepository) ListEndpointsHandler {
	return &listEndpointsHandler{repo: repo}
}

// Handle 执行列表查询
func (h *listEndpointsHandler) Handle(ctx context.Context, q ListEndpointsQuery) ([]*EndpointView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	endpoints, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam)
	if err != nil {
		log.Error("[EndpointQuery] List endpoints failed", zap.Error(err))
		return nil, nil, err
	}

	views := make([]*EndpointView, 0, len(endpoints))
	for _, ep := range endpoints {
		views = append(views, &EndpointView{
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
		})
	}

	log.Info("[EndpointQuery] List endpoints", zap.Int("count", len(views)))
	return views, pageInfo, nil
}
