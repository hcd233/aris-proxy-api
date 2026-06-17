package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
)

// listCronCallAuditsHandler 列出 CronCallAudit 处理器
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type listCronCallAuditsHandler struct{ repo port.CronCallAuditRepository }

// NewListCronCallAuditsHandler 构造列出 CronCallAudit 处理器
//
//	@param repo port.CronCallAuditRepository
//	@return port.ListCronCallAuditsHandler
func NewListCronCallAuditsHandler(repo port.CronCallAuditRepository) port.ListCronCallAuditsHandler {
	return &listCronCallAuditsHandler{repo: repo}
}

// Handle 处理列出 CronCallAudit 请求
//
//	@receiver h *listCronCallAuditsHandler
//	@param ctx context.Context
//	@param param model.CommonParam
//	@param startTime time.Time
//	@param endTime time.Time
//	@param filterStr string
//	@return []*port.CronCallAuditView
//	@return *model.PageInfo
//	@return error
func (h *listCronCallAuditsHandler) Handle(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, filterStr string) ([]*port.CronCallAuditView, *model.PageInfo, error) {
	daoParam := dao.CommonParam{
		PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
		SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldCronName, constant.FieldTraceID}},
	}
	return h.repo.List(ctx, daoParam, startTime, endTime, filterStr)
}
