package handler

import (
	"context"

	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type EndpointHandler interface {
	HandleCreateEndpoint(ctx context.Context, req *dto.CreateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleListEndpoints(ctx context.Context, req *dto.ListEndpointsReq) (*dto.HTTPResponse[*dto.ListEndpointsRsp], error)
	HandleUpdateEndpoint(ctx context.Context, req *dto.UpdateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteEndpoint(ctx context.Context, req *dto.DeleteEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type EndpointDependencies struct {
	Create command.CreateEndpointHandler
	Update command.UpdateEndpointHandler
	Delete command.DeleteEndpointHandler
	List   query.ListEndpointsHandler
}

type endpointHandler struct {
	create command.CreateEndpointHandler
	update command.UpdateEndpointHandler
	delete command.DeleteEndpointHandler
	list   query.ListEndpointsHandler
}

func NewEndpointHandler(deps EndpointDependencies) EndpointHandler {
	return &endpointHandler{
		create: deps.Create,
		update: deps.Update,
		delete: deps.Delete,
		list:   deps.List,
	}
}

func (h *endpointHandler) HandleCreateEndpoint(ctx context.Context, req *dto.CreateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	result, err := h.create.Handle(ctx, command.CreateEndpointCommand{
		Name:                        req.Body.Name,
		OpenaiBaseURL:               req.Body.OpenaiBaseURL,
		AnthropicBaseURL:            req.Body.AnthropicBaseURL,
		APIKey:                      req.Body.APIKey,
		SupportOpenAIChatCompletion: req.Body.SupportOpenAIChatCompletion,
		SupportOpenAIResponse:       req.Body.SupportOpenAIResponse,
		SupportAnthropicMessage:     req.Body.SupportAnthropicMessage,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Create endpoint failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	_ = result.EndpointID
	logger.WithCtx(ctx).Info("[EndpointHandler] Create endpoint success",
		zap.Uint("userID", userID), zap.String("name", req.Body.Name))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *endpointHandler) HandleListEndpoints(ctx context.Context, req *dto.ListEndpointsReq) (*dto.HTTPResponse[*dto.ListEndpointsRsp], error) {
	rsp := &dto.ListEndpointsRsp{}

	views, pageInfo, err := h.list.Handle(ctx, query.ListEndpointsQuery{
		CommonParam: req.CommonParam,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] List endpoints failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Endpoints = make([]*dto.EndpointItem, 0, len(views))
	for _, v := range views {
		rsp.Endpoints = append(rsp.Endpoints, &dto.EndpointItem{
			ID:                          v.ID,
			Name:                        v.Name,
			OpenaiBaseURL:               v.OpenaiBaseURL,
			AnthropicBaseURL:            v.AnthropicBaseURL,
			MaskedAPIKey:                v.MaskedAPIKey,
			SupportOpenAIChatCompletion: v.SupportOpenAIChatCompletion,
			SupportOpenAIResponse:       v.SupportOpenAIResponse,
			SupportAnthropicMessage:     v.SupportAnthropicMessage,
			CreatedAt:                   v.CreatedAt,
			UpdatedAt:                   v.UpdatedAt,
		})
	}
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *endpointHandler) HandleUpdateEndpoint(ctx context.Context, req *dto.UpdateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.update.Handle(ctx, command.UpdateEndpointCommand{
		EndpointID:                  req.ID,
		Name:                        req.Body.Name,
		OpenaiBaseURL:               req.Body.OpenaiBaseURL,
		AnthropicBaseURL:            req.Body.AnthropicBaseURL,
		APIKey:                      req.Body.APIKey,
		SupportOpenAIChatCompletion: req.Body.SupportOpenAIChatCompletion,
		SupportOpenAIResponse:       req.Body.SupportOpenAIResponse,
		SupportAnthropicMessage:     req.Body.SupportAnthropicMessage,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Update endpoint failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *endpointHandler) HandleDeleteEndpoint(ctx context.Context, req *dto.DeleteEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.delete.Handle(ctx, command.DeleteEndpointCommand{EndpointID: req.ID})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Delete endpoint failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
