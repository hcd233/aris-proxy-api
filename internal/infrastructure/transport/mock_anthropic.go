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

// BuildMessageStream 生成 Anthropic Messages 流式事件序列：
// message_start → content_block_start（空块）→ N×content_block_delta（正文）→
// content_block_stop → message_delta → message_stop。
// 事件类型须全在聚合器已知集合内（processAnthropicSSEEvent 对未知类型报错）；
// content_block_start 的 Index 与 delta 的 Index 必须对应，否则聚合器报 unknown index。
func BuildMessageStream(spec MockSpec, model, messageID string) []MockResponseEvent {
	pieces := spec.Chunks()
	msg := BuildMessage(spec, model)
	out := make([]MockResponseEvent, 0, len(pieces)+5)

	out = append(out,
		mockEvent(enum.AnthropicSSEEventTypeMessageStart, &dto.AnthropicSSEMessageStart{
			Message: &dto.AnthropicMessage{
				ID:    messageID,
				Type:  enum.AnthropicMessageTypeMessage,
				Role:  enum.RoleAssistant,
				Model: model,
				Usage: &dto.AnthropicUsage{InputTokens: spec.InputTokens},
			},
		}),
		mockEvent(enum.AnthropicSSEEventTypeContentBlockStart, &dto.AnthropicSSEContentBlockStart{
			Index:        0,
			ContentBlock: buildEmptyMessageBlock(),
		}),
	)
	for _, piece := range pieces {
		out = append(out, mockEvent(enum.AnthropicSSEEventTypeContentBlockDelta, &dto.AnthropicSSEContentBlockDelta{
			Index: 0,
			Delta: dto.AnthropicSSEContentBlockDeltaPayload{Type: enum.AnthropicDeltaTypeTextDelta, Text: piece},
		}))
	}
	out = append(out,
		mockEvent(enum.AnthropicSSEEventTypeContentBlockStop, map[string]any{
			constant.ResponseStreamFieldType: enum.AnthropicSSEEventTypeContentBlockStop,
		}),
		mockEvent(enum.AnthropicSSEEventTypeMessageDelta, &dto.AnthropicSSEMessageDelta{
			Delta: dto.AnthropicSSEMessageDeltaPayload{StopReason: lo.ToPtr(enum.AnthropicStopReasonEndTurn)},
			Usage: msg.Usage,
		}),
		mockEvent(enum.AnthropicSSEEventTypeMessageStop, map[string]any{
			constant.ResponseStreamFieldType: enum.AnthropicSSEEventTypeMessageStop,
		}),
	)
	return out
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
		Usage:      &dto.AnthropicUsage{InputTokens: spec.InputTokens, OutputTokens: spec.Tokens()},
	}
}

// BuildTokensCount 生成 count_tokens 响应（按同口径估算）。
func BuildTokensCount(spec MockSpec) *dto.AnthropicTokensCount {
	return &dto.AnthropicTokensCount{InputTokens: spec.Tokens()}
}
