package converter

import (
	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

func convertOpenAIFinishReasonToAnthropic(reason enum.FinishReason) *string {
	switch reason {
	case enum.FinishReasonStop:
		return lo.ToPtr(enum.AnthropicStopReasonEndTurn)
	case enum.FinishReasonLength:
		return lo.ToPtr(enum.AnthropicStopReasonMaxTokens)
	case enum.FinishReasonToolCalls:
		return lo.ToPtr(enum.AnthropicStopReasonToolUse)
	case enum.FinishReasonContentFilter:
		return lo.ToPtr(enum.AnthropicStopReasonEndTurn)
	default:
		return lo.ToPtr(enum.AnthropicStopReasonEndTurn)
	}
}

func convertOpenAIMessageToAnthropicContent(msg *dto.OpenAIChatCompletionMessageParam) ([]*dto.AnthropicContentBlock, error) {
	if msg == nil {
		return []*dto.AnthropicContentBlock{}, nil
	}

	var blocks []*dto.AnthropicContentBlock

	// 推理内容 -> thinking block
	if lo.FromPtr(msg.ReasoningContent) != "" {
		blocks = append(blocks, &dto.AnthropicContentBlock{
			Type:     enum.AnthropicContentBlockTypeThinking,
			Thinking: msg.ReasoningContent,
		})
	}

	// 文本内容 -> text block
	if msg.Content != nil {
		if msg.Content.Text != "" {
			t := msg.Content.Text
			blocks = append(blocks, &dto.AnthropicContentBlock{
				Type: enum.AnthropicContentBlockTypeText,
				Text: &t,
			})
		} else if len(msg.Content.Parts) > 0 {
			for _, part := range msg.Content.Parts {
				if part.Type == enum.ContentPartTypeText && lo.FromPtr(part.Text) != "" {
					blocks = append(blocks, &dto.AnthropicContentBlock{
						Type: enum.AnthropicContentBlockTypeText,
						Text: part.Text,
					})
				}
			}
		}
	}

	// 工具调用 -> tool_use blocks
	for _, tc := range msg.ToolCalls {
		if tc.Function != nil {
			var input map[string]any
			if tc.Function.Arguments != "" {
				if err := sonic.UnmarshalString(tc.Function.Arguments, &input); err != nil {
					return nil, ierr.Wrapf(ierr.ErrDTOUnmarshal, err, "unmarshal tool call arguments for %q", tc.Function.Name)
				}
			}
			name := tc.Function.Name
			blocks = append(blocks, &dto.AnthropicContentBlock{
				Type:  enum.AnthropicContentBlockTypeToolUse,
				ID:    tc.ID,
				Name:  &name,
				Input: input,
			})
		}
	}

	if len(blocks) == 0 {
		blocks = append(blocks, &dto.AnthropicContentBlock{
			Type: enum.AnthropicContentBlockTypeText,
			Text: lo.ToPtr(""),
		})
	}

	return blocks, nil
}

func convertChunkUsageToAnthropic(usage *dto.OpenAICompletionUsage) *dto.AnthropicUsage {
	if usage == nil {
		return &dto.AnthropicUsage{}
	}
	return &dto.AnthropicUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}
}

func newContentBlockStartEvent(index int, block *dto.AnthropicContentBlock) dto.AnthropicSSEEvent {
	return dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeContentBlockStart,
		Data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockStart{
			Index:        index,
			ContentBlock: block,
		})),
	}
}

func newTextDeltaEvent(index int, text string) dto.AnthropicSSEEvent {
	return dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeContentBlockDelta,
		Data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockDelta{
			Index: index,
			Delta: dto.AnthropicSSEContentBlockDeltaPayload{
				Type: enum.AnthropicDeltaTypeTextDelta,
				Text: text,
			},
		})),
	}
}

func newThinkingDeltaEvent(index int, thinking string) dto.AnthropicSSEEvent {
	return dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeContentBlockDelta,
		Data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockDelta{
			Index: index,
			Delta: dto.AnthropicSSEContentBlockDeltaPayload{
				Type:     enum.AnthropicDeltaTypeThinkingDelta,
				Thinking: thinking,
			},
		})),
	}
}

func newInputJSONDeltaEvent(index int, partialJSON string) dto.AnthropicSSEEvent {
	return dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeContentBlockDelta,
		Data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockDelta{
			Index: index,
			Delta: dto.AnthropicSSEContentBlockDeltaPayload{
				Type:        enum.AnthropicDeltaTypeInputJSONDelta,
				PartialJSON: partialJSON,
			},
		})),
	}
}
