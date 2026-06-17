package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// listCronCallAuditOptionsHandler 获取 CronCallAudit 筛选项处理器
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type listCronCallAuditOptionsHandler struct{ repo port.CronCallAuditRepository }

// NewListCronCallAuditOptionsHandler 构造获取 CronCallAudit 筛选项处理器
//
//	@param repo port.CronCallAuditRepository
//	@return port.ListCronCallAuditOptionsHandler
func NewListCronCallAuditOptionsHandler(repo port.CronCallAuditRepository) port.ListCronCallAuditOptionsHandler {
	return &listCronCallAuditOptionsHandler{repo: repo}
}

// Handle 处理获取 CronCallAudit 筛选项请求
//
//	@receiver h *listCronCallAuditOptionsHandler
//	@param ctx context.Context
//	@param field string
//	@param keyword string
//	@param startTime time.Time
//	@param endTime time.Time
//	@return []string
//	@return error
func (h *listCronCallAuditOptionsHandler) Handle(ctx context.Context, field, keyword string, startTime, endTime time.Time) ([]string, error) {
	switch field {
	case constant.CronAuditFilterFieldType:
		return h.repo.ListDistinctTypes(ctx, keyword, startTime, endTime)
	case constant.CronAuditFilterFieldStatus:
		return h.repo.ListDistinctStatuses(ctx, keyword, startTime, endTime)
	default:
		return []string{}, nil
	}
}
