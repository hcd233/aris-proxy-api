package usecase

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	convvo "github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
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
		Metadata:     util.ExtractAnthropicMetadata(req.Body.Metadata),
	}); err != nil {
		log.Error("[AnthropicUseCase] Failed to submit message store task", zap.Error(err))
	}
}

func (u *anthropicUseCase) convertAnthropicRequestMessages(ctx context.Context, req *dto.AnthropicCreateMessageRequest, assistantMsg *dto.AnthropicMessage) ([]*convvo.UnifiedMessage, []*convvo.UnifiedTool, int, int, error) {
	log := logger.WithCtx(ctx)
	unifiedMessages := make([]*convvo.UnifiedMessage, 0, len(req.Body.Messages)+1)
	for _, msg := range req.Body.Messages {
		um, err := dto.FromAnthropicMessage(msg)
		if err != nil {
			log.Error("[AnthropicUseCase] Failed to convert anthropic message", zap.Error(err))
			return nil, nil, 0, 0, err
		}
		unifiedMessages = append(unifiedMessages, um)
	}

	aiMsg, err := dto.FromAnthropicResponse(assistantMsg)
	if err != nil {
		log.Error("[AnthropicUseCase] Failed to convert anthropic response", zap.Error(err))
		return nil, nil, 0, 0, err
	}
	unifiedMessages = append(unifiedMessages, aiMsg)

	unifiedTools := make([]*convvo.UnifiedTool, 0, len(req.Body.Tools))
	for _, tool := range req.Body.Tools {
		unifiedTools = append(unifiedTools, dto.FromAnthropicTool(tool))
	}

	var inputTokens, outputTokens int
	if assistantMsg.Usage != nil {
		inputTokens = assistantMsg.Usage.InputTokens
		outputTokens = assistantMsg.Usage.OutputTokens
	}

	return unifiedMessages, unifiedTools, inputTokens, outputTokens, nil
}
