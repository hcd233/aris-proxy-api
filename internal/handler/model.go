package handler

import (
	"context"

	"github.com/samber/lo"
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
	HandleListModels(ctx context.Context, req *dto.ListModelsReq) (*dto.HTTPResponse[*dto.ListModelsRsp], error)
	HandleUpdateModel(ctx context.Context, req *dto.UpdateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteModel(ctx context.Context, req *dto.DeleteModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type ModelDependencies struct {
	Create port.CreateModelHandler
	Update port.UpdateModelHandler
	Delete port.DeleteModelHandler
	List   port.ListModelsHandler
}

type modelHandler struct {
	create port.CreateModelHandler
	update port.UpdateModelHandler
	delete port.DeleteModelHandler
	list   port.ListModelsHandler
}

func NewModelHandler(deps ModelDependencies) ModelHandler {
	return &modelHandler{
		create: deps.Create,
		update: deps.Update,
		delete: deps.Delete,
		list:   deps.List,
	}
}

func (h *modelHandler) HandleCreateModel(ctx context.Context, req *dto.CreateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	result, err := h.create.Handle(ctx, port.CreateModelCommand{
		Alias:           req.Body.Alias,
		ModelName:       req.Body.ModelName,
		EndpointID:      req.Body.EndpointID,
		ContextLength:   req.Body.ContextLength,
		MaxOutputTokens: req.Body.MaxOutputTokens,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Create model failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	_ = result.ModelID
	logger.WithCtx(ctx).Info("[ModelHandler] Create model success",
		zap.Uint("userID", userID), zap.String("alias", req.Body.Alias))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *modelHandler) HandleListModels(ctx context.Context, req *dto.ListModelsReq) (*dto.HTTPResponse[*dto.ListModelsRsp], error) {
	rsp := &dto.ListModelsRsp{}

	views, pageInfo, err := h.list.Handle(ctx, port.ListModelsQuery{
		CommonParam: req.CommonParam,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] List models failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Models = lo.Map(views, func(v *port.ModelView, _ int) *dto.ModelItem {
		item := &dto.ModelItem{
			ID:              v.ID,
			Alias:           v.Alias,
			ModelName:       v.ModelName,
			Enabled:         v.Enabled,
			ContextLength:   v.ContextLength,
			MaxOutputTokens: v.MaxOutputTokens,
			CreatedAt:       v.CreatedAt,
			UpdatedAt:       v.UpdatedAt,
		}
		if v.Endpoint != nil {
			item.Endpoint = &dto.EndpointItem{
				ID:                          v.Endpoint.ID,
				Name:                        v.Endpoint.Name,
				OpenaiBaseURL:               v.Endpoint.OpenaiBaseURL,
				AnthropicBaseURL:            v.Endpoint.AnthropicBaseURL,
				MaskedAPIKey:                v.Endpoint.MaskedAPIKey,
				SupportOpenAIChatCompletion: v.Endpoint.SupportOpenAIChatCompletion,
				SupportOpenAIResponse:       v.Endpoint.SupportOpenAIResponse,
				SupportAnthropicMessage:     v.Endpoint.SupportAnthropicMessage,
				CreatedAt:                   v.Endpoint.CreatedAt,
				UpdatedAt:                   v.Endpoint.UpdatedAt,
			}
		}
		return item
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *modelHandler) HandleUpdateModel(ctx context.Context, req *dto.UpdateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.update.Handle(ctx, port.UpdateModelCommand{
		ModelID:         req.ID,
		Alias:           req.Body.Alias,
		ModelName:       req.Body.ModelName,
		EndpointID:      req.Body.EndpointID,
		Enabled:         req.Body.Enabled,
		ContextLength:   req.Body.ContextLength,
		MaxOutputTokens: req.Body.MaxOutputTokens,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Update model failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *modelHandler) HandleDeleteModel(ctx context.Context, req *dto.DeleteModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.delete.Handle(ctx, port.DeleteModelCommand{ModelID: req.ID})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Delete model failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
