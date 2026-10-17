package transport

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// buildMessageBlock 构造承载完整正文的 text block（非流式响应与流内 message_done 用）。
func buildMessageBlock(spec MockSpec) *dto.AnthropicContentBlock {
	text := string(spec.Chars)
	return &dto.AnthropicContentBlock{Type: enum.AnthropicContentBlockTypeText, Text: &text}
}

// buildEmptyMessageBlock 构造 content_block_start 的占位块：只声明块存在，text 为空串。
//
// 真协议与本仓 OpenAI→Anthropic 转换器（converter/openai.go handleTextDelta）都发空串，
// 正文全部走 content_block_delta。若 start 带全文，native 路径逐字转发后，客户端（官方
// SDK 的累加语义）会得到两倍正文；网关自身聚合因覆盖语义恰好掩盖该缺陷，落库与
// 既有单测都发现不了。
func buildEmptyMessageBlock() *dto.AnthropicContentBlock {
	return &dto.AnthropicContentBlock{Type: enum.AnthropicContentBlockTypeText, Text: lo.ToPtr("")}
}

// MessageStreamLen 流式事件总数：message_start + content_block_start + N×delta +
// content_block_stop + message_delta + message_stop。
func MessageStreamLen(spec MockSpec) int { return spec.ChunkCount() + 5 }

// buildMessageUsage 非流式 message 与流内 message_delta 共用的 usage（同口径）。
func buildMessageUsage(spec MockSpec) *dto.AnthropicUsage {
	return &dto.AnthropicUsage{InputTokens: spec.InputTokens, OutputTokens: spec.Tokens()}
}

// BuildMessageEventAt 按需构造第 i 个流式事件（i ∈ [0, MessageStreamLen)）：
// message_start → content_block_start（空块）→ N×content_block_delta（正文）→
// content_block_stop → message_delta → message_stop。
// 事件类型须全在聚合器已知集合内（processAnthropicSSEEvent 对未知类型报错）；
// content_block_start 的 Index 与 delta 的 Index 必须对应，否则聚合器报 unknown index。
// 逐事件按需构造，不预物化整条流（16K 字符上限 = 4096 个 delta）。
func BuildMessageEventAt(spec MockSpec, model, messageID string, i int) MockResponseEvent {
	chunkCount := spec.ChunkCount()
	switch {
	case i == 0:
		return mockEvent(enum.AnthropicSSEEventTypeMessageStart, &dto.AnthropicSSEMessageStart{
			Message: &dto.AnthropicMessage{
				ID:    messageID,
				Type:  enum.AnthropicMessageTypeMessage,
				Role:  enum.RoleAssistant,
				Model: model,
				Usage: &dto.AnthropicUsage{InputTokens: spec.InputTokens},
			},
		})
	case i == 1:
		return mockEvent(enum.AnthropicSSEEventTypeContentBlockStart, &dto.AnthropicSSEContentBlockStart{
			Index:        0,
			ContentBlock: buildEmptyMessageBlock(),
		})
	case i < chunkCount+2:
		return mockEvent(enum.AnthropicSSEEventTypeContentBlockDelta, &dto.AnthropicSSEContentBlockDelta{
			Index: 0,
			Delta: dto.AnthropicSSEContentBlockDeltaPayload{Type: enum.AnthropicDeltaTypeTextDelta, Text: spec.ChunkAt(i - 2)},
		})
	case i == chunkCount+2:
		return mockEvent(enum.AnthropicSSEEventTypeContentBlockStop, map[string]any{
			constant.ResponseStreamFieldType: enum.AnthropicSSEEventTypeContentBlockStop,
		})
	case i == chunkCount+3:
		return mockEvent(enum.AnthropicSSEEventTypeMessageDelta, &dto.AnthropicSSEMessageDelta{
			Delta: dto.AnthropicSSEMessageDeltaPayload{StopReason: lo.ToPtr(enum.AnthropicStopReasonEndTurn)},
			Usage: buildMessageUsage(spec),
		})
	default:
		return mockEvent(enum.AnthropicSSEEventTypeMessageStop, map[string]any{
			constant.ResponseStreamFieldType: enum.AnthropicSSEEventTypeMessageStop,
		})
	}
}

// BuildMessage 生成非流式 Anthropic Message 响应。
// ID 用与流式同一模板生成（每次唯一）；流式路径由 emitMessageStream 覆写为
// 流内 message_start 的 ID，保证同一响应对象 ID 一致。
func BuildMessage(spec MockSpec, model string) *dto.AnthropicMessage {
	return &dto.AnthropicMessage{
		ID:         mockID(constant.AnthropicMessageIDTemplate),
		Type:       enum.AnthropicMessageTypeMessage,
		Role:       enum.RoleAssistant,
		Model:      model,
		Content:    []*dto.AnthropicContentBlock{buildMessageBlock(spec)},
		StopReason: lo.ToPtr(enum.AnthropicStopReasonEndTurn),
		Usage:      buildMessageUsage(spec),
	}
}

// BuildTokensCount 生成 count_tokens 响应：与 /messages 的 usage.input_tokens 同源
// （都取请求体估算值），保证同一 body 的计数稳定、跨端点口径一致。
func BuildTokensCount(inputTokens int) *dto.AnthropicTokensCount {
	return &dto.AnthropicTokensCount{InputTokens: inputTokens}
}
