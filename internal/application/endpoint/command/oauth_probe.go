package command

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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
// 取 token 走 token provider（可能触发刷新）：凭据失效时按可切换错误语义上抛。
func (h *oauthProbeHandler) Handle(ctx context.Context, cmd port.ProbeOAuthQuotaCommand) (*port.OAuthQuotaView, error) {
	log := logger.WithCtx(ctx)

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

	access, err := h.tokens.GetAccessToken(ctx, cmd.EndpointID)
	if err != nil {
		return nil, err
	}

	cred, err := h.credRepo.FindByEndpointID(ctx, cmd.EndpointID)
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "oauth credential not found")
	}
	client := h.providers[cred.Provider()]
	if client == nil {
		return nil, ierr.Newf(ierr.ErrInternal, "oauth provider client %q not wired", string(cred.Provider()))
	}

	snapshot, err := client.ProbeQuota(ctx, access.Token, access.AccountRef)
	if err != nil {
		log.Warn("[OAuthProbe] Quota probe failed", zap.Uint("endpointID", cmd.EndpointID), zap.Error(err))
		// 写 last_error 供管理端展示（不改状态机，区别于刷新失败的 MarkReauth）
		cred.SetLastError(fmt.Sprintf(constant.OAuthReasonQuotaProbeFailed+"%v", err))
		if saveErr := h.credRepo.Save(ctx, cred); saveErr != nil {
			log.Error("[OAuthProbe] Persist probe error failed", zap.Error(saveErr))
		}
		return nil, err
	}
	now := time.Now()
	cred.SetQuota(*snapshot, now)
	if err := h.credRepo.Save(ctx, cred); err != nil {
		log.Error("[OAuthProbe] Persist quota snapshot failed", zap.Error(err))
		return nil, err
	}

	windows := make([]port.OAuthQuotaWindowView, 0, len(snapshot.Windows))
	for _, w := range snapshot.Windows {
		windows = append(windows, port.OAuthQuotaWindowView{
			Key:           w.Key,
			UsedPercent:   w.UsedPercent,
			ResetsAt:      w.ResetsAt,
			WindowMinutes: w.WindowMinutes,
		})
	}
	updatedAt := cred.QuotaUpdatedAt()
	return &port.OAuthQuotaView{
		Status:         string(cred.Status()),
		PlanType:       cred.PlanType(),
		LastError:      cred.LastError(),
		Quota:          &port.OAuthQuotaSnapshotView{Windows: windows, PlanType: snapshot.PlanType, CapturedAt: snapshot.CapturedAt},
		QuotaUpdatedAt: &updatedAt,
	}, nil
}
