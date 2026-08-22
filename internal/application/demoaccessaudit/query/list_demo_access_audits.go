package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/demoaccessaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

type listDemoAccessAuditsHandler struct {
	repo port.DemoAccessAuditRepository
}

// NewListDemoAccessAuditsHandler 构造列出 Demo 访问审计处理器
//
//	@param repo port.DemoAccessAuditRepository
//	@return port.ListDemoAccessAuditsHandler
//	@author centonhuang
//	@update 2026-08-23 10:00:00
func NewListDemoAccessAuditsHandler(repo port.DemoAccessAuditRepository) port.ListDemoAccessAuditsHandler {
	return &listDemoAccessAuditsHandler{repo: repo}
}

// Handle 处理列出 Demo 访问审计请求
func (h *listDemoAccessAuditsHandler) Handle(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, filterStr string) ([]*port.DemoAccessAuditView, *model.PageInfo, error) {
	param.QueryFields = []string{constant.FieldPath, constant.FieldIP}
	return h.repo.List(ctx, param, startTime, endTime, filterStr)
}
