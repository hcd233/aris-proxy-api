// Package stream_aggregator 验证流式增量聚合器与批量 Concat 函数的结果一致。
//
// 回归保护：
//   - ChatCompletionStreamAggregator / AnthropicSSEStreamAggregator 用于在 SSE 流式转发中
//     增量聚合 chunk/event，替代全量切片驻留内存。
//   - 增量聚合结果必须与 ConcatChatCompletionChunks / ConcatAnthropicSSEEvents 完全一致。
package stream_aggregator

import (
	"reflect"
	"testing"

	"github.com/bytedance/sonic"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

func chatStreamChunks() []*dto.OpenAIChatCompletionChunk {
	return []*dto.OpenAIChatCompletionChunk{
		{
			ID:      "chatcmpl-1",
			Created: 1700000000,
			Model:   "gpt-x",
			Object:  "chat.completion.chunk",
			Choices: []*dto.OpenAIChatCompletionChunkChoice{
				{Index: 0, Delta: &dto.OpenAIChatCompletionChunkDelta{Role: enum.RoleAssistant, Content: lo.ToPtr("Hel")}},
			},
		},
		{
			ID:      "chatcmpl-1",
			Created: 1700000000,
			Model:   "gpt-x",
			Object:  "chat.completion.chunk",
			Choices: []*dto.OpenAIChatCompletionChunkChoice{
				{Index: 0, Delta: &dto.OpenAIChatCompletionChunkDelta{
					Content: lo.ToPtr("lo"),
					ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{
						{Index: lo.ToPtr(0), ID: lo.ToPtr("call_1"), Type: enum.ToolTypeFunction, Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{Name: "get_"}},
					},
				}},
			},
		},
		{
			ID:      "chatcmpl-1",
			Created: 1700000000,
			Model:   "gpt-x",
			Object:  "chat.completion.chunk",
			Choices: []*dto.OpenAIChatCompletionChunkChoice{
				{Index: 0, Delta: &dto.OpenAIChatCompletionChunkDelta{
					ReasoningContent: lo.ToPtr("think-1"),
					ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{
						{Index: lo.ToPtr(0), Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{Name: "time", Arguments: `{"tz"`}},
					},
				}},
				{Index: 1, Delta: &dto.OpenAIChatCompletionChunkDelta{Content: lo.ToPtr("alt")}},
			},
		},
		{
			ID:      "chatcmpl-1",
			Created: 1700000000,
			Model:   "gpt-x",
			Object:  "chat.completion.chunk",
			Choices: []*dto.OpenAIChatCompletionChunkChoice{
				{Index: 0, Delta: &dto.OpenAIChatCompletionChunkDelta{
					ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{
						{Index: lo.ToPtr(0), Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{Arguments: `":"UTC"}`}},
					},
				}, FinishReason: lo.ToPtr(enum.FinishReasonToolCalls)},
			},
			Usage: &dto.OpenAICompletionUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
	}
}

func TestChatCompletionStreamAggregator_MatchesConcat(t *testing.T) {
	t.Parallel()
	chunks := chatStreamChunks()

	agg := proxyutil.NewChatCompletionStreamAggregator()
	for _, chunk := range chunks {
		agg.Add(chunk)
	}
	got := agg.Completion()

	want, err := proxyutil.ConcatChatCompletionChunks(chunks)
	if err != nil {
		t.Fatalf("ConcatChatCompletionChunks unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := sonic.MarshalString(got)
		wantJSON, _ := sonic.MarshalString(want)
		t.Errorf("incremental aggregation mismatch:\n  got:  %s\n  want: %s", gotJSON, wantJSON)
	}
	if agg.Count() != len(chunks) {
		t.Errorf("Count() = %d, want %d", agg.Count(), len(chunks))
	}
}

func TestChatCompletionStreamAggregator_EmptyAndNilChunk(t *testing.T) {
	t.Parallel()

	agg := proxyutil.NewChatCompletionStreamAggregator()
	agg.Add(nil)
	if agg.Count() != 0 {
		t.Errorf("Count() after Add(nil) = %d, want 0", agg.Count())
	}

	got := agg.Completion()
	want, err := proxyutil.ConcatChatCompletionChunks(nil)
	if err != nil {
		t.Fatalf("ConcatChatCompletionChunks(nil) unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("empty aggregation mismatch:\n  got:  %+v\n  want: %+v", got, want)
	}
}

func anthropicStreamEvents() []dto.AnthropicSSEEvent {
	return []dto.AnthropicSSEEvent{
		{Event: enum.AnthropicSSEEventTypeMessageStart, Data: []byte(`{"message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-x","usage":{"input_tokens":10,"output_tokens":1}}}`)},
		{Event: enum.AnthropicSSEEventTypeContentBlockStart, Data: []byte(`{"index":0,"content_block":{"type":"text","text":""}}`)},
		{Event: enum.AnthropicSSEEventTypeContentBlockDelta, Data: []byte(`{"index":0,"delta":{"type":"text_delta","text":"Hel"}}`)},
		{Event: enum.AnthropicSSEEventTypeContentBlockDelta, Data: []byte(`{"index":0,"delta":{"type":"text_delta","text":"lo"}}`)},
		{Event: enum.AnthropicSSEEventTypeContentBlockStop, Data: []byte(`{"index":0}`)},
		{Event: enum.AnthropicSSEEventTypeMessageDelta, Data: []byte(`{"delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`)},
		{Event: enum.AnthropicSSEEventTypeMessageStop, Data: []byte(`{"type":"message_stop"}`)},
	}
}

func TestAnthropicSSEStreamAggregator_MatchesConcat(t *testing.T) {
	t.Parallel()
	events := anthropicStreamEvents()

	agg := proxyutil.NewAnthropicSSEStreamAggregator()
	for _, event := range events {
		if err := agg.Add(event); err != nil {
			t.Fatalf("Add unexpected error: %v", err)
		}
	}
	got, err := agg.Message()
	if err != nil {
		t.Fatalf("Message unexpected error: %v", err)
	}

	want, err := proxyutil.ConcatAnthropicSSEEvents(events)
	if err != nil {
		t.Fatalf("ConcatAnthropicSSEEvents unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := sonic.MarshalString(got)
		wantJSON, _ := sonic.MarshalString(want)
		t.Errorf("incremental aggregation mismatch:\n  got:  %s\n  want: %s", gotJSON, wantJSON)
	}
	if agg.Count() != len(events) {
		t.Errorf("Count() = %d, want %d", agg.Count(), len(events))
	}
}

func TestAnthropicSSEStreamAggregator_ErrorPropagation(t *testing.T) {
	t.Parallel()

	bad := dto.AnthropicSSEEvent{Event: enum.AnthropicSSEEventTypeContentBlockDelta, Data: []byte(`{invalid json`)}

	agg := proxyutil.NewAnthropicSSEStreamAggregator()
	if err := agg.Add(bad); err == nil {
		t.Fatalf("Add should return parse error")
	}
	if agg.Count() != 0 {
		t.Errorf("Count() after failed Add = %d, want 0", agg.Count())
	}

	if _, err := proxyutil.ConcatAnthropicSSEEvents([]dto.AnthropicSSEEvent{bad}); err == nil {
		t.Fatalf("ConcatAnthropicSSEEvents should return parse error")
	}
}
