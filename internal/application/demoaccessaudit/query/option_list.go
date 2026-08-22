package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/demoaccessaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

type listDemoAccessAuditOptionsHandler struct {
	repo port.DemoAccessAuditRepository
}

// NewListDemoAccessAuditOptionsHandler 构造获取 Demo 访问审计筛选项处理器
//
//	@param repo port.DemoAccessAuditRepository
//	@return port.ListDemoAccessAuditOptionsHandler
//	@author centonhuang
//	@update 2026-08-23 10:00:00
func NewListDemoAccessAuditOptionsHandler(repo port.DemoAccessAuditRepository) port.ListDemoAccessAuditOptionsHandler {
	return &listDemoAccessAuditOptionsHandler{repo: repo}
}

// Handle 处理获取 Demo 访问审计筛选项请求
func (h *listDemoAccessAuditOptionsHandler) Handle(ctx context.Context, field, keyword string, startTime, endTime time.Time) ([]string, error) {
	switch field {
	case constant.DemoAccessAuditFilterFieldAction:
		return h.repo.ListDistinctActions(ctx, keyword, startTime, endTime)
	case constant.DemoAccessAuditFilterFieldModule:
		return h.repo.ListDistinctModules(ctx, keyword, startTime, endTime)
	default:
		return []string{}, nil
	}
}
