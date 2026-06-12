package command

import (
	"context"

	"github.com/samber/mo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// UpdateEndpointHandler 更新命令处理器
type UpdateEndpointHandler interface {
	Handle(ctx context.Context, cmd port.UpdateEndpointCommand) error
}

type updateEndpointHandler struct {
	repo llmproxy.EndpointRepository
}

// NewUpdateEndpointHandler 构造更新命令处理器
func NewUpdateEndpointHandler(repo llmproxy.EndpointRepository) UpdateEndpointHandler {
	return &updateEndpointHandler{repo: repo}
}

func ptrToOption[T any](ptr *T) mo.Option[T] {
	if ptr == nil {
		return mo.None[T]()
	}
	return mo.Some(*ptr)
}

// Handle 执行更新命令
func (h *updateEndpointHandler) Handle(ctx context.Context, cmd port.UpdateEndpointCommand) error {
	log := logger.WithCtx(ctx)

	epResult := h.repo.FindByID(ctx, cmd.EndpointID)
	if epResult.IsError() {
		log.Error("[EndpointCommand] Find endpoint for update failed", zap.Error(epResult.Error()))
		return epResult.Error()
	}
	ep := epResult.MustGet()

	ep.Update(
		ptrToOption(cmd.Name),
		ptrToOption(cmd.OpenaiBaseURL),
		ptrToOption(cmd.AnthropicBaseURL),
		ptrToOption(cmd.APIKey),
		ptrToOption(cmd.SupportOpenAIChatCompletion),
		ptrToOption(cmd.SupportOpenAIResponse),
		ptrToOption(cmd.SupportAnthropicMessage),
	)

	if err := h.repo.Update(ctx, ep); err != nil {
		log.Error("[EndpointCommand] Update endpoint failed", zap.Error(err))
		return err
	}

	log.Info("[EndpointCommand] Update endpoint success", zap.Uint("id", cmd.EndpointID))
	return nil
}
