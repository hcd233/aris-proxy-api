package usecase

import (
	"context"

	"go.uber.org/zap"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	convvo "github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func (u *anthropicUseCase) storeAnthropicFromMsg(ctx context.Context, req *dto.AnthropicCreateMessageRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}
	u.storeAnthropicMessages(ctx, req, msg, upstreamModel)
}

func (u *anthropicUseCase) storeAnthropicMessages(ctx context.Context, req *dto.AnthropicCreateMessageRequest, assistantMsg *dto.AnthropicMessage, upstreamModel string) {
	log := logger.WithCtx(ctx)
	unifiedMessages, unifiedTools, inputTokens, outputTokens, err := u.convertAnthropicRequestMessages(ctx, req, assistantMsg)
	if err != nil {
		return
	}

	if err := u.taskSubmitter.SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     unifiedMessages,
		Tools:        unifiedTools,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Metadata:     proxyutil.ExtractAnthropicMetadata(req.Body.Metadata),
	}); err != nil {
		log.Error("[AnthropicUseCase] Failed to submit message store task", zap.Error(err))
	}
}

func (u *anthropicUseCase) convertAnthropicRequestMessages(ctx context.Context, req *dto.AnthropicCreateMessageRequest, assistantMsg *dto.AnthropicMessage) (messages []*convvo.UnifiedMessage, tools []*convvo.UnifiedTool, inputTokens, outputTokens int, err error) {
	log := logger.WithCtx(ctx)
	messages = make([]*convvo.UnifiedMessage, 0, len(req.Body.Messages)+1)
	for _, msg := range req.Body.Messages {
		um, convErr := dto.FromAnthropicMessage(msg)
		if convErr != nil {
			log.Error("[AnthropicUseCase] Failed to convert anthropic message", zap.Error(convErr))
			return nil, nil, 0, 0, convErr
		}
		messages = append(messages, um)
	}

	aiMsg, convErr := dto.FromAnthropicResponse(assistantMsg)
	if convErr != nil {
		log.Error("[AnthropicUseCase] Failed to convert anthropic response", zap.Error(convErr))
		return nil, nil, 0, 0, convErr
	}
	messages = append(messages, aiMsg)

	tools = make([]*convvo.UnifiedTool, 0, len(req.Body.Tools))
	for _, tool := range req.Body.Tools {
		tools = append(tools, dto.FromAnthropicTool(tool))
	}

	if assistantMsg.Usage != nil {
		inputTokens = assistantMsg.Usage.InputTokens
		outputTokens = assistantMsg.Usage.OutputTokens
	}

	return messages, tools, inputTokens, outputTokens, nil
}
