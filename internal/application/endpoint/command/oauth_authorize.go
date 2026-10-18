package command

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

type oauthAuthorizeHandler struct {
	flowStore service.OAuthFlowStore
	clients   map[enum.OAuthProvider]service.OAuthProviderClient
}

// NewOAuthAuthorizeHandler 构造订阅账号授权发起处理器
func NewOAuthAuthorizeHandler(flowStore service.OAuthFlowStore, clients map[enum.OAuthProvider]service.OAuthProviderClient) port.AuthorizeOAuthHandler {
	return &oauthAuthorizeHandler{flowStore: flowStore, clients: clients}
}

// Handle 生成 PKCE 与 state、落授权流记录并返回授权链接。
//
// verifier 只进 flow 记录（服务端），授权链接只携带 challenge——兑换阶段
// 用 flow 中 verifier 完成 PKCE 校验，任何一方缺失都无法换到 token。
func (h *oauthAuthorizeHandler) Handle(ctx context.Context, cmd port.AuthorizeOAuthCommand) (*port.AuthorizeOAuthResult, error) {
	if cmd.UserID == 0 {
		return nil, ierr.New(ierr.ErrValidation, "oauth authorize user id is required")
	}
	if !cmd.Provider.IsValid() {
		return nil, ierr.Newf(ierr.ErrValidation, "unsupported oauth provider %q", string(cmd.Provider))
	}
	client := h.clients[cmd.Provider]
	if client == nil {
		return nil, ierr.Newf(ierr.ErrInternal, "oauth provider client %q not wired", string(cmd.Provider))
	}

	verifier, challenge, err := commonutil.GeneratePKCE(cmd.Provider)
	if err != nil {
		return nil, err
	}
	state, err := commonutil.GenerateOAuthState()
	if err != nil {
		return nil, err
	}
	flow := vo.OAuthFlow{UserID: cmd.UserID, Provider: cmd.Provider, Verifier: verifier, CreatedAt: time.Now()}
	if err := h.flowStore.Put(ctx, state, flow); err != nil {
		return nil, err
	}
	return &port.AuthorizeOAuthResult{
		AuthorizeURL: client.AuthorizeURL(challenge, state),
		State:        state,
	}, nil
}
