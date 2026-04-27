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
//	@return *dto.OpenAIChatCompletionChunk
//	@return error
//	@author centonhuang
//	@update 2026-03-06 18:08:53
func ConcatChatCompletionChunks(chunks []*dto.OpenAIChatCompletionChunk) (*dto.OpenAIChatCompletion, error) {
	cmpl := &dto.OpenAIChatCompletion{}

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
		logprobs              *dto.OpenAILogprobs
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
			// OpenAIChatCompletionMessageToolCall.Index) that indicates which logical
			// tool_call the delta belongs to. We accumulate id, type, function
			// name/arguments fragments and merge them into one complete tool_call
			// per index.
			for _, tc := range choice.Delta.ToolCalls {
				tcIdx := choice.Index
				if tc.Index != nil {
					tcIdx = *tc.Index
				}
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
					cs.logprobs = &dto.OpenAILogprobs{}
				}
				cs.logprobs.Content = append(cs.logprobs.Content, choice.Logprobs.Content...)
				cs.logprobs.Refusal = append(cs.logprobs.Refusal, choice.Logprobs.Refusal...)
			}
		}
	}

	cmpl.Choices = make([]*dto.OpenAIChatCompletionChoice, 0, len(choiceOrder))
	for _, idx := range choiceOrder {
		cs := choiceMap[idx]

		// Build merged tool_calls from accumulated deltas.
		var mergedToolCalls []*dto.OpenAIChatCompletionMessageToolCall
		for _, tcIdx := range cs.toolCallOrder {
			tcs := cs.toolCallMap[tcIdx]
			tc := &dto.OpenAIChatCompletionMessageToolCall{
				ID:   tcs.id,
				Type: tcs.toolType,
			}
			if tcs.hasFunction {
				tc.Function = &dto.OpenAIChatCompletionMessageFunctionToolCall{
					Name:      strings.Join(tcs.functionName, ""),
					Arguments: strings.Join(tcs.functionArgs, ""),
				}
			}
			if tcs.hasCustom {
				tc.Custom = &dto.OpenAIChatCompletionMessageCustomToolCall{
					Name:  strings.Join(tcs.customName, ""),
					Input: strings.Join(tcs.customInput, ""),
				}
			}
			mergedToolCalls = append(mergedToolCalls, tc)
		}

		// Use nil instead of empty string for Content to match non-stream responses
		// when there is no textual content (e.g. tool-call-only messages).
		var content *dto.OpenAIMessageContent
		if joined := strings.Join(cs.contentParts, ""); joined != "" {
			content = &dto.OpenAIMessageContent{Text: joined}
		}

		message := &dto.OpenAIChatCompletionMessageParam{
			Role:             cmp.Or(cs.role, enum.RoleAssistant),
			Content:          content,
			ReasoningContent: strings.Join(cs.reasoningContentParts, ""),
			Refusal:          strings.Join(cs.refusalParts, ""),
			ToolCalls:        mergedToolCalls,
		}
		cmpl.Choices = append(cmpl.Choices, &dto.OpenAIChatCompletionChoice{
			Index:        cs.index,
			Message:      message,
			FinishReason: cs.finishReason,
			Logprobs:     cs.logprobs,
		})
	}

	return cmpl, nil
}

// IsResponseAPITerminalEvent reports whether event is one of the three
// terminal SSE events emitted by the OpenAI Response API
// (response.completed / response.failed / response.incomplete). Each of
// them carries the final Response object with usage, which the gateway
// needs for both audit accounting and error reporting.
//
//	@param event string
//	@return bool
//	@author centonhuang
//	@update 2026-04-18 17:00:00
func IsResponseAPITerminalEvent(event string) bool {
	switch event {
	case enum.ResponseStreamEventCompleted,
		enum.ResponseStreamEventFailed,
		enum.ResponseStreamEventIncomplete:
		return true
	}
	return false
}

// IsResponseAPIDeltaEvent reports whether event is a delta SSE event that
// carries real generated tokens.
//
// All events that deliver generated content share the `.delta` suffix
// (response.output_text.delta, response.reasoning_text.delta,
// response.function_call_arguments.delta, response.audio.delta,
// response.custom_tool_call_input.delta, ...). Metadata events like
// response.created / response.in_progress / response.output_item.added do
// not. Measuring time-to-first-token on delta events keeps the audit
// metric comparable to /chat/completions (which only points on content
// deltas) instead of the first SSE frame of the stream.
//
//	@param event string
//	@return bool
//	@author centonhuang
//	@update 2026-04-18 17:00:00
func IsResponseAPIDeltaEvent(event string) bool {
	return strings.HasSuffix(event, enum.ResponseStreamEventDeltaSuffix)
}

// reasoningContentPlaceholder 用于给缺失 reasoning_content 的 assistant tool call
// message 补位。Moonshot AI 的思考模式会把空字符串 "" 判为 missing 直接返回 400
// （参考 LiteLLM issue #21672），因此使用单空格作为最小合法占位，既满足上游
// 校验又对模型行为影响最小；与 LiteLLM PR #23580 的修复方式保持一致。
const reasoningContentPlaceholder = " "

// EnsureAssistantMessageReasoningContent 在序列化后的 JSON body 中，为缺少
// reasoning_content 的 assistant tool call message 补上占位符 " "。
//
// 部分上游 provider（如 Moonshot AI）在 thinking 模式下要求带 tool_calls 的
// assistant message 必须携带**非空的** reasoning_content 字段；Go struct 的
// omitempty 会把空字符串省略，而上游又会把 "" 判为 missing，两种情况都会导致
// 400: "thinking is enabled but reasoning_content is missing in assistant tool
// call message"。因此这里统一补一个空格占位。
//
//	@param body []byte 序列化后的 OpenAIChatCompletionReq JSON
//	@return []byte 处理后的 JSON
//	@author centonhuang
//	@update 2026-04-27 16:40:00
func EnsureAssistantMessageReasoningContent(body []byte) []byte {
	var root map[string]any
	if err := sonic.Unmarshal(body, &root); err != nil {
		return body
	}
	msgsRaw, ok := root["messages"].([]any)
	if !ok {
		return body
	}
	modified := false
	for i, msgRaw := range msgsRaw {
		msg, ok := msgRaw.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role != enum.RoleAssistant {
			continue
		}
		// 没有 tool_calls 直接跳过（上游只要求 tool call message 有该字段）
		if tcs, hasTC := msg["tool_calls"].([]any); !hasTC || len(tcs) == 0 {
			continue
		}
		// 已有非空 reasoning_content 则跳过；空串 / null 同样需要补位
		if rc, hasRC := msg["reasoning_content"]; hasRC {
			if s, isStr := rc.(string); isStr && s != "" {
				continue
			}
		}
		msg["reasoning_content"] = reasoningContentPlaceholder
		msgsRaw[i] = msg
		modified = true
	}
	if !modified {
		return body
	}
	root["messages"] = msgsRaw
	result, err := sonic.Marshal(root)
	if err != nil {
		return body
	}
	return result
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
			_, _ = humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
				Error: &dto.OpenAIError{
					Message: errMsg,
					Type:    "upstream_error",
					Code:    "upstream_error",
				},
			})))
		},
	}
}
