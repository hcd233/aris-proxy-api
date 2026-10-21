package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/oauthquota"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type oauthProbeHandler struct {
	endpointRepo llmproxy.EndpointRepository
	credRepo     llmproxy.OAuthCredentialRepository
	tokens       service.OAuthTokenProvider
	providers    map[enum.OAuthProvider]service.OAuthProviderClient
}

// NewOAuthProbeHandler 构造订阅端点额度探测处理器
func NewOAuthProbeHandler(
	endpointRepo llmproxy.EndpointRepository,
	credRepo llmproxy.OAuthCredentialRepository,
	tokens service.OAuthTokenProvider,
	providers map[enum.OAuthProvider]service.OAuthProviderClient,
) port.ProbeOAuthQuotaHandler {
	return &oauthProbeHandler{endpointRepo: endpointRepo, credRepo: credRepo, tokens: tokens, providers: providers}
}

// Handle 同步探测一次额度并写回快照；返回探测后的最新视图。
//
// 探测与落库走 oauthquota.Probe（与 cron 维护共用同一实现，避免两处漂移）；
// 落库只更新额度列——整行写回会把并发刷新出的新 token 覆盖成快照里的旧值。
// 取 token 走 token provider（可能触发刷新）：凭据失效时按可切换错误语义上抛。
func (h *oauthProbeHandler) Handle(ctx context.Context, cmd port.ProbeOAuthQuotaCommand) (*port.OAuthQuotaView, error) {
	ep, err := h.endpointRepo.FindByID(ctx, cmd.EndpointID, cmd.ScopeUserID)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "endpoint not found")
	}
	if !ep.AuthType().IsOAuth() {
		return nil, ierr.New(ierr.ErrValidation, "endpoint is not an oauth subscription endpoint")
	}

	// provider 需先读凭据（探测内部按 provider 选客户端）
	cred, err := h.credRepo.FindByEndpointID(ctx, cmd.EndpointID)
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "oauth credential not found")
	}

	if _, probeErr := oauthquota.Probe(ctx, cmd.EndpointID, cred.Provider(), h.tokens, h.providers, h.credRepo); probeErr != nil {
		logger.WithCtx(ctx).Warn("[OAuthProbe] Quota probe failed",
			zap.Uint("endpointID", cmd.EndpointID), zap.Error(probeErr))
		return nil, probeErr
	}

	// 重新读取：额度与最近错误以落库后的最新值为准
	latest, err := h.credRepo.FindByEndpointID(ctx, cmd.EndpointID)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "oauth credential not found")
	}
	return port.NewOAuthQuotaView(latest), nil
}
