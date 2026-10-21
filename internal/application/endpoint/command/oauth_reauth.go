package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type oauthReauthHandler struct {
	flowStore    service.OAuthFlowStore
	clients      map[enum.OAuthProvider]service.OAuthProviderClient
	endpointRepo llmproxy.EndpointRepository
	credRepo     llmproxy.OAuthCredentialRepository
	accessCache  service.OAuthAccessCache
}

// NewOAuthReauthHandler 构造订阅账号重新授权处理器
func NewOAuthReauthHandler(flowStore service.OAuthFlowStore, clients map[enum.OAuthProvider]service.OAuthProviderClient, endpointRepo llmproxy.EndpointRepository, credRepo llmproxy.OAuthCredentialRepository, accessCache service.OAuthAccessCache) port.ReauthOAuthHandler {
	return &oauthReauthHandler{flowStore: flowStore, clients: clients, endpointRepo: endpointRepo, credRepo: credRepo, accessCache: accessCache}
}

// Handle 重新授权既有订阅端点：更新凭据并恢复 active。
//
// 与绑定（exchange）共用 flow 校验与 PKCE 兑换；不改变端点的名称/归属/连接字段。
func (h *oauthReauthHandler) Handle(ctx context.Context, cmd port.ReauthOAuthCommand) error {
	log := logger.WithCtx(ctx)

	if cmd.OperatorUserID == 0 || cmd.EndpointID == 0 {
		return ierr.New(ierr.ErrValidation, "oauth reauth parameters are incomplete")
	}

	ep, err := h.endpointRepo.FindByID(ctx, cmd.EndpointID, cmd.ScopeUserID)
	if err != nil {
		return err
	}
	if ep == nil {
		return ierr.New(ierr.ErrDataNotExists, "endpoint not found")
	}
	// 降级保护：只有 admin 能重新授权公开端点（会改写所有用户共用的上游凭据）。
	if ep.Visibility().IsPublic() && !cmd.IsAdmin {
		return ierr.New(ierr.ErrUnauthorized, "only admin can reauthorize a public endpoint")
	}
	provider, ok := providerOfAuthType(ep.AuthType())
	if !ok {
		return ierr.New(ierr.ErrValidation, "endpoint is not an oauth subscription endpoint")
	}

	code, state, err := commonutil.ParseOAuthCode(cmd.CodeInput, cmd.State)
	if err != nil {
		return err
	}

	flow, ok, err := h.flowStore.Take(ctx, state)
	if err != nil {
		return err
	}
	if !ok {
		return ierr.New(ierr.ErrValidation, "oauth flow not found or expired, please restart authorization")
	}
	if flow.UserID != cmd.OperatorUserID {
		return ierr.New(ierr.ErrNoPermission, "oauth flow belongs to another user")
	}
	if flow.Provider != provider {
		return ierr.New(ierr.ErrValidation, "oauth flow provider mismatch")
	}

	client := h.clients[provider]
	if client == nil {
		return ierr.Newf(ierr.ErrInternal, "oauth provider client %q not wired", string(provider))
	}
	tokenSet, err := client.ExchangeCode(ctx, code, flow.Verifier, state)
	if err != nil {
		log.Warn("[OAuthReauth] Exchange authorization code failed",
			zap.String("provider", string(provider)), zap.Error(err))
		return err
	}

	cred, err := h.credRepo.FindByEndpointID(ctx, cmd.EndpointID)
	if err != nil {
		return err
	}
	if cred == nil {
		// 缺凭据行（历史异常态）：补建，避免用户永远无法恢复该端点
		cred, err = createCredentialForEndpoint(cmd.EndpointID, provider, tokenSet)
		if err != nil {
			return err
		}
	} else {
		cred.ApplyTokens(tokenSet.AccessToken, tokenSet.RefreshToken, tokenSet.ExpiresAt)
		// 账号信息随新授权一起更新：换用另一个 Codex 账号重新授权后，
		// 若只换 token，chatgpt-account-id 头仍指向旧账号，上游会按旧账号计费/限流。
		cred.ApplyAccount(tokenSet.AccountRef, tokenSet.AccountEmail, tokenSet.PlanType)
	}
	if err := h.credRepo.Save(ctx, cred); err != nil {
		log.Error("[OAuthReauth] Save credential failed", zap.Error(err))
		return err
	}
	// 旧 access token 可能已被上游吊销：必须失效缓存，否则缓存命中会让
	// "重新授权恢复"在旧 token 的剩余 TTL 内继续使用死 token（C1 修复）
	h.accessCache.Delete(ctx, cmd.EndpointID)
	log.Info("[OAuthReauth] OAuth endpoint reauthorized",
		zap.Uint("endpointID", cmd.EndpointID), zap.String("provider", string(provider)))
	return nil
}

// providerOfAuthType 从端点认证类型推断 OAuth provider
func providerOfAuthType(authType enum.EndpointAuthType) (enum.OAuthProvider, bool) {
	switch authType {
	case enum.EndpointAuthOAuthAnthropic:
		return enum.OAuthProviderAnthropic, true
	case enum.EndpointAuthOAuthOpenAI:
		return enum.OAuthProviderOpenAI, true
	default:
		return "", false
	}
}

// createCredentialForEndpoint 为既有端点补建凭据（reauth 的历史异常态恢复路径）
func createCredentialForEndpoint(endpointID uint, provider enum.OAuthProvider, tokenSet *service.OAuthTokenSet) (*aggregate.OAuthCredential, error) {
	cred, err := aggregate.CreateOAuthCredential(endpointID, provider, tokenSet.AccessToken, tokenSet.RefreshToken, tokenSet.ExpiresAt, tokenSet.AccountRef, tokenSet.AccountEmail, tokenSet.PlanType)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "validate oauth credential")
	}
	return cred, nil
}
