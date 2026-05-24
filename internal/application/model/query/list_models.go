package query

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ListModelsQuery 列出 Models 查询命令
type ListModelsQuery struct{}

// ModelView Model 只读投影
type ModelView struct {
	ID         uint
	Alias      string
	ModelName  string
	EndpointID uint
}

// ListModelsHandler 查询处理器
type ListModelsHandler interface {
	Handle(ctx context.Context, q ListModelsQuery) ([]*ModelView, error)
}

type listModelsHandler struct {
	repo llmproxy.ModelRepository
}

// NewListModelsHandler 构造查询处理器
func NewListModelsHandler(repo llmproxy.ModelRepository) ListModelsHandler {
	return &listModelsHandler{repo: repo}
}

// Handle 执行列表查询
func (h *listModelsHandler) Handle(ctx context.Context, _ ListModelsQuery) ([]*ModelView, error) {
	log := logger.WithCtx(ctx)

	models, err := h.repo.List(ctx)
	if err != nil {
		log.Error("[ModelQuery] List models failed", zap.Error(err))
		return nil, err
	}

	views := make([]*ModelView, 0, len(models))
	for _, m := range models {
		views = append(views, &ModelView{
			ID:         m.AggregateID(),
			Alias:      m.Alias().String(),
			ModelName:  m.ModelName(),
			EndpointID: m.EndpointID(),
		})
	}

	log.Info("[ModelQuery] List models", zap.Int("count", len(views)))
	return views, nil
}
