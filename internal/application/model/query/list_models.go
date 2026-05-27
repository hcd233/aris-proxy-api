package query

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ListModelsQuery 列出 Models 查询命令
type ListModelsQuery struct {
	// Page 页码
	Page int
	// PageSize 每页数量
	PageSize int
	// Query 搜索关键词
	Query string
	// Sort 排序方式
	Sort string
	// SortField 排序字段
	SortField string
}

// ModelView Model 只读投影
type ModelView struct {
	ID         uint
	Alias      string
	ModelName  string
	EndpointID uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ListModelsHandler 查询处理器
type ListModelsHandler interface {
	Handle(ctx context.Context, q ListModelsQuery) ([]*ModelView, *model.PageInfo, error)
}

type listModelsHandler struct {
	repo llmproxy.ModelRepository
}

// NewListModelsHandler 构造查询处理器
func NewListModelsHandler(repo llmproxy.ModelRepository) ListModelsHandler {
	return &listModelsHandler{repo: repo}
}

// Handle 执行列表查询
func (h *listModelsHandler) Handle(ctx context.Context, q ListModelsQuery) ([]*ModelView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	param := llmproxy.PageParam{
		Page:      q.Page,
		PageSize:  q.PageSize,
		Query:     q.Query,
		Sort:      q.Sort,
		SortField: q.SortField,
	}

	models, pageInfo, err := h.repo.Paginate(ctx, param)
	if err != nil {
		log.Error("[ModelQuery] List models failed", zap.Error(err))
		return nil, nil, err
	}

	views := make([]*ModelView, 0, len(models))
	for _, m := range models {
		views = append(views, &ModelView{
			ID:         m.AggregateID(),
			Alias:      m.Alias().String(),
			ModelName:  m.ModelName(),
			EndpointID: m.EndpointID(),
			CreatedAt:  m.CreatedAt(),
			UpdatedAt:  m.UpdatedAt(),
		})
	}

	log.Info("[ModelQuery] List models", zap.Int("count", len(views)))
	return views, pageInfo, nil
}
