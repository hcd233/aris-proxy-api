package query

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"

	endpointquery "github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
)

// ListModelsQuery 列出 Models 查询命令
type ListModelsQuery struct {
	model.CommonParam
}

// ModelView Model 只读投影
type ModelView struct {
	ID        uint
	Alias     string
	ModelName string
	Endpoint  *endpointquery.EndpointView
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ListModelsHandler 查询处理器
type ListModelsHandler interface {
	Handle(ctx context.Context, q ListModelsQuery) ([]*ModelView, *model.PageInfo, error)
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
func (h *listModelsHandler) Handle(ctx context.Context, q ListModelsQuery) ([]*ModelView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	models, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam)
	if err != nil {
		log.Error("[ModelQuery] List models failed", zap.Error(err))
		return nil, nil, err
	}

	views := make([]*ModelView, 0, len(models))
	for _, m := range models {
		endpointView, err := h.getEndpointView(ctx, m.EndpointID())
		if err != nil {
			log.Error("[ModelQuery] Get endpoint failed", zap.Error(err))
			return nil, nil, err
		}

		views = append(views, &ModelView{
			ID:        m.AggregateID(),
			Alias:     m.Alias().String(),
			ModelName: m.ModelName(),
			Endpoint:  endpointView,
			CreatedAt: m.CreatedAt(),
			UpdatedAt: m.UpdatedAt(),
		})
	}

	log.Info("[ModelQuery] List models", zap.Int("count", len(views)))
	return views, pageInfo, nil
}

func (h *listModelsHandler) getEndpointView(ctx context.Context, endpointID uint) (*endpointquery.EndpointView, error) {
	ep, err := h.endpointRepo.FindByID(ctx, endpointID)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, nil
	}

	return &endpointquery.EndpointView{
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
	}, nil
}
