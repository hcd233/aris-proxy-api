package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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
	Create port.CreateEndpointHandler
	Update port.UpdateEndpointHandler
	Delete port.DeleteEndpointHandler
	List   port.ListEndpointsHandler
}

type endpointHandler struct {
	create port.CreateEndpointHandler
	update port.UpdateEndpointHandler
	delete port.DeleteEndpointHandler
	list   port.ListEndpointsHandler
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

	isAdmin := util.CtxValuePermission(ctx) == enum.PermissionAdmin
	ownerUserID := lo.Ternary(isAdmin && req.Body.OwnerUserID != nil, lo.FromPtr(req.Body.OwnerUserID), userID)
	result, err := h.create.Handle(ctx, port.CreateEndpointCommand{
		OwnerUserID:                 ownerUserID,
		Name:                        req.Body.Name,
		OpenaiBaseURL:               lo.FromPtr(req.Body.OpenaiBaseURL),
		AnthropicBaseURL:            lo.FromPtr(req.Body.AnthropicBaseURL),
		APIKey:                      req.Body.APIKey,
		SupportOpenAIChatCompletion: lo.FromPtr(req.Body.SupportOpenAIChatCompletion),
		SupportOpenAIResponse:       lo.FromPtr(req.Body.SupportOpenAIResponse),
		SupportAnthropicMessage:     lo.FromPtr(req.Body.SupportAnthropicMessage),
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Create endpoint failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	_ = result.EndpointID
	logger.WithCtx(ctx).Info("[EndpointHandler] Create endpoint success",
		zap.Uint("userID", userID), zap.String("name", req.Body.Name))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *endpointHandler) HandleListEndpoints(ctx context.Context, req *dto.ListEndpointsReq) (*dto.HTTPResponse[*dto.ListEndpointsRsp], error) {
	rsp := &dto.ListEndpointsRsp{}

	perm := util.CtxValuePermission(ctx)
	isGlobalScope := perm == enum.PermissionAdmin
	views, pageInfo, err := h.list.Handle(ctx, port.ListEndpointsQuery{
		CommonParam: req.CommonParam,
		IsDemo:      perm == enum.PermissionDemo,
		ScopeUserID: lo.Ternary(isGlobalScope, 0, util.CtxValueUint(ctx, constant.CtxKeyUserID)),
		Username:    req.Username,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] List endpoints failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.Endpoints = lo.Map(views, func(v *port.EndpointView, _ int) *dto.EndpointItem {
		return &dto.EndpointItem{
			ID:                          v.ID,
			Username:                    v.Username,
			Name:                        v.Name,
			OpenaiBaseURL:               v.OpenaiBaseURL,
			AnthropicBaseURL:            v.AnthropicBaseURL,
			MaskedAPIKey:                v.MaskedAPIKey,
			SupportOpenAIChatCompletion: v.SupportOpenAIChatCompletion,
			SupportOpenAIResponse:       v.SupportOpenAIResponse,
			SupportAnthropicMessage:     v.SupportAnthropicMessage,
			CreatedAt:                   v.CreatedAt,
			UpdatedAt:                   v.UpdatedAt,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *endpointHandler) HandleUpdateEndpoint(ctx context.Context, req *dto.UpdateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.update.Handle(ctx, port.UpdateEndpointCommand{
		ScopeUserID:                 scopeFor(ctx, util.CtxValuePermission(ctx)),
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
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *endpointHandler) HandleDeleteEndpoint(ctx context.Context, req *dto.DeleteEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.delete.Handle(ctx, port.DeleteEndpointCommand{
		ScopeUserID: scopeFor(ctx, util.CtxValuePermission(ctx)),
		EndpointID:  req.ID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Delete endpoint failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// scopeFor 多租户隔离 scope 计算：admin 返回 0（不过滤），其余用户限定自身。
func scopeFor(ctx context.Context, perm enum.Permission) uint {
	if perm == enum.PermissionAdmin {
		return 0
	}
	return util.CtxValueUint(ctx, constant.CtxKeyUserID)
}
