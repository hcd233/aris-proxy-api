package util

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
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

// SendOpenAIUpstreamError 发送上游错误响应
//
//	@param statusCode int
//	@param body string
//	@return rsp
//	@author centonhuang
//	@update 2026-03-31 10:00:00
func SendOpenAIUpstreamError(statusCode int, body string) (rsp *huma.StreamResponse) {
	// 尝试从上游错误响应中提取安全的 error message
	var errMsg string
	var errResp dto.OpenAIErrorResponse
	if err := sonic.UnmarshalString(body, &errResp); err == nil && errResp.Error != nil && errResp.Error.Message != "" {
		errMsg = errResp.Error.Message
	} else {
		errMsg = fmt.Sprintf("Upstream returned status %d", statusCode)
	}

	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			humaCtx.SetStatus(statusCode)
			humaCtx.SetHeader("Content-Type", "application/json")
			humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
				Error: &dto.OpenAIError{
					Message: errMsg,
					Type:    "upstream_error",
					Code:    "upstream_error",
				},
			})))
		},
	}
}
