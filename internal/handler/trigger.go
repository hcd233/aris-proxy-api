package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/trigger/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type TriggerHandler interface {
	HandleCreateTrigger(ctx context.Context, req *dto.CreateTriggerReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleListTrigger(ctx context.Context, req *dto.ListTriggerReq) (*dto.HTTPResponse[*dto.ListTriggerRsp], error)
	HandleUpdateTrigger(ctx context.Context, req *dto.UpdateTriggerReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteTrigger(ctx context.Context, req *dto.DeleteTriggerReq) (*dto.HTTPResponse[*dto.DeleteTriggerRsp], error)
}

type TriggerDependencies struct {
	Create port.CreateTriggerHandler
	Update port.UpdateTriggerHandler
	Delete port.DeleteTriggerHandler
	List   port.ListTriggerHandler
}

type triggerHandler struct {
	create port.CreateTriggerHandler
	update port.UpdateTriggerHandler
	delete port.DeleteTriggerHandler
	list   port.ListTriggerHandler
}

func NewTriggerHandler(deps TriggerDependencies) TriggerHandler {
	return &triggerHandler{
		create: deps.Create,
		update: deps.Update,
		delete: deps.Delete,
		list:   deps.List,
	}
}

func (h *triggerHandler) HandleCreateTrigger(ctx context.Context, req *dto.CreateTriggerReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	result, err := h.create.Handle(ctx, port.CreateTriggerCommand{
		Word:   req.Body.Word,
		Action: req.Body.Action,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[TriggerHandler] Create trigger word failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	_ = result.TriggerID
	logger.WithCtx(ctx).Info("[TriggerHandler] Create trigger word success",
		zap.Uint("userID", userID), zap.String("word", req.Body.Word))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *triggerHandler) HandleListTrigger(ctx context.Context, req *dto.ListTriggerReq) (*dto.HTTPResponse[*dto.ListTriggerRsp], error) {
	rsp := &dto.ListTriggerRsp{}

	views, pageInfo, err := h.list.Handle(ctx, port.ListTriggerQuery{
		CommonParam: req.CommonParam,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[TriggerHandler] List trigger words failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.Trigger = lo.Map(views, func(v *port.TriggerView, _ int) *dto.TriggerItem {
		return &dto.TriggerItem{
			ID:        v.ID,
			Word:      v.Word,
			Action:    v.Action,
			HitCount:  v.HitCount,
			CreatedAt: v.CreatedAt,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *triggerHandler) HandleUpdateTrigger(ctx context.Context, req *dto.UpdateTriggerReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	if req.Body == nil || req.Body.Action == nil {
		return nil, apiutil.NewHumaBizErrorFromModel(ctx, ierr.ErrValidation.BizError())
	}

	err := h.update.Handle(ctx, port.UpdateTriggerCommand{TriggerID: req.ID, Action: *req.Body.Action})
	if err != nil {
		logger.WithCtx(ctx).Error("[TriggerHandler] Update trigger word failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	logger.WithCtx(ctx).Info("[TriggerHandler] Update trigger word success",
		zap.Uint("triggerID", req.ID), zap.String("action", *req.Body.Action))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleDeleteTrigger 删除触发词（支持逗号分隔批量删除）
func (h *triggerHandler) HandleDeleteTrigger(ctx context.Context, req *dto.DeleteTriggerReq) (*dto.HTTPResponse[*dto.DeleteTriggerRsp], error) {
	rsp := &dto.DeleteTriggerRsp{}

	ids, parseErr := parseCommaSeparatedIDs(req.IDs)
	if parseErr != nil {
		return nil, apiutil.NewHumaBizError(ctx, parseErr, ierr.ErrValidation.BizError())
	}

	err := h.delete.Handle(ctx, port.DeleteTriggerCommand{TriggerIDs: ids})
	if err != nil {
		logger.WithCtx(ctx).Error("[TriggerHandler] Delete trigger word failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.DeletedCount = len(ids)
	logger.WithCtx(ctx).Info("[TriggerHandler] Trigger word(s) deleted", zap.Int("total", len(ids)))

	return apiutil.WrapHTTPResponse(rsp, nil)
}
