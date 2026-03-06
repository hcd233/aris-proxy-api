package util

import (
	"fmt"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// ConcatChatCompletionChunks 合并聊天完成流式块
//
//	@param chunks
//	@return *dto.ChatCompletionChunk
//	@return error
//	@author centonhuang
//	@update 2026-03-06 18:08:53
func ConcatChatCompletionChunks(chunks []*dto.ChatCompletionChunk) (*dto.ChatCompletionChunk, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks to concat")
	}

	ret := &dto.ChatCompletionChunk{}

	// choiceBuilders accumulates per-index delta state.
	type choiceState struct {
		role         enum.Role
		contentParts []string
		refusalParts []string
		toolCalls    []any
		finishReason enum.FinishReason
		logprobs     *dto.Logprobs
		index        int
	}
	choiceMap := make(map[int]*choiceState)
	choiceOrder := make([]int, 0)

	for idx, chunk := range chunks {
		if chunk == nil {
			return nil, fmt.Errorf("unexpected nil chunk at index %d", idx)
		}

		// Metadata: use the first chunk's values.
		if ret.ID == "" {
			ret.ID = chunk.ID
			ret.Created = chunk.Created
			ret.Object = chunk.Object
			ret.ServiceTier = chunk.ServiceTier
			ret.SystemFingerprint = chunk.SystemFingerprint
			ret.Model = chunk.Model
		}

		// Usage: keep the last non-nil value (upstream sends it in the final chunk).
		if chunk.Usage != nil {
			ret.Usage = chunk.Usage
		}

		for _, choice := range chunk.Choices {
			if choice == nil {
				continue
			}
			cs, exists := choiceMap[choice.Index]
			if !exists {
				cs = &choiceState{index: choice.Index}
				choiceMap[choice.Index] = cs
				choiceOrder = append(choiceOrder, choice.Index)
			}

			if choice.Delta != nil {
				if cs.role == "" && choice.Delta.Role != "" {
					cs.role = choice.Delta.Role
				}
				if choice.Delta.Content != "" {
					cs.contentParts = append(cs.contentParts, choice.Delta.Content)
				}
				if choice.Delta.Refusal != "" {
					cs.refusalParts = append(cs.refusalParts, choice.Delta.Refusal)
				}
				if len(choice.Delta.ToolCalls) > 0 {
					cs.toolCalls = append(cs.toolCalls, choice.Delta.ToolCalls...)
				}
			}

			if choice.FinishReason != "" {
				cs.finishReason = choice.FinishReason
			}

			if choice.Logprobs != nil {
				if cs.logprobs == nil {
					cs.logprobs = &dto.Logprobs{}
				}
				cs.logprobs.Content = append(cs.logprobs.Content, choice.Logprobs.Content...)
				cs.logprobs.Refusal = append(cs.logprobs.Refusal, choice.Logprobs.Refusal...)
			}
		}
	}

	ret.Choices = make([]*dto.ChatCompletionChunkChoice, 0, len(choiceOrder))
	for _, idx := range choiceOrder {
		cs := choiceMap[idx]
		delta := &dto.ChatCompletionChunkDelta{
			Role:      cs.role,
			Content:   strings.Join(cs.contentParts, ""),
			Refusal:   strings.Join(cs.refusalParts, ""),
			ToolCalls: cs.toolCalls,
		}
		ret.Choices = append(ret.Choices, &dto.ChatCompletionChunkChoice{
			Index:        cs.index,
			Delta:        delta,
			FinishReason: cs.finishReason,
			Logprobs:     cs.logprobs,
		})
	}

	return ret, nil
}
