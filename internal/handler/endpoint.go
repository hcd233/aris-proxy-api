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
	HandleUpdateEndpoint(ctx context.Context, req *dto.UpdateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteEndpoint(ctx context.Context, req *dto.DeleteEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type EndpointDependencies struct {
	Create port.CreateEndpointHandler
	Update port.UpdateEndpointHandler
	Delete port.DeleteEndpointHandler
}

type endpointHandler struct {
	create port.CreateEndpointHandler
	update port.UpdateEndpointHandler
	delete port.DeleteEndpointHandler
}

func NewEndpointHandler(deps EndpointDependencies) EndpointHandler {
	return &endpointHandler{
		create: deps.Create,
		update: deps.Update,
		delete: deps.Delete,
	}
}

func (h *endpointHandler) HandleCreateEndpoint(ctx context.Context, req *dto.CreateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	isAdmin := util.CtxValuePermission(ctx) == enum.PermissionAdmin
	ownerUserID := lo.Ternary(isAdmin && req.Body.OwnerUserID != nil, lo.FromPtr(req.Body.OwnerUserID), userID)
	_, err := h.create.Handle(ctx, port.CreateEndpointCommand{
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

	logger.WithCtx(ctx).Info("[EndpointHandler] Create endpoint success",
		zap.Uint("userID", userID), zap.String("name", req.Body.Name))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *endpointHandler) HandleUpdateEndpoint(ctx context.Context, req *dto.UpdateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	err = h.update.Handle(ctx, port.UpdateEndpointCommand{
		ScopeUserID:                 scope,
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
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	err = h.delete.Handle(ctx, port.DeleteEndpointCommand{
		ScopeUserID: scope,
		EndpointID:  req.ID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Delete endpoint failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// scopeFor 多租户隔离 scope 计算：admin 返回 nil（不过滤），其余用户限定自身。
//
// 非 admin 且 ctx 缺 userID（==0，认证中间件异常）时返回错误——
// 0 若被当作"全量视角"哨兵会让请求静默退化为全平台可见。
func scopeFor(ctx context.Context, perm enum.Permission) (*uint, error) {
	if perm == enum.PermissionAdmin {
		return nil, nil
	}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if userID == 0 {
		return nil, ierr.New(ierr.ErrUnauthorized, "user id is required for non-admin scope")
	}
	return &userID, nil
}
