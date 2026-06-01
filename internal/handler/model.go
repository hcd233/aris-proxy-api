package handler

import (
	"context"

	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/model/command"
	"github.com/hcd233/aris-proxy-api/internal/application/model/query"
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
	Create command.CreateModelHandler
	Update command.UpdateModelHandler
	Delete command.DeleteModelHandler
	List   query.ListModelsHandler
}

type modelHandler struct {
	create command.CreateModelHandler
	update command.UpdateModelHandler
	delete command.DeleteModelHandler
	list   query.ListModelsHandler
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

	result, err := h.create.Handle(ctx, command.CreateModelCommand{
		Alias:      req.Body.Alias,
		ModelName:  req.Body.ModelName,
		EndpointID: req.Body.EndpointID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Create model failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	_ = result.ModelID
	logger.WithCtx(ctx).Info("[ModelHandler] Create model success",
		zap.Uint("userID", userID), zap.String("alias", req.Body.Alias))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *modelHandler) HandleListModels(ctx context.Context, req *dto.ListModelsReq) (*dto.HTTPResponse[*dto.ListModelsRsp], error) {
	rsp := &dto.ListModelsRsp{}

	views, pageInfo, err := h.list.Handle(ctx, query.ListModelsQuery{
		CommonParam: req.CommonParam,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] List models failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Models = make([]*dto.ModelItem, 0, len(views))
	for _, v := range views {
		item := &dto.ModelItem{
			ID:        v.ID,
			Alias:     v.Alias,
			ModelName: v.ModelName,
			CreatedAt: v.CreatedAt,
			UpdatedAt: v.UpdatedAt,
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
		rsp.Models = append(rsp.Models, item)
	}
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *modelHandler) HandleUpdateModel(ctx context.Context, req *dto.UpdateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.update.Handle(ctx, command.UpdateModelCommand{
		ModelID:    req.ID,
		Alias:      req.Body.Alias,
		ModelName:  req.Body.ModelName,
		EndpointID: req.Body.EndpointID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Update model failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *modelHandler) HandleDeleteModel(ctx context.Context, req *dto.DeleteModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.delete.Handle(ctx, command.DeleteModelCommand{ModelID: req.ID})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Delete model failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
