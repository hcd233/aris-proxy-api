// Package usecase LLMProxy 域用例层 — 消息存储辅助函数
//
//	@author centonhuang
//	@update 2026-04-28 20:00:00
package usecase

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// storeOpenAIChatFromCompletion 原生 OpenAI 响应 → 消息存储
func (u *openAIUseCase) storeOpenAIChatFromCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest, completion *dto.OpenAIChatCompletion, proxyErr error, upstreamModel string) {
	if proxyErr != nil || completion == nil || len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return
	}
	u.storeOpenAIChatMessages(ctx, req, completion.Choices[0].Message, upstreamModel, completion.Usage)
}

// storeOpenAIChatFromAnthropicMsg Anthropic 响应先转 OpenAI 再落盘
func (u *openAIUseCase) storeOpenAIChatFromAnthropicMsg(ctx context.Context, req *dto.OpenAIChatCompletionRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	log := logger.WithCtx(ctx)
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}
	conv := converter.AnthropicProtocolConverter{}
	completion, err := conv.ToOpenAIResponse(msg)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert for storage", zap.Error(err))
		return
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
		return
	}
	u.storeOpenAIChatMessages(ctx, req, completion.Choices[0].Message, upstreamModel, completion.Usage)
}

// storeOpenAIChatMessages ChatCompletion 存储基元：req.Messages + assistantMsg → UnifiedMessage 列表
func (u *openAIUseCase) storeOpenAIChatMessages(ctx context.Context, req *dto.OpenAIChatCompletionRequest, assistantMsg *dto.OpenAIChatCompletionMessageParam, upstreamModel string, usage *dto.OpenAICompletionUsage) {
	log := logger.WithCtx(ctx)
	unifiedMessages, unifiedTools, err := u.convertRequestMessages(ctx, req)
	if err != nil {
		return
	}

	aiMsg, err := dto.FromOpenAIMessage(assistantMsg)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert ai response message", zap.Error(err))
		return
	}
	unifiedMessages = append(unifiedMessages, aiMsg)

	var inputTokens, outputTokens int
	if usage != nil {
		inputTokens = usage.PromptTokens
		outputTokens = usage.CompletionTokens
	}

	if err := u.taskSubmitter.SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     unifiedMessages,
		Tools:        unifiedTools,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Metadata:     req.Body.Metadata,
	}); err != nil {
		log.Error("[OpenAIUseCase] Failed to submit message store task", zap.Error(err))
	}
}

// convertRequestMessages 将 OpenAI 请求消息和工具转换为统一格式
//
//	@receiver u *openAIUseCase
//	@param req *dto.OpenAIChatCompletionRequest
//
// @return vo.UnifiedMessage list
// @return vo.UnifiedTool list
// @return error
// @author centonhuang
// @update 2026-04-26 12:00:00
func (u *openAIUseCase) convertRequestMessages(ctx context.Context, req *dto.OpenAIChatCompletionRequest) ([]*vo.UnifiedMessage, []*vo.UnifiedTool, error) {
	log := logger.WithCtx(ctx)
	unifiedMessages := make([]*vo.UnifiedMessage, 0, len(req.Body.Messages))
	for _, msg := range req.Body.Messages {
		um, err := dto.FromOpenAIMessage(msg)
		if err != nil {
			log.Error("[OpenAIUseCase] Failed to convert openai message", zap.Error(err))
			return nil, nil, err
		}
		unifiedMessages = append(unifiedMessages, um)
	}
	unifiedTools := lo.Map(req.Body.Tools, func(tool dto.OpenAIChatCompletionTool, _ int) *vo.UnifiedTool {
		return dto.FromOpenAITool(&tool)
	})
	return unifiedMessages, unifiedTools, nil
}

// ==================== Store Helpers: Response API 路径 ====================

// storeResponseFromRsp Response API 原生响应 → 消息存储
func (u *openAIUseCase) storeResponseFromRsp(ctx context.Context, req *dto.OpenAICreateResponseRequest, rsp *dto.OpenAICreateResponseRsp, proxyErr error, upstreamModel string) {
	if proxyErr != nil || rsp == nil {
		return
	}
	switch rsp.Status {
	case "",
		enum.ResponseStatusCompleted,
		enum.ResponseStatusIncomplete:
		// persistable
	default:
		return
	}

	unifiedMessages, ok := buildResponseRequestUnifiedMessages(ctx, req)
	if !ok {
		return
	}

	outputMsgs, ok := convertResponseOutput(rsp)
	if !ok {
		return
	}
	unifiedMessages = append(unifiedMessages, outputMsgs...)

	var inputTokens, outputTokens int
	if rsp.Usage != nil {
		inputTokens = rsp.Usage.InputTokens
		outputTokens = rsp.Usage.OutputTokens
	}

	submitResponseMessageStoreTask(ctx, u.taskSubmitter, req, upstreamModel, unifiedMessages, inputTokens, outputTokens)
}

// convertResponseOutput 将 Response API 响应输出项转换为统一消息格式
//
//	@param rsp *dto.OpenAICreateResponseRsp
//	@return []*vo.UnifiedMessage 转换后的统一消息列表
//	@return bool 是否转换成功
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func convertResponseOutput(rsp *dto.OpenAICreateResponseRsp) ([]*vo.UnifiedMessage, bool) {
	log := logger.Logger()
	outputMsgs, err := dto.FromResponseAPIOutputItems(rsp.Output)
	if err != nil {
		log.Error("[OpenAIUseCase] Failed to convert response output items", zap.Error(err))
		return nil, false
	}
	if len(outputMsgs) == 0 {
		return nil, false
	}
	return outputMsgs, true
}

// storeResponseFromAnthropicMsg Response API Anthropic 变体 → 消息存储
func (u *openAIUseCase) storeResponseFromAnthropicMsg(ctx context.Context, req *dto.OpenAICreateResponseRequest, msg *dto.AnthropicMessage, proxyErr error, upstreamModel string) {
	if proxyErr != nil || msg == nil || len(msg.Content) == 0 {
		return
	}

	unifiedMessages, ok := buildResponseRequestUnifiedMessages(ctx, req)
	if !ok {
		return
	}

	outputMsgs, ok := anthropicResponseContentToUnified(msg.Content)
	if !ok {
		return
	}
	unifiedMessages = append(unifiedMessages, outputMsgs...)

	var inputTokens, outputTokens int
	if msg.Usage != nil {
		inputTokens = msg.Usage.InputTokens
		outputTokens = msg.Usage.OutputTokens
	}

	submitResponseMessageStoreTask(ctx, u.taskSubmitter, req, upstreamModel, unifiedMessages, inputTokens, outputTokens)
}

// buildResponseRequestUnifiedMessages Response API 请求 → UnifiedMessage 前置列表
//
// 返回 (messages, ok)：ok=false 表示 input.Items 转换失败；ok=true 时 messages 可能为空。
func buildResponseRequestUnifiedMessages(ctx context.Context, req *dto.OpenAICreateResponseRequest) ([]*vo.UnifiedMessage, bool) {
	log := logger.WithCtx(ctx)
	var messages []*vo.UnifiedMessage

	if req.Body.Instructions != nil && *req.Body.Instructions != "" {
		messages = append(messages, &vo.UnifiedMessage{
			Role:    enum.RoleSystem,
			Content: &vo.UnifiedContent{Text: *req.Body.Instructions},
		})
	}

	if req.Body.Input != nil {
		if len(req.Body.Input.Items) > 0 {
			inputMsgs, err := dto.FromResponseAPIInputItems(req.Body.Input.Items)
			if err != nil {
				log.Error("[OpenAIUseCase] Failed to convert response input items", zap.Error(err))
				return nil, false
			}
			messages = append(messages, inputMsgs...)
		} else if req.Body.Input.Text != "" {
			messages = append(messages, &vo.UnifiedMessage{
				Role:    enum.RoleUser,
				Content: &vo.UnifiedContent{Text: req.Body.Input.Text},
			})
		}
	}

	return messages, true
}

// buildResponseUnifiedTools Response API 请求 tools → UnifiedTool
func buildResponseUnifiedTools(tools []*dto.ResponseTool) []*vo.UnifiedTool {
	result := make([]*vo.UnifiedTool, 0, len(tools))
	for _, tool := range tools {
		if ut := dto.FromResponseAPITool(tool); ut != nil {
			result = append(result, ut)
		}
	}
	return result
}

// anthropicResponseContentToUnified Anthropic content blocks → UnifiedMessage 列表
//
// 任何 tool_use 块 marshal 失败 → 放弃整条响应落盘（避免残缺消息写入）。
func anthropicResponseContentToUnified(blocks []*dto.AnthropicContentBlock) ([]*vo.UnifiedMessage, bool) {
	log := logger.Logger()
	var messages []*vo.UnifiedMessage
	for _, block := range blocks {
		if block == nil {
			continue
		}
		switch block.Type {
		case enum.AnthropicContentBlockTypeText:
			messages = append(messages, &vo.UnifiedMessage{
				Role:    enum.RoleAssistant,
				Content: &vo.UnifiedContent{Text: block.Text},
			})
		case enum.AnthropicContentBlockTypeThinking:
			messages = append(messages, &vo.UnifiedMessage{
				Role:             enum.RoleAssistant,
				ReasoningContent: lo.FromPtr(block.Thinking),
			})
		case enum.AnthropicContentBlockTypeToolUse:
			args, err := sonic.MarshalString(block.Input)
			if err != nil {
				log.Error("[OpenAIUseCase] Failed to marshal tool_use input, abort storage to avoid partial conversation",
					zap.String("toolID", block.ID), zap.String("toolName", block.Name), zap.Error(err))
				return nil, false
			}
			messages = append(messages, &vo.UnifiedMessage{
				Role: enum.RoleAssistant,
				ToolCalls: []*vo.UnifiedToolCall{{
					ID:   block.ID,
					Name: block.Name,
				}},
				Content: &vo.UnifiedContent{Text: args},
			})
		}
	}
	return messages, true
}

// submitResponseMessageStoreTask Response API 路径统一的消息存储投递
func submitResponseMessageStoreTask(ctx context.Context, submitter TaskSubmitter, req *dto.OpenAICreateResponseRequest, upstreamModel string, messages []*vo.UnifiedMessage, inputTokens, outputTokens int) {
	log := logger.WithCtx(ctx)
	if err := submitter.SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     messages,
		Tools:        buildResponseUnifiedTools(req.Body.Tools),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Metadata:     req.Body.Metadata,
	}); err != nil {
		log.Error("[OpenAIUseCase] Failed to submit response message store task", zap.Error(err))
	}
}
