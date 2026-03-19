package util

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
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
	type toolCallState struct {
		id           string
		toolType     enum.ToolType
		functionName []string
		functionArgs []string
		customName   []string
		customInput  []string
		hasFunction  bool
		hasCustom    bool
	}

	type choiceState struct {
		role                  enum.Role
		contentParts          []string
		reasoningContentParts []string
		refusalParts          []string
		toolCallMap           map[int]*toolCallState // keyed by tool_call index
		toolCallOrder         []int
		finishReason          enum.FinishReason
		logprobs              *dto.Logprobs
		index                 int
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
				cs = &choiceState{
					index:       choice.Index,
					toolCallMap: make(map[int]*toolCallState),
				}
				choiceMap[choice.Index] = cs
				choiceOrder = append(choiceOrder, choice.Index)
			}

			if cs.role == "" && choice.Delta.Role != "" {
				cs.role = choice.Delta.Role
			}
			if choice.Delta.Content != "" {
				cs.contentParts = append(cs.contentParts, choice.Delta.Content)
			}
			if choice.Delta.ReasoningContent != "" {
				cs.reasoningContentParts = append(cs.reasoningContentParts, choice.Delta.ReasoningContent)
			}
			if choice.Delta.Refusal != "" {
				cs.refusalParts = append(cs.refusalParts, choice.Delta.Refusal)
			}

			// Merge tool_call deltas by their index within the tool_calls array.
			// Streaming chunks carry tool_calls with an "index" field (encoded in
			// ChatCompletionMessageToolCall.Index) that indicates which logical
			// tool_call the delta belongs to. We accumulate id, type, function
			// name/arguments fragments and merge them into one complete tool_call
			// per index.
			for _, tc := range choice.Delta.ToolCalls {
				tcIdx := tc.Index
				tcs, ok := cs.toolCallMap[tcIdx]
				if !ok {
					tcs = &toolCallState{}
					cs.toolCallMap[tcIdx] = tcs
					cs.toolCallOrder = append(cs.toolCallOrder, tcIdx)
				}
				if tc.ID != "" {
					tcs.id = tc.ID
				}
				if tc.Type != "" {
					tcs.toolType = tc.Type
				}
				if tc.Function != nil {
					tcs.hasFunction = true
					if tc.Function.Name != "" {
						tcs.functionName = append(tcs.functionName, tc.Function.Name)
					}
					if tc.Function.Arguments != "" {
						tcs.functionArgs = append(tcs.functionArgs, tc.Function.Arguments)
					}
				}
				if tc.Custom != nil {
					tcs.hasCustom = true
					if tc.Custom.Name != "" {
						tcs.customName = append(tcs.customName, tc.Custom.Name)
					}
					if tc.Custom.Input != "" {
						tcs.customInput = append(tcs.customInput, tc.Custom.Input)
					}
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

	cmpl.Choices = make([]*dto.ChatCompletionChoice, 0, len(choiceOrder))
	for _, idx := range choiceOrder {
		cs := choiceMap[idx]

		// Build merged tool_calls from accumulated deltas.
		var mergedToolCalls []*dto.ChatCompletionMessageToolCall
		for _, tcIdx := range cs.toolCallOrder {
			tcs := cs.toolCallMap[tcIdx]
			tc := &dto.ChatCompletionMessageToolCall{
				ID:   tcs.id,
				Type: tcs.toolType,
			}
			if tcs.hasFunction {
				tc.Function = &dto.ChatCompletionMessageFunctionToolCall{
					Name:      strings.Join(tcs.functionName, ""),
					Arguments: strings.Join(tcs.functionArgs, ""),
				}
			}
			if tcs.hasCustom {
				tc.Custom = &dto.ChatCompletionMessageCustomToolCall{
					Name:  strings.Join(tcs.customName, ""),
					Input: strings.Join(tcs.customInput, ""),
				}
			}
			mergedToolCalls = append(mergedToolCalls, tc)
		}

		// Use nil instead of empty string for Content to match non-stream responses
		// when there is no textual content (e.g. tool-call-only messages).
		var content *dto.MessageContent
		if joined := strings.Join(cs.contentParts, ""); joined != "" {
			content = &dto.MessageContent{Text: joined}
		}

		message := &dto.ChatCompletionMessageParam{
			Role:             cmp.Or(cs.role, enum.RoleAssistant),
			Content:          content,
			ReasoningContent: strings.Join(cs.reasoningContentParts, ""),
			Refusal:          strings.Join(cs.refusalParts, ""),
			ToolCalls:        mergedToolCalls,
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

// ComputeMessageChecksum 计算统一消息校验和
//
// 对 UnifiedMessage 做规范化处理，确保语义相同但表示不同的消息产生相同的 checksum：
//
//   - 清除 ToolCalls 中的 ID（上游分配的标识符，不影响消息语义，
//     且同一条消息在流式和非流式路径中可能产生不同的 ID 格式）
//
//   - 清除 ToolCallID（工具结果消息中引用的调用 ID，同理不影响语义）
//
//   - 序列化规范化后的结构体，计算 SHA256
//
//     @param msg *dto.UnifiedMessage
//     @return string
//     @author centonhuang
//     @update 2026-03-18 10:00:00
func ComputeMessageChecksum(msg *dto.UnifiedMessage) string {
	// 深拷贝以避免修改原始消息
	normalized := *msg

	// 清除易变的标识符字段
	normalized.ToolCallID = ""

	if len(normalized.ToolCalls) > 0 {
		cleanedCalls := make([]*dto.UnifiedToolCall, len(normalized.ToolCalls))
		for i, tc := range normalized.ToolCalls {
			cleanedCalls[i] = &dto.UnifiedToolCall{
				Name:      tc.Name,
				Arguments: normalizeJSONString(tc.Arguments),
			}
		}
		normalized.ToolCalls = cleanedCalls
	}

	hash := sha256.Sum256(lo.Must1(sonic.Marshal(normalized)))
	return hex.EncodeToString(hash[:])
}

// normalizeJSONString 将 JSON 字符串反序列化后重新序列化为紧凑格式，消除空格等格式差异
//
//	@param s string JSON字符串
//	@return string 规范化后的JSON字符串，如果解析失败则返回原始字符串
//	@author centonhuang
//	@update 2026-03-18 10:00:00
func normalizeJSONString(s string) string {
	var obj map[string]any
	lo.Must0(sonic.UnmarshalString(s, &obj))
	return lo.Must1(sonic.MarshalString(obj))
}
