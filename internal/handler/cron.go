package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	cronauditport "github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	cronmgmtport "github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// CronHandler Cron 相关 HTTP 处理器接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronHandler interface {
	HandleListCronJobs(ctx context.Context, req *dto.ListCronJobsReq) (*dto.HTTPResponse[*dto.ListCronJobsRsp], error)
	HandleUpdateCronJob(ctx context.Context, req *dto.UpdateCronJobReq) (*dto.HTTPResponse[*dto.UpdateCronJobRsp], error)
	HandleListCronCallAudits(ctx context.Context, req *dto.ListCronCallAuditsReq) (*dto.HTTPResponse[*dto.ListCronCallAuditsRsp], error)
	HandleListCronCallAuditOptions(ctx context.Context, req *dto.CronCallAuditOptionListReq) (*dto.HTTPResponse[*dto.CronCallAuditOptionListRsp], error)
}

// CronDependencies CronHandler 依赖
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronDependencies struct {
	ListCronJobs             cronmgmtport.ListCronJobsHandler
	UpdateCronJob            cronmgmtport.UpdateCronJobHandler
	ListCronCallAudits       cronauditport.ListCronCallAuditsHandler
	ListCronCallAuditOptions cronauditport.ListCronCallAuditOptionsHandler
}

type cronHandler struct {
	listCronJobs          cronmgmtport.ListCronJobsHandler
	updateCronJob         cronmgmtport.UpdateCronJobHandler
	listCronCallAudits    cronauditport.ListCronCallAuditsHandler
	listCronCallAuditOpts cronauditport.ListCronCallAuditOptionsHandler
}

// NewCronHandler 构造 CronHandler
//
//	@param deps CronDependencies
//	@return CronHandler
func NewCronHandler(deps CronDependencies) CronHandler {
	return &cronHandler{
		listCronJobs:          deps.ListCronJobs,
		updateCronJob:         deps.UpdateCronJob,
		listCronCallAudits:    deps.ListCronCallAudits,
		listCronCallAuditOpts: deps.ListCronCallAuditOptions,
	}
}

func (h *cronHandler) HandleListCronJobs(ctx context.Context, req *dto.ListCronJobsReq) (*dto.HTTPResponse[*dto.ListCronJobsRsp], error) {
	rsp := &dto.ListCronJobsRsp{}
	jobs, pageInfo, err := h.listCronJobs.Handle(ctx, model.CommonParam{
		PageParam:  model.PageParam{Page: req.Page, PageSize: req.PageSize},
		QueryParam: model.QueryParam{Query: req.Query},
		SortParam:  model.SortParam{Sort: req.Sort, SortField: req.SortField},
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[CronHandler] List cron jobs failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Jobs = lo.Map(jobs, func(job *cronmgmtport.CronJobView, _ int) *dto.CronJobItem {
		return &dto.CronJobItem{
			Name:        job.Name,
			Type:        job.Type,
			Spec:        job.Spec,
			Description: job.Description,
			Enabled:     job.Enabled,
			CreatedAt:   job.CreatedAt,
			UpdatedAt:   job.UpdatedAt,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *cronHandler) HandleUpdateCronJob(ctx context.Context, req *dto.UpdateCronJobReq) (*dto.HTTPResponse[*dto.UpdateCronJobRsp], error) {
	rsp := &dto.UpdateCronJobRsp{}
	if req.Body == nil {
		rsp.Error = ierr.ErrValidation.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	// 至少传一个字段
	if req.Body.Enabled == nil && req.Body.Spec == nil {
		rsp.Error = ierr.ErrValidation.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	params := cronmgmtport.UpdateCronJobParams{
		Enabled: req.Body.Enabled,
		Spec:    req.Body.Spec,
	}
	if err := h.updateCronJob.Handle(ctx, req.Name, params); err != nil {
		logger.WithCtx(ctx).Error("[CronHandler] Update cron job failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *cronHandler) HandleListCronCallAudits(ctx context.Context, req *dto.ListCronCallAuditsReq) (*dto.HTTPResponse[*dto.ListCronCallAuditsRsp], error) {
	rsp := &dto.ListCronCallAuditsRsp{}
	logs, pageInfo, err := h.listCronCallAudits.Handle(ctx,
		model.CommonParam{
			PageParam:  model.PageParam{Page: req.Page, PageSize: req.PageSize},
			QueryParam: model.QueryParam{Query: req.Query},
			SortParam:  model.SortParam{Sort: req.Sort, SortField: req.SortField},
		},
		req.StartTime, req.EndTime, req.Filter,
	)
	if err != nil {
		logger.WithCtx(ctx).Error("[CronHandler] List cron call audits failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Logs = lo.Map(logs, func(log *cronauditport.CronCallAuditView, _ int) *dto.CronCallAuditItem {
		return &dto.CronCallAuditItem{
			ID:         log.ID,
			CronName:   log.CronName,
			TraceID:    log.TraceID,
			StartedAt:  log.StartedAt,
			EndedAt:    log.EndedAt,
			DurationMs: log.DurationMs,
			Status:     log.Status,
			Message:    log.Message,
			CreatedAt:  log.CreatedAt,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *cronHandler) HandleListCronCallAuditOptions(ctx context.Context, req *dto.CronCallAuditOptionListReq) (*dto.HTTPResponse[*dto.CronCallAuditOptionListRsp], error) {
	rsp := &dto.CronCallAuditOptionListRsp{}
	items, err := h.listCronCallAuditOpts.Handle(ctx, req.Field, req.Keyword, req.StartTime, req.EndTime)
	if err != nil {
		logger.WithCtx(ctx).Error("[CronHandler] List cron call audit options failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Items = items
	return apiutil.WrapHTTPResponse(rsp, nil)
}
