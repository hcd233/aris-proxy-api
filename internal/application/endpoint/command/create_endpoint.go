package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type createEndpointHandler struct {
	repo llmproxy.EndpointRepository
}

// NewCreateEndpointHandler 构造创建命令处理器
func NewCreateEndpointHandler(repo llmproxy.EndpointRepository) port.CreateEndpointHandler {
	return &createEndpointHandler{repo: repo}
}

// Handle 执行创建命令
//
// cmd.OwnerUserID 已由 handler 层计算好归属（普通用户=自身，admin 可指定），直接写入。
func (h *createEndpointHandler) Handle(ctx context.Context, cmd port.CreateEndpointCommand) (*port.CreateEndpointResult, error) {
	log := logger.WithCtx(ctx)

	if cmd.OwnerUserID == 0 {
		return nil, ierr.New(ierr.ErrValidation, "endpoint owner user id is required")
	}

	// baseURL SSRF 校验放应用层（domain 不做网络 IO）；空值表示该协议未配置，交给聚合根非空规则。
	for _, raw := range []string{cmd.OpenaiBaseURL, cmd.AnthropicBaseURL} {
		if raw == "" {
			continue
		}
		if err := util.ValidateEndpointBaseURL(raw); err != nil {
			return nil, err
		}
	}

	ep, err := aggregate.CreateEndpoint(0, cmd.Name, cmd.OpenaiBaseURL, cmd.AnthropicBaseURL, cmd.APIKey, cmd.SupportOpenAIChatCompletion, cmd.SupportOpenAIResponse, cmd.SupportAnthropicMessage, cmd.SupportOpenAIDecision)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "validate endpoint")
	}

	id, err := h.repo.Create(ctx, ep, cmd.OwnerUserID)
	if err != nil {
		log.Error("[EndpointCommand] Create endpoint failed", zap.Error(err))
		return nil, err
	}

	log.Info("[EndpointCommand] Create endpoint success",
		zap.Uint("id", id), zap.Uint("ownerUserID", cmd.OwnerUserID))
	return &port.CreateEndpointResult{EndpointID: id}, nil
}
