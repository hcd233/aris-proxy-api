package transport

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// buildMessageBlock 构造 mock 内容块：一条 text block。
func buildMessageBlock(spec MockSpec) *dto.AnthropicContentBlock {
	text := string(spec.Chars)
	return &dto.AnthropicContentBlock{Type: enum.AnthropicContentBlockTypeText, Text: &text}
}

// BuildMessageStream 生成 Anthropic Messages 流式事件序列：
// message_start → content_block_start → N×content_block_delta → content_block_stop
// → message_delta → message_stop。
// 事件类型须全在聚合器已知集合内（processAnthropicSSEEvent 对未知类型报错）；
// content_block_start 的 Index 与 delta 的 Index 必须对应，否则聚合器报 unknown index。
func BuildMessageStream(spec MockSpec, model, messageID string) []MockResponseEvent {
	pieces := spec.Chunks()
	block := buildMessageBlock(spec)
	msg := BuildMessage(spec, model)
	out := make([]MockResponseEvent, 0, len(pieces)+5)

	out = append(out,
		mockEvent(enum.AnthropicSSEEventTypeMessageStart, &dto.AnthropicSSEMessageStart{
			Message: &dto.AnthropicMessage{
				ID:    messageID,
				Type:  enum.AnthropicMessageTypeMessage,
				Role:  enum.RoleAssistant,
				Model: model,
				Usage: &dto.AnthropicUsage{},
			},
		}),
		mockEvent(enum.AnthropicSSEEventTypeContentBlockStart, &dto.AnthropicSSEContentBlockStart{
			Index:        0,
			ContentBlock: block,
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
func BuildMessage(spec MockSpec, model string) *dto.AnthropicMessage {
	return &dto.AnthropicMessage{
		ID:         constant.MockUpstreamModel + "-msg",
		Type:       enum.AnthropicMessageTypeMessage,
		Role:       enum.RoleAssistant,
		Model:      model,
		Content:    []*dto.AnthropicContentBlock{buildMessageBlock(spec)},
		StopReason: lo.ToPtr(enum.AnthropicStopReasonEndTurn),
		Usage:      &dto.AnthropicUsage{OutputTokens: spec.Tokens()},
	}
}

// BuildTokensCount 生成 count_tokens 响应（按同口径估算）。
func BuildTokensCount(spec MockSpec) *dto.AnthropicTokensCount {
	return &dto.AnthropicTokensCount{InputTokens: spec.Tokens()}
}
