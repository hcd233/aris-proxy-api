package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// CreateEndpointHandler 创建命令处理器
type CreateEndpointHandler interface {
	Handle(ctx context.Context, cmd port.CreateEndpointCommand) (*port.CreateEndpointResult, error)
}

type createEndpointHandler struct {
	repo llmproxy.EndpointRepository
}

// NewCreateEndpointHandler 构造创建命令处理器
func NewCreateEndpointHandler(repo llmproxy.EndpointRepository) CreateEndpointHandler {
	return &createEndpointHandler{repo: repo}
}

// Handle 执行创建命令
func (h *createEndpointHandler) Handle(ctx context.Context, cmd port.CreateEndpointCommand) (*port.CreateEndpointResult, error) {
	log := logger.WithCtx(ctx)

	ep, err := aggregate.CreateEndpoint(0, cmd.Name, cmd.OpenaiBaseURL, cmd.AnthropicBaseURL, cmd.APIKey, cmd.SupportOpenAIChatCompletion, cmd.SupportOpenAIResponse, cmd.SupportAnthropicMessage)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "validate endpoint")
	}

	id, err := h.repo.Create(ctx, ep)
	if err != nil {
		log.Error("[EndpointCommand] Create endpoint failed", zap.Error(err))
		return nil, err
	}

	log.Info("[EndpointCommand] Create endpoint success", zap.Uint("id", id))
	return &port.CreateEndpointResult{EndpointID: id}, nil
}
