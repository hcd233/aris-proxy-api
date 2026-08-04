package proxyutil

import (
	"cmp"
	"strings"

	"github.com/bytedance/sonic"
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
	if v := lo.FromPtr(choice.Delta.Content); v != "" {
		cs.contentParts = append(cs.contentParts, v)
	}
	if v := lo.FromPtr(choice.Delta.ReasoningContent); v != "" {
		cs.reasoningContentParts = append(cs.reasoningContentParts, v)
	}
	if v := lo.FromPtr(choice.Delta.Refusal); v != "" {
		cs.refusalParts = append(cs.refusalParts, v)
	}
	for _, tc := range choice.Delta.ToolCalls {
		cs.mergeToolCallDelta(tc)
	}
	if v := lo.FromPtr(choice.FinishReason); v != "" {
		cs.finishReason = v
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

// ChatCompletionStreamAggregator 流式增量聚合 OpenAI Chat Completion chunk。
//
// 与 ConcatChatCompletionChunks 语义一致，但无需驻留全部 chunk：
// 每收到一个 chunk 调用 Add，流结束后调用 Completion 取聚合结果。
type ChatCompletionStreamAggregator struct {
	cmpl        *dto.OpenAIChatCompletion
	choiceMap   map[int]*choiceState
	choiceOrder []int
	count       int
}

// NewChatCompletionStreamAggregator 创建流式聚合器
func NewChatCompletionStreamAggregator() *ChatCompletionStreamAggregator {
	return &ChatCompletionStreamAggregator{
		cmpl:      &dto.OpenAIChatCompletion{},
		choiceMap: make(map[int]*choiceState),
	}
}

// Add 增量合并一个 chunk（nil chunk 忽略）。
func (a *ChatCompletionStreamAggregator) Add(chunk *dto.OpenAIChatCompletionChunk) {
	if chunk == nil {
		return
	}
	a.count++

	if a.cmpl.ID == "" {
		a.cmpl.ID = chunk.ID
		a.cmpl.Created = chunk.Created
		a.cmpl.Object = chunk.Object
		a.cmpl.ServiceTier = chunk.ServiceTier
		a.cmpl.SystemFingerprint = chunk.SystemFingerprint
		a.cmpl.Model = chunk.Model
	}

	if chunk.Usage != nil {
		a.cmpl.Usage = chunk.Usage
	}

	for _, choice := range chunk.Choices {
		cs, exists := a.choiceMap[choice.Index]
		if !exists {
			cs = &choiceState{
				index:       choice.Index,
				toolCallMap: make(map[int]*toolCallState),
			}
			a.choiceMap[choice.Index] = cs
			a.choiceOrder = append(a.choiceOrder, choice.Index)
		}
		cs.mergeDelta(choice)
	}
}

// Count 返回已聚合的非 nil chunk 数。
func (a *ChatCompletionStreamAggregator) Count() int {
	return a.count
}

// Completion 输出聚合结果。
func (a *ChatCompletionStreamAggregator) Completion() *dto.OpenAIChatCompletion {
	if a.count == 0 {
		return a.cmpl
	}
	a.cmpl.Choices = lo.Map(a.choiceOrder, func(idx int, _ int) *dto.OpenAIChatCompletionChoice {
		return buildChoice(a.choiceMap[idx])
	})
	return a.cmpl
}

func ConcatChatCompletionChunks(chunks []*dto.OpenAIChatCompletionChunk) (*dto.OpenAIChatCompletion, error) {
	agg := NewChatCompletionStreamAggregator()
	for _, chunk := range chunks {
		agg.Add(chunk)
	}
	return agg.Completion(), nil
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
