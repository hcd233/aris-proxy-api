package query

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// ListAuditOptionQuery 审计筛选选项查询
type ListAuditOptionQuery struct {
	Field   string
	Keyword string
}

// ListAuditOptionHandler 审计筛选选项查询处理器
type ListAuditOptionHandler interface {
	Handle(ctx context.Context, q ListAuditOptionQuery) ([]string, error)
}

type listAuditOptionHandler struct {
	repo modelcall.AuditRepository
}

// NewListAuditOptionHandler 构造审计筛选选项查询处理器
func NewListAuditOptionHandler(repo modelcall.AuditRepository) ListAuditOptionHandler {
	return &listAuditOptionHandler{repo: repo}
}

// Handle 执行筛选选项查询
func (h *listAuditOptionHandler) Handle(ctx context.Context, q ListAuditOptionQuery) ([]string, error) {
	switch q.Field {
	case "user":
		return h.repo.ListDistinctUserNames(ctx, q.Keyword)
	case "model":
		return h.repo.ListDistinctModels(ctx, q.Keyword)
	default:
		return []string{}, nil
	}
}
