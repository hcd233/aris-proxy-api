package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
func ConcatChatCompletionChunks(chunks []*dto.ChatCompletionChunk) (*dto.ChatCompletion, error) {
	cmpl := &dto.ChatCompletion{}

	if len(chunks) == 0 {
		return cmpl, nil
	}

	// choiceBuilders accumulates per-index delta state.
	type choiceState struct {
		role         enum.Role
		contentParts []string
		refusalParts []string
		toolCalls    []*dto.ChatCompletionMessageToolCall
		finishReason enum.FinishReason
		logprobs     *dto.Logprobs
		index        int
	}
	choiceMap := make(map[int]*choiceState)
	choiceOrder := make([]int, 0)

	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}

		// Metadata: use the first chunk's values.
		if cmpl.ID == "" {
			cmpl.ID = chunk.ID
			cmpl.Created = chunk.Created
			cmpl.Object = chunk.Object
			cmpl.ServiceTier = chunk.ServiceTier
			cmpl.SystemFingerprint = chunk.SystemFingerprint
			cmpl.Model = chunk.Model
		}

		// Usage: keep the last non-nil value (upstream sends it in the final chunk).
		if chunk.Usage != nil {
			cmpl.Usage = chunk.Usage
		}

		for _, choice := range chunk.Choices {
			cs, exists := choiceMap[choice.Index]
			if !exists {
				cs = &choiceState{index: choice.Index}
				choiceMap[choice.Index] = cs
				choiceOrder = append(choiceOrder, choice.Index)
			}

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

	cmpl.Choices = make([]*dto.ChatCompletionChoice, 0, len(choiceOrder))
	for _, idx := range choiceOrder {
		cs := choiceMap[idx]
		message := &dto.ChatCompletionMessageParam{
			Role:      cs.role,
			Content:   strings.Join(cs.contentParts, ""),
			Refusal:   strings.Join(cs.refusalParts, ""),
			ToolCalls: cs.toolCalls,
		}
		cmpl.Choices = append(cmpl.Choices, &dto.ChatCompletionChoice{
			Index:        cs.index,
			Message:      message,
			FinishReason: cs.finishReason,
			Logprobs:     cs.logprobs,
		})
	}

	return cmpl, nil
}

// ComputeMessageChecksum 计算消息校验和（基于Content和ToolCalls）
//
//	@param msg *dto.ChatCompletionMessageParam
//	@return string
//	@return error
//	@author centonhuang
//	@update 2026-03-10 10:00:00
func ComputeMessageChecksum(msg *dto.ChatCompletionMessageParam) (string, error) {
	if msg == nil {
		return "", nil
	}

	// 构建用于计算校验和的数据结构
	data := struct {
		Content   any                                  `json:"content"`
		ToolCalls []*dto.ChatCompletionMessageToolCall `json:"tool_calls"`
	}{
		Content:   msg.Content,
		ToolCalls: msg.ToolCalls,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:]), nil
}
