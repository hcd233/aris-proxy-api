package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// UpdateEndpointCommand 更新 Endpoint 命令
type UpdateEndpointCommand struct {
	EndpointID                  uint
	Name                        *string
	OpenaiBaseURL               *string
	AnthropicBaseURL            *string
	APIKey                      *string
	SupportOpenAIChatCompletion *bool
	SupportOpenAIResponse       *bool
	SupportAnthropicMessage     *bool
}

// UpdateEndpointHandler 更新命令处理器
type UpdateEndpointHandler interface {
	Handle(ctx context.Context, cmd UpdateEndpointCommand) error
}

type updateEndpointHandler struct {
	repo llmproxy.EndpointRepository
}

// NewUpdateEndpointHandler 构造更新命令处理器
func NewUpdateEndpointHandler(repo llmproxy.EndpointRepository) UpdateEndpointHandler {
	return &updateEndpointHandler{repo: repo}
}

// Handle 执行更新命令
func (h *updateEndpointHandler) Handle(ctx context.Context, cmd UpdateEndpointCommand) error {
	log := logger.WithCtx(ctx)

	ep, err := h.repo.FindByID(ctx, cmd.EndpointID)
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
