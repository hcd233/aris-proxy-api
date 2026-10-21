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
	HandleAuthorizeOAuth(ctx context.Context, req *dto.AuthorizeOAuthReq) (*dto.HTTPResponse[*dto.AuthorizeOAuthRspBody], error)
	HandleExchangeOAuth(ctx context.Context, req *dto.ExchangeOAuthReq) (*dto.HTTPResponse[*dto.ExchangeOAuthRspBody], error)
	HandleReauthOAuth(ctx context.Context, req *dto.ReauthOAuthReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleGetOAuthQuota(ctx context.Context, req *dto.GetOAuthQuotaReq) (*dto.HTTPResponse[*dto.GetOAuthQuotaRspBody], error)
	HandleProbeOAuthQuota(ctx context.Context, req *dto.ProbeOAuthQuotaReq) (*dto.HTTPResponse[*dto.GetOAuthQuotaRspBody], error)
}

type EndpointDependencies struct {
	Create   port.CreateEndpointHandler
	Update   port.UpdateEndpointHandler
	Delete   port.DeleteEndpointHandler
	OAuth    port.AuthorizeOAuthHandler
	Exchange port.ExchangeOAuthHandler
	Reauth   port.ReauthOAuthHandler
	Quota    port.GetOAuthQuotaHandler
	Probe    port.ProbeOAuthQuotaHandler
}

type endpointHandler struct {
	create   port.CreateEndpointHandler
	update   port.UpdateEndpointHandler
	delete   port.DeleteEndpointHandler
	oauth    port.AuthorizeOAuthHandler
	exchange port.ExchangeOAuthHandler
	reauth   port.ReauthOAuthHandler
	quota    port.GetOAuthQuotaHandler
	probe    port.ProbeOAuthQuotaHandler
}

func NewEndpointHandler(deps EndpointDependencies) EndpointHandler {
	return &endpointHandler{
		create:   deps.Create,
		update:   deps.Update,
		delete:   deps.Delete,
		oauth:    deps.OAuth,
		exchange: deps.Exchange,
		reauth:   deps.Reauth,
		quota:    deps.Quota,
		probe:    deps.Probe,
	}
}

func (h *endpointHandler) HandleCreateEndpoint(ctx context.Context, req *dto.CreateEndpointReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	perm := util.CtxValuePermission(ctx)
	isAdmin := perm == enum.PermissionAdmin
	visibility := enum.EndpointVisibilityPrivate
	if req.Body.Visibility != nil {
		if !isAdmin {
			return nil, apiutil.NewHumaBizError(ctx, ierr.ErrUnauthorized, ierr.ErrUnauthorized.BizError())
		}
		visibility = req.Body.Visibility.Normalize()
		if !visibility.IsValid() {
			return nil, apiutil.NewHumaBizError(ctx, ierr.ErrValidation, ierr.ErrValidation.BizError())
		}
	}
	// 公开端点归属强制为操作者本人：不允许代建公开端点给普通用户。
	ownerUserID := lo.Ternary(isAdmin && req.Body.OwnerUserID != nil && !visibility.IsPublic(), lo.FromPtr(req.Body.OwnerUserID), userID)
	_, err := h.create.Handle(ctx, port.CreateEndpointCommand{
		OwnerUserID:                 ownerUserID,
		Visibility:                  visibility,
		Name:                        req.Body.Name,
		OpenaiBaseURL:               lo.FromPtr(req.Body.OpenaiBaseURL),
		AnthropicBaseURL:            lo.FromPtr(req.Body.AnthropicBaseURL),
		APIKey:                      req.Body.APIKey,
		SupportOpenAIChatCompletion: lo.FromPtr(req.Body.SupportOpenAIChatCompletion),
		SupportOpenAIResponse:       lo.FromPtr(req.Body.SupportOpenAIResponse),
		SupportAnthropicMessage:     lo.FromPtr(req.Body.SupportAnthropicMessage),
		SupportOpenAIDecision:       lo.FromPtr(req.Body.SupportOpenAIDecision),
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
	perm := util.CtxValuePermission(ctx)
	isAdmin := perm == enum.PermissionAdmin
	if req.Body.Visibility != nil {
		if !isAdmin {
			return nil, apiutil.NewHumaBizError(ctx, ierr.ErrUnauthorized, ierr.ErrUnauthorized.BizError())
		}
		if !req.Body.Visibility.IsValid() {
			return nil, apiutil.NewHumaBizError(ctx, ierr.ErrValidation, ierr.ErrValidation.BizError())
		}
	}
	scope, err := scopeFor(ctx, perm)
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	err = h.update.Handle(ctx, port.UpdateEndpointCommand{
		ScopeUserID:                 scope,
		EndpointID:                  req.ID,
		IsAdmin:                     isAdmin,
		Visibility:                  req.Body.Visibility,
		Name:                        req.Body.Name,
		OpenaiBaseURL:               req.Body.OpenaiBaseURL,
		AnthropicBaseURL:            req.Body.AnthropicBaseURL,
		APIKey:                      req.Body.APIKey,
		SupportOpenAIChatCompletion: req.Body.SupportOpenAIChatCompletion,
		SupportOpenAIResponse:       req.Body.SupportOpenAIResponse,
		SupportAnthropicMessage:     req.Body.SupportAnthropicMessage,
		SupportOpenAIDecision:       req.Body.SupportOpenAIDecision,
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
		IsAdmin:     util.CtxValuePermission(ctx) == enum.PermissionAdmin,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Delete endpoint failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleAuthorizeOAuth 发起订阅账号授权（生成授权链接与一次性 state）
func (h *endpointHandler) HandleAuthorizeOAuth(ctx context.Context, req *dto.AuthorizeOAuthReq) (*dto.HTTPResponse[*dto.AuthorizeOAuthRspBody], error) {
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	provider := enum.OAuthProvider(req.Body.Provider)
	if !provider.IsValid() {
		return nil, apiutil.NewHumaBizError(ctx, ierr.Newf(ierr.ErrValidation, "unsupported oauth provider %q", req.Body.Provider), ierr.ErrValidation.BizError())
	}

	result, err := h.oauth.Handle(ctx, port.AuthorizeOAuthCommand{UserID: userID, Provider: provider})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Authorize oauth failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.AuthorizeOAuthRspBody{AuthorizeURL: result.AuthorizeURL, State: result.State}, nil)
}

// HandleExchangeOAuth 用授权码完成订阅账号绑定
func (h *endpointHandler) HandleExchangeOAuth(ctx context.Context, req *dto.ExchangeOAuthReq) (*dto.HTTPResponse[*dto.ExchangeOAuthRspBody], error) {
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	provider := enum.OAuthProvider(req.Body.Provider)
	if !provider.IsValid() {
		return nil, apiutil.NewHumaBizError(ctx, ierr.Newf(ierr.ErrValidation, "unsupported oauth provider %q", req.Body.Provider), ierr.ErrValidation.BizError())
	}

	isAdmin := util.CtxValuePermission(ctx) == enum.PermissionAdmin
	ownerUserID := lo.Ternary(isAdmin && req.Body.OwnerUserID != nil, lo.FromPtr(req.Body.OwnerUserID), userID)
	result, err := h.exchange.Handle(ctx, port.ExchangeOAuthCommand{
		OperatorUserID: userID,
		OwnerUserID:    ownerUserID,
		Name:           req.Body.Name,
		Provider:       provider,
		CodeInput:      req.Body.Code,
		State:          lo.FromPtr(req.Body.State),
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Exchange oauth failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	logger.WithCtx(ctx).Info("[EndpointHandler] OAuth endpoint bound", zap.Uint("endpointID", result.EndpointID), zap.Uint("ownerUserID", ownerUserID))
	return apiutil.WrapHTTPResponse(&dto.ExchangeOAuthRspBody{EndpointID: result.EndpointID}, nil)
}

// HandleReauthOAuth 重新授权既有订阅端点
func (h *endpointHandler) HandleReauthOAuth(ctx context.Context, req *dto.ReauthOAuthReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	err = h.reauth.Handle(ctx, port.ReauthOAuthCommand{
		OperatorUserID: util.CtxValueUint(ctx, constant.CtxKeyUserID),
		ScopeUserID:    scope,
		EndpointID:     req.ID,
		IsAdmin:        util.CtxValuePermission(ctx) == enum.PermissionAdmin,
		CodeInput:      req.Body.Code,
		State:          lo.FromPtr(req.Body.State),
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[EndpointHandler] Reauth oauth failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.EmptyRsp{}, nil)
}

// HandleGetOAuthQuota 查询订阅端点凭据状态与额度快照
func (h *endpointHandler) HandleGetOAuthQuota(ctx context.Context, req *dto.GetOAuthQuotaReq) (*dto.HTTPResponse[*dto.GetOAuthQuotaRspBody], error) {
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	view, err := h.quota.Handle(ctx, port.GetOAuthQuotaQuery{ScopeUserID: scope, EndpointID: req.ID})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.GetOAuthQuotaRspBody{
		Status:         view.Status,
		PlanType:       view.PlanType,
		LastError:      view.LastError,
		Quota:          toOAuthQuotaDTO(view.Quota),
		QuotaUpdatedAt: view.QuotaUpdatedAt,
	}, nil)
}

// toOAuthQuotaDTO 应用层额度视图 → DTO（逐字段映射；两侧均为扁平结构，无 domain 依赖）
func toOAuthQuotaDTO(view *port.OAuthQuotaSnapshotView) *dto.OAuthQuotaSnapshot {
	if view == nil {
		return nil
	}
	return &dto.OAuthQuotaSnapshot{
		Windows: lo.Map(view.Windows, func(w port.OAuthQuotaWindowView, _ int) dto.OAuthQuotaWindow {
			return dto.OAuthQuotaWindow{Key: w.Key, UsedPercent: w.UsedPercent, ResetsAt: w.ResetsAt, WindowMinutes: w.WindowMinutes}
		}),
		PlanType:   view.PlanType,
		CapturedAt: view.CapturedAt,
	}
}

// HandleProbeOAuthQuota 手动触发一次额度探测（同步；服务端处理超时见 constant.OAuthProbeTimeout）
func (h *endpointHandler) HandleProbeOAuthQuota(ctx context.Context, req *dto.ProbeOAuthQuotaReq) (*dto.HTTPResponse[*dto.GetOAuthQuotaRspBody], error) {
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}
	probeCtx, cancel := context.WithTimeout(ctx, constant.OAuthProbeTimeout)
	defer cancel()

	view, err := h.probe.Handle(probeCtx, port.ProbeOAuthQuotaCommand{ScopeUserID: scope, EndpointID: req.ID})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.GetOAuthQuotaRspBody{
		Status:         view.Status,
		PlanType:       view.PlanType,
		LastError:      view.LastError,
		Quota:          toOAuthQuotaDTO(view.Quota),
		QuotaUpdatedAt: view.QuotaUpdatedAt,
	}, nil)
}

// scopeFor 多租户隔离 scope 计算：admin 返回 nil（不过滤），其余用户限定自身。
// handler 层共享（endpoint / model / upstream 统一使用本实现）。
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
