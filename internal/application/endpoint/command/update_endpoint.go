package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type updateEndpointHandler struct {
	repo llmproxy.EndpointRepository
}

// NewUpdateEndpointHandler 构造更新命令处理器
func NewUpdateEndpointHandler(repo llmproxy.EndpointRepository) port.UpdateEndpointHandler {
	return &updateEndpointHandler{repo: repo}
}

// Handle 执行更新命令
func (h *updateEndpointHandler) Handle(ctx context.Context, cmd port.UpdateEndpointCommand) error {
	log := logger.WithCtx(ctx)

	// baseURL SSRF 校验放应用层且先于仓储查询：既避免非法 URL 触达 repo，
	// 也避免"URL 非法"与"端点不存在"的报错差异被用来探测端点 ID。
	// nil/空串表示不改或清空，不产生服务端请求目标，交由聚合根既有规则。
	for _, raw := range []*string{cmd.OpenaiBaseURL, cmd.AnthropicBaseURL} {
		if raw == nil || *raw == "" {
			continue
		}
		if err := util.ValidateEndpointBaseURL(*raw); err != nil {
			return err
		}
	}

	ep, err := h.repo.FindByID(ctx, cmd.EndpointID, cmd.ScopeUserID)
	if err != nil {
		log.Error("[EndpointCommand] Find endpoint for update failed", zap.Error(err))
		return err
	}
	if ep == nil {
		return ierr.New(ierr.ErrDataNotExists, "endpoint not found")
	}

	ep.Update(cmd.Name, cmd.OpenaiBaseURL, cmd.AnthropicBaseURL, cmd.APIKey, cmd.SupportOpenAIChatCompletion, cmd.SupportOpenAIResponse, cmd.SupportAnthropicMessage)

	if err := h.repo.Update(ctx, ep); err != nil {
		log.Error("[EndpointCommand] Update endpoint failed", zap.Error(err))
		return err
	}

	log.Info("[EndpointCommand] Update endpoint success", zap.Uint("id", cmd.EndpointID))
	return nil
}
