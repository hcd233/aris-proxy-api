package command

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type oauthExchangeHandler struct {
	flowStore    service.OAuthFlowStore
	clients      map[enum.OAuthProvider]service.OAuthProviderClient
	endpointRepo llmproxy.OAuthEndpointRepository
}

// NewOAuthExchangeHandler 构造订阅账号绑定处理器
func NewOAuthExchangeHandler(flowStore service.OAuthFlowStore, clients map[enum.OAuthProvider]service.OAuthProviderClient, endpointRepo llmproxy.OAuthEndpointRepository) port.ExchangeOAuthHandler {
	return &oauthExchangeHandler{flowStore: flowStore, clients: clients, endpointRepo: endpointRepo}
}

// Handle 用授权码完成订阅账号绑定：
// 校验授权流（一次性 + 归属）→ 换 token → 单事务创建 OAuth 端点与凭据。
//
// 任一环节失败都不产生任何行；flow 已被 Take（即取即删），用户需重新发起授权。
func (h *oauthExchangeHandler) Handle(ctx context.Context, cmd port.ExchangeOAuthCommand) (*port.ExchangeOAuthResult, error) {
	log := logger.WithCtx(ctx)

	if cmd.OwnerUserID == 0 || cmd.OperatorUserID == 0 {
		return nil, ierr.New(ierr.ErrValidation, "oauth exchange user id is required")
	}
	if !cmd.Provider.IsValid() {
		return nil, ierr.Newf(ierr.ErrValidation, "unsupported oauth provider %q", string(cmd.Provider))
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint name cannot be empty")
	}

	code, state, err := commonutil.ParseOAuthCode(cmd.CodeInput, cmd.State)
	if err != nil {
		return nil, err
	}

	flow, ok, err := h.flowStore.Take(ctx, state)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ierr.New(ierr.ErrValidation, "oauth flow not found or expired, please restart authorization")
	}
	if flow.UserID != cmd.OperatorUserID {
		return nil, ierr.New(ierr.ErrNoPermission, "oauth flow belongs to another user")
	}
	if flow.Provider != cmd.Provider {
		return nil, ierr.New(ierr.ErrValidation, "oauth flow provider mismatch")
	}

	client := h.clients[cmd.Provider]
	if client == nil {
		return nil, ierr.Newf(ierr.ErrInternal, "oauth provider client %q not wired", string(cmd.Provider))
	}
	tokenSet, err := client.ExchangeCode(ctx, code, flow.Verifier, state)
	if err != nil {
		log.Warn("[OAuthExchange] Exchange authorization code failed",
			zap.String("provider", string(cmd.Provider)), zap.Error(err))
		return nil, err
	}

	ep, cred, err := buildOAuthEndpointAndCredential(cmd.Name, cmd.Provider, tokenSet)
	if err != nil {
		return nil, err
	}
	endpointID, err := h.endpointRepo.CreateWithOAuthCredential(ctx, ep, cred, cmd.OwnerUserID)
	if err != nil {
		log.Error("[OAuthExchange] Create oauth endpoint failed", zap.Error(err))
		return nil, err
	}
	log.Info("[OAuthExchange] OAuth endpoint bound",
		zap.Uint("endpointID", endpointID), zap.String("provider", string(cmd.Provider)), zap.Uint("ownerUserID", cmd.OwnerUserID))
	return &port.ExchangeOAuthResult{EndpointID: endpointID}, nil
}

// buildOAuthEndpointAndCredential 构造 OAuth 端点与凭据。
//
// 连接字段是服务端固定值（协议事实）：OAuth 端点只开启本 provider 的协议能力，
// baseURL 指向官方端点——用户不可配，避免把订阅凭据发到任意主机。
func buildOAuthEndpointAndCredential(name string, provider enum.OAuthProvider, tokenSet *service.OAuthTokenSet) (*aggregate.Endpoint, *aggregate.OAuthCredential, error) {
	var (
		ep       *aggregate.Endpoint
		authType enum.EndpointAuthType
		err      error
	)
	switch provider {
	case enum.OAuthProviderAnthropic:
		authType = enum.EndpointAuthOAuthAnthropic
		ep, err = aggregate.CreateEndpoint(0, name, "", constant.OAuthClaudeUpstreamBaseURL, "", false, false, true, false, authType)
	case enum.OAuthProviderOpenAI:
		authType = enum.EndpointAuthOAuthOpenAI
		ep, err = aggregate.CreateEndpoint(0, name, constant.OAuthOpenAIUpstreamBaseURL, "", "", false, true, false, false, authType)
	default:
		return nil, nil, ierr.Newf(ierr.ErrValidation, "unsupported oauth provider %q", string(provider))
	}
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrValidation, err, "validate oauth endpoint")
	}

	cred, err := aggregate.CreateOAuthCredential(0, provider, tokenSet.AccessToken, tokenSet.RefreshToken, tokenSet.ExpiresAt, tokenSet.AccountRef, tokenSet.AccountEmail, tokenSet.PlanType)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrValidation, err, "validate oauth credential")
	}
	return ep, cred, nil
}
