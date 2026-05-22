package converter

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
)

func convertContentBlockDeltaToChunks(data sonic.NoCopyRawMessage, model, chunkID string) ([]*dto.OpenAIChatCompletionChunk, error) {
	var payload dto.AnthropicSSEContentBlockDelta
	if err := sonic.Unmarshal(data, &payload); err != nil {
		return nil, ierr.Wrap(ierr.ErrSSEParse, err, "unmarshal content_block_delta")
	}

	chunk := &dto.OpenAIChatCompletionChunk{
		ID:      chunkID,
		Object:  constant.OpenAICompletionChunkObject,
		Created: time.Now().Unix(),
		Model:   model,
	}

	delta := &dto.OpenAIChatCompletionChunkDelta{}

	switch payload.Delta.Type {
	case enum.AnthropicDeltaTypeTextDelta:
		delta.Content = payload.Delta.Text
	case enum.AnthropicDeltaTypeThinkingDelta:
		delta.ReasoningContent = payload.Delta.Thinking
	case enum.AnthropicDeltaTypeInputJSONDelta:
		delta.ToolCalls = []*dto.OpenAIChatCompletionMessageToolCall{{
			Index: lo.ToPtr(payload.Index),
			Type:  enum.ToolTypeFunction,
			Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
				Arguments: payload.Delta.PartialJSON,
			},
		}}
	default:
		return nil, nil
	}

	chunk.Choices = []*dto.OpenAIChatCompletionChunkChoice{{
		Index: payload.Index,
		Delta: delta,
	}}

	return []*dto.OpenAIChatCompletionChunk{chunk}, nil
}

func convertMessageDeltaToChunks(data sonic.NoCopyRawMessage, model, chunkID string) ([]*dto.OpenAIChatCompletionChunk, error) {
	var payload dto.AnthropicSSEMessageDelta
	if err := sonic.Unmarshal(data, &payload); err != nil {
		return nil, ierr.Wrap(ierr.ErrSSEParse, err, "unmarshal message_delta")
	}

	chunk := &dto.OpenAIChatCompletionChunk{
		ID:      chunkID,
		Object:  constant.OpenAICompletionChunkObject,
		Created: time.Now().Unix(),
		Model:   model,
	}

	finishReason := convertAnthropicStopReasonToOpenAI(payload.Delta.StopReason)

	chunk.Choices = []*dto.OpenAIChatCompletionChunkChoice{{
		Index:        0,
		Delta:        &dto.OpenAIChatCompletionChunkDelta{},
		FinishReason: finishReason,
	}}

	if payload.Usage != nil {
		chunk.Usage = &dto.OpenAICompletionUsage{
			PromptTokens:     payload.Usage.InputTokens,
			CompletionTokens: payload.Usage.OutputTokens,
			TotalTokens:      payload.Usage.InputTokens + payload.Usage.OutputTokens,
		}
	}

	return []*dto.OpenAIChatCompletionChunk{chunk}, nil
}

func convertContentBlockStartToChunks(data sonic.NoCopyRawMessage, model, chunkID string) ([]*dto.OpenAIChatCompletionChunk, error) {
	var payload dto.AnthropicSSEContentBlockStart
	if err := sonic.Unmarshal(data, &payload); err != nil {
		return nil, ierr.Wrap(ierr.ErrSSEParse, err, "unmarshal content_block_start")
	}

	if payload.ContentBlock == nil {
		return nil, nil
	}

	// tool_use 开始事件 -> OpenAI tool_calls chunk
	if payload.ContentBlock.Type == enum.AnthropicContentBlockTypeToolUse {
		name := lo.FromPtr(payload.ContentBlock.Name)
		chunk := &dto.OpenAIChatCompletionChunk{
			ID:      chunkID,
			Object:  constant.OpenAICompletionChunkObject,
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []*dto.OpenAIChatCompletionChunkChoice{{
				Index: 0,
				Delta: &dto.OpenAIChatCompletionChunkDelta{
					ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
						Index: lo.ToPtr(payload.Index),
						ID:    payload.ContentBlock.ID,
						Type:  enum.ToolTypeFunction,
						Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
							Name: name,
						},
					}},
				},
			}},
		}
		return []*dto.OpenAIChatCompletionChunk{chunk}, nil
	}

	// text/thinking 开始事件 -> OpenAI role chunk
	if payload.ContentBlock.Type == enum.AnthropicContentBlockTypeText ||
		payload.ContentBlock.Type == enum.AnthropicContentBlockTypeThinking {
		chunk := &dto.OpenAIChatCompletionChunk{
			ID:      chunkID,
			Object:  constant.OpenAICompletionChunkObject,
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []*dto.OpenAIChatCompletionChunkChoice{{
				Index: 0,
				Delta: &dto.OpenAIChatCompletionChunkDelta{
					Role: enum.RoleAssistant,
				},
			}},
		}
		return []*dto.OpenAIChatCompletionChunk{chunk}, nil
	}

	return nil, nil
}
