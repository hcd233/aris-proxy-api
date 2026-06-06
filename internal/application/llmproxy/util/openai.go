package proxyutil

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

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
	toolCallMap           map[int]*toolCallState
	toolCallOrder         []int
	finishReason          enum.FinishReason
	logprobs              *dto.OpenAILogprobs
	index                 int
}

func (cs *choiceState) mergeToolCallDelta(tc *dto.OpenAIChatCompletionMessageToolCall) {
	tcIdx := cs.index
	if tc.Index != nil {
		tcIdx = *tc.Index
	}
	tcs, ok := cs.toolCallMap[tcIdx]
	if !ok {
		tcs = &toolCallState{}
		cs.toolCallMap[tcIdx] = tcs
		cs.toolCallOrder = append(cs.toolCallOrder, tcIdx)
	}
	if lo.FromPtr(tc.ID) != "" {
		tcs.id = lo.FromPtr(tc.ID)
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

func (cs *choiceState) mergeDelta(choice *dto.OpenAIChatCompletionChunkChoice) {
	if cs.role == "" && choice.Delta.Role != "" {
		cs.role = choice.Delta.Role
	}
	if choice.Delta.Content != nil && *choice.Delta.Content != "" {
		cs.contentParts = append(cs.contentParts, *choice.Delta.Content)
	}
	if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
		cs.reasoningContentParts = append(cs.reasoningContentParts, *choice.Delta.ReasoningContent)
	}
	if choice.Delta.Refusal != nil && *choice.Delta.Refusal != "" {
		cs.refusalParts = append(cs.refusalParts, *choice.Delta.Refusal)
	}
	for _, tc := range choice.Delta.ToolCalls {
		cs.mergeToolCallDelta(tc)
	}
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		cs.finishReason = *choice.FinishReason
	}
	if choice.Logprobs != nil {
		if cs.logprobs == nil {
			cs.logprobs = &dto.OpenAILogprobs{}
		}
		cs.logprobs.Content = append(cs.logprobs.Content, choice.Logprobs.Content...)
		cs.logprobs.Refusal = append(cs.logprobs.Refusal, choice.Logprobs.Refusal...)
	}
}

func buildMergedToolCalls(cs *choiceState) []*dto.OpenAIChatCompletionMessageToolCall {
	var mergedToolCalls []*dto.OpenAIChatCompletionMessageToolCall
	for _, tcIdx := range cs.toolCallOrder {
		tcs := cs.toolCallMap[tcIdx]
		id := tcs.id
		tc := &dto.OpenAIChatCompletionMessageToolCall{
			ID:   &id,
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
	return mergedToolCalls
}

func buildChoice(cs *choiceState) *dto.OpenAIChatCompletionChoice {
	var content *dto.OpenAIMessageContent
	if joined := strings.Join(cs.contentParts, ""); joined != "" {
		content = &dto.OpenAIMessageContent{Text: joined}
	}
	reasoningContent := strings.Join(cs.reasoningContentParts, "")
	refusal := strings.Join(cs.refusalParts, "")
	message := &dto.OpenAIChatCompletionMessageParam{
		Role:             cmp.Or(cs.role, enum.RoleAssistant),
		Content:          content,
		ReasoningContent: &reasoningContent,
		Refusal:          &refusal,
		ToolCalls:        buildMergedToolCalls(cs),
	}
	return &dto.OpenAIChatCompletionChoice{
		Index:        cs.index,
		Message:      message,
		FinishReason: cs.finishReason,
		Logprobs:     cs.logprobs,
	}
}

func ConcatChatCompletionChunks(chunks []*dto.OpenAIChatCompletionChunk) (*dto.OpenAIChatCompletion, error) {
	cmpl := &dto.OpenAIChatCompletion{}

	if len(chunks) == 0 {
		return cmpl, nil
	}

	choiceMap := make(map[int]*choiceState)
	choiceOrder := make([]int, 0)

	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}

		if cmpl.ID == "" {
			cmpl.ID = chunk.ID
			cmpl.Created = chunk.Created
			cmpl.Object = chunk.Object
			cmpl.ServiceTier = chunk.ServiceTier
			cmpl.SystemFingerprint = chunk.SystemFingerprint
			cmpl.Model = chunk.Model
		}

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
			cs.mergeDelta(choice)
		}
	}

	cmpl.Choices = make([]*dto.OpenAIChatCompletionChoice, 0, len(choiceOrder))
	for _, idx := range choiceOrder {
		cmpl.Choices = append(cmpl.Choices, buildChoice(choiceMap[idx]))
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

// FillResponseTerminalOutput patches a terminal Response API SSE payload when
// the upstream terminal response omits output but earlier output_item.done
// events already carried complete output items.
func FillResponseTerminalOutput(data []byte, accumulatedOutput []*dto.ResponseInputItem) (patched []byte, changed bool, err error) {
	if len(accumulatedOutput) == 0 {
		return data, false, nil
	}
	var ev dto.ResponseStreamTerminalEvent
	if err := sonic.Unmarshal(data, &ev); err != nil {
		return nil, false, err
	}
	if ev.Response == nil || len(ev.Response.Output) > 0 {
		return data, false, nil
	}
	ev.Response.Output = accumulatedOutput
	patched, err = sonic.Marshal(&ev)
	if err != nil {
		return nil, false, err
	}
	return patched, true, nil
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
		errMsg = fmt.Sprintf(constant.UpstreamStatusMessageTemplate, statusCode)
	}

	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			humaCtx.SetStatus(statusCode)
			humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
			_, _ = humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{ //nolint:errcheck // best-effort write in error handler
				Error: &dto.OpenAIError{
					Message: errMsg,
					Type:    constant.UpstreamErrorType,
					Code:    constant.UpstreamErrorType,
				},
			})))
		},
	}
}
