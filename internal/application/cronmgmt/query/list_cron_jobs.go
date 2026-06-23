package query

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// listCronJobsHandler 列出 CronJob 处理器
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type listCronJobsHandler struct{ repo port.CronJobRepository }

// NewListCronJobsHandler 构造列出 CronJob 处理器
//
//	@param repo port.CronJobRepository
//	@return port.ListCronJobsHandler
func NewListCronJobsHandler(repo port.CronJobRepository) port.ListCronJobsHandler {
	return &listCronJobsHandler{repo: repo}
}

// Handle 处理列出 CronJob 请求
//
//	@receiver h *listCronJobsHandler
//	@param ctx context.Context
//	@param param model.CommonParam
//	@return []*port.CronJobView
//	@return *model.PageInfo
//	@return error
func (h *listCronJobsHandler) Handle(ctx context.Context, param model.CommonParam) ([]*port.CronJobView, *model.PageInfo, error) {
	if param.SortField == "" {
		param.SortField = constant.FieldName
	}
	param.QueryFields = []string{constant.FieldName, constant.FieldSpec}
	return h.repo.List(ctx, param)
}
