package handler

import (
	"context"

	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type ModelHandler interface {
	HandleCreateModel(ctx context.Context, req *dto.CreateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleUpdateModel(ctx context.Context, req *dto.UpdateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteModel(ctx context.Context, req *dto.DeleteModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type ModelDependencies struct {
	Create port.CreateModelHandler
	Update port.UpdateModelHandler
	Delete port.DeleteModelHandler
}

type modelHandler struct {
	create port.CreateModelHandler
	update port.UpdateModelHandler
	delete port.DeleteModelHandler
}

func NewModelHandler(deps ModelDependencies) ModelHandler {
	return &modelHandler{
		create: deps.Create,
		update: deps.Update,
		delete: deps.Delete,
	}
}

func (h *modelHandler) HandleCreateModel(ctx context.Context, req *dto.CreateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	_, err := h.create.Handle(ctx, port.CreateModelCommand{
		ScopeUserID:     scopeFor(ctx, util.CtxValuePermission(ctx)),
		Alias:           req.Body.Alias,
		ModelID:         req.Body.ModelID,
		UpstreamModel:   req.Body.UpstreamModel,
		EndpointID:      req.Body.EndpointID,
		ContextLength:   req.Body.ContextLength,
		MaxOutputTokens: req.Body.MaxOutputTokens,
		Capabilities:    req.Body.Capabilities,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Create model failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	logger.WithCtx(ctx).Info("[ModelHandler] Create model success",
		zap.Uint("userID", userID), zap.String("alias", req.Body.Alias))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *modelHandler) HandleUpdateModel(ctx context.Context, req *dto.UpdateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.update.Handle(ctx, port.UpdateModelCommand{
		ScopeUserID:     scopeFor(ctx, util.CtxValuePermission(ctx)),
		ID:              req.ID,
		Alias:           req.Body.Alias,
		UpstreamModel:   req.Body.UpstreamModel,
		EndpointID:      req.Body.EndpointID,
		Enabled:         req.Body.Enabled,
		ContextLength:   req.Body.ContextLength,
		MaxOutputTokens: req.Body.MaxOutputTokens,
		Capabilities:    req.Body.Capabilities,
		ModelID:         req.Body.ModelID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Update model failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *modelHandler) HandleDeleteModel(ctx context.Context, req *dto.DeleteModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.delete.Handle(ctx, port.DeleteModelCommand{
		ScopeUserID: scopeFor(ctx, util.CtxValuePermission(ctx)),
		ModelID:     req.ID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Delete model failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
