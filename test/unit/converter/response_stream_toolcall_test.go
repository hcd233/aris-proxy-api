package converter

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	convapi "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// parseSSEEvents 将 SSE 输出解析为 (event, data) 对列表
func parseSSEEvents(t *testing.T, raw string) []struct {
	Event string
	Data  map[string]any
} {
	t.Helper()
	var events []struct {
		Event string
		Data  map[string]any
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			ev := strings.TrimPrefix(line, "event: ")
			events = append(events, struct {
				Event string
				Data  map[string]any
			}{Event: ev})
		}
		if strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			var data map[string]any
			if err := sonic.UnmarshalString(dataStr, &data); err != nil {
				t.Fatalf("failed to parse SSE data: %v\nraw: %s", err, dataStr)
			}
			if len(events) > 0 {
				events[len(events)-1].Data = data
			}
		}
	}
	return events
}

// findEvents 查找所有指定类型的事件
func findEvents(events []struct {
	Event string
	Data  map[string]any
}, eventType string) []map[string]any {
	var result []map[string]any
	for _, e := range events {
		if e.Event == eventType {
			result = append(result, e.Data)
		}
	}
	return result
}

// TestStreamingToolCall_FunctionCall_OutputItemAdded 验证流式工具调用时，
// 应为 function_call 创建独立的 output_item.added 事件，item.type 为 "function_call"，
// 携带 call_id 和 name。
func TestStreamingToolCall_FunctionCall_OutputItemAdded(t *testing.T) {
	t.Parallel()
	conv := &convapi.ResponseProtocolConverter{}
	conv.SetToolTypeMap(map[string]string{
		"get_weather": enum.ResponseToolTypeFunction,
	})

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	state := convapi.NewStreamItemState()

	chunk := &dto.OpenAIChatCompletionChunk{
		ID:    "chatcmpl-1",
		Model: "test",
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					Index: lo.ToPtr(0),
					ID:    lo.ToPtr("call_abc"),
					Type:  enum.ToolTypeFunction,
					Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
						Name: "get_weather",
					},
				}},
			},
		}},
	}

	if _, err := convapi.WriteResponseDeltaFromChatChunk(w, chunk, state, "resp_123", conv); err != nil {
		t.Fatalf("WriteResponseDeltaFromChatChunk() error: %v", err)
	}
	_ = w.Flush()

	events := parseSSEEvents(t, buf.String())

	// 应该有一个 output_item.added 事件，item.type = "function_call"
	addedEvents := findEvents(events, "response.output_item.added")
	if len(addedEvents) == 0 {
		t.Fatal("expected at least one response.output_item.added event, got none")
	}

	// 找到 function_call 类型的 added 事件
	var fcAdded map[string]any
	for _, ev := range addedEvents {
		if item, ok := ev["item"].(map[string]any); ok {
			if itemType, _ := item["type"].(string); itemType == "function_call" {
				fcAdded = ev
				break
			}
		}
	}
	if fcAdded == nil {
		t.Fatalf("no output_item.added event with item.type=function_call found; got: %+v", addedEvents)
	}

	item := fcAdded["item"].(map[string]any)
	if item["call_id"] != "call_abc" {
		t.Errorf("item.call_id = %v, want %q", item["call_id"], "call_abc")
	}
	if item["name"] != "get_weather" {
		t.Errorf("item.name = %v, want %q", item["name"], "get_weather")
	}
	if item["type"] != "function_call" {
		t.Errorf("item.type = %v, want %q", item["type"], "function_call")
	}
}

// TestStreamingToolCall_FunctionCall_ArgumentsDelta 验证函数调用参数增量事件
// 使用独立的 item_id（function_call 的 ID，不是 message 的 ID）。
func TestStreamingToolCall_FunctionCall_ArgumentsDelta(t *testing.T) {
	t.Parallel()
	conv := &convapi.ResponseProtocolConverter{}
	conv.SetToolTypeMap(map[string]string{
		"get_weather": enum.ResponseToolTypeFunction,
	})

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	state := convapi.NewStreamItemState()

	// chunk 1: 函数名 + ID
	chunk1 := &dto.OpenAIChatCompletionChunk{
		ID:    "chatcmpl-1",
		Model: "test",
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					Index: lo.ToPtr(0),
					ID:    lo.ToPtr("call_abc"),
					Type:  enum.ToolTypeFunction,
					Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
						Name: "get_weather",
					},
				}},
			},
		}},
	}
	// chunk 2: 参数增量
	chunk2 := &dto.OpenAIChatCompletionChunk{
		ID:    "chatcmpl-1",
		Model: "test",
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					Index: lo.ToPtr(0),
					Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
						Arguments: `{"location":"Boston"`,
					},
				}},
			},
		}},
	}

	if _, err := convapi.WriteResponseDeltaFromChatChunk(w, chunk1, state, "resp_123", conv); err != nil {
		t.Fatalf("chunk1 error: %v", err)
	}
	if _, err := convapi.WriteResponseDeltaFromChatChunk(w, chunk2, state, "resp_123", conv); err != nil {
		t.Fatalf("chunk2 error: %v", err)
	}
	_ = w.Flush()

	events := parseSSEEvents(t, buf.String())
	argDeltas := findEvents(events, "response.function_call_arguments.delta")
	if len(argDeltas) == 0 {
		t.Fatal("expected response.function_call_arguments.delta events, got none")
	}

	// item_id 应该是 function_call 的 ID，不是 message 的 ID
	firstDelta := argDeltas[0]
	itemID, _ := firstDelta["item_id"].(string)
	if !strings.HasPrefix(itemID, "fc_") {
		t.Errorf("arguments.delta item_id = %q, should start with 'fc_' (function_call item ID)", itemID)
	}

	// delta 应包含参数片段
	delta, _ := firstDelta["delta"].(string)
	if delta != `{"location":"Boston"` {
		t.Errorf("delta = %q, want %q", delta, `{"location":"Boston"`)
	}
}

// TestStreamingToolCall_FunctionCall_OutputItemDone 验证 finalize 时
// 为 function_call 发送 output_item.done 事件，携带完整的 arguments。
func TestStreamingToolCall_FunctionCall_OutputItemDone(t *testing.T) {
	t.Parallel()
	conv := &convapi.ResponseProtocolConverter{}
	conv.SetToolTypeMap(map[string]string{
		"get_weather": enum.ResponseToolTypeFunction,
	})

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	state := convapi.NewStreamItemState()

	// 完整的函数调用 chunk
	chunk := &dto.OpenAIChatCompletionChunk{
		ID:    "chatcmpl-1",
		Model: "test",
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					Index: lo.ToPtr(0),
					ID:    lo.ToPtr("call_abc"),
					Type:  enum.ToolTypeFunction,
					Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
						Name:      "get_weather",
						Arguments: `{"location":"Boston"}`,
					},
				}},
			},
		}},
	}

	if _, err := convapi.WriteResponseDeltaFromChatChunk(w, chunk, state, "resp_123", conv); err != nil {
		t.Fatalf("chunk error: %v", err)
	}

	// finalize
	completion := &dto.OpenAIChatCompletion{
		ID:    "chatcmpl-1",
		Model: "test",
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Index: 0,
			Message: &dto.OpenAIChatCompletionMessageParam{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					ID:   lo.ToPtr("call_abc"),
					Type: enum.ToolTypeFunction,
					Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
						Name:      "get_weather",
						Arguments: `{"location":"Boston"}`,
					},
				}},
			},
			FinishReason: enum.FinishReasonToolCalls,
		}},
	}

	rsp := convapi.FinalizeResponseFromChatCompletion(w, completion, "test", "resp_123", conv)
	_ = w.Flush()

	if rsp == nil {
		t.Fatal("FinalizeResponseFromChatCompletion returned nil")
	}

	events := parseSSEEvents(t, buf.String())
	doneEvents := findEvents(events, "response.output_item.done")

	// 应该有一个 function_call 类型的 done 事件
	var fcDone map[string]any
	for _, ev := range doneEvents {
		if item, ok := ev["item"].(map[string]any); ok {
			if itemType, _ := item["type"].(string); itemType == "function_call" {
				fcDone = ev
				break
			}
		}
	}
	if fcDone == nil {
		t.Fatalf("no output_item.done event with item.type=function_call found; got events: %+v", doneEvents)
	}

	item := fcDone["item"].(map[string]any)
	if item["call_id"] != "call_abc" {
		t.Errorf("item.call_id = %v, want %q", item["call_id"], "call_abc")
	}
	if item["name"] != "get_weather" {
		t.Errorf("item.name = %v, want %q", item["name"], "get_weather")
	}
	if item["arguments"] != `{"location":"Boston"}` {
		t.Errorf("item.arguments = %v, want %q", item["arguments"], `{"location":"Boston"}`)
	}
}

// TestStreamingToolCall_CustomToolCall_OutputItemAdded 验证 custom tool call
// 流式转换生成 custom_tool_call 类型的 output item。
func TestStreamingToolCall_CustomToolCall_OutputItemAdded(t *testing.T) {
	t.Parallel()
	conv := &convapi.ResponseProtocolConverter{}
	conv.SetToolTypeMap(map[string]string{
		"my_custom": enum.ResponseToolTypeCustom,
	})

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	state := convapi.NewStreamItemState()

	chunk := &dto.OpenAIChatCompletionChunk{
		ID:    "chatcmpl-1",
		Model: "test",
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					Index: lo.ToPtr(0),
					ID:    lo.ToPtr("call_custom1"),
					Type:  enum.ToolTypeCustom,
					Custom: &dto.OpenAIChatCompletionMessageCustomToolCall{
						Name: "my_custom",
					},
				}},
			},
		}},
	}

	if _, err := convapi.WriteResponseDeltaFromChatChunk(w, chunk, state, "resp_123", conv); err != nil {
		t.Fatalf("WriteResponseDeltaFromChatChunk() error: %v", err)
	}
	_ = w.Flush()

	events := parseSSEEvents(t, buf.String())
	addedEvents := findEvents(events, "response.output_item.added")

	var ctcAdded map[string]any
	for _, ev := range addedEvents {
		if item, ok := ev["item"].(map[string]any); ok {
			if itemType, _ := item["type"].(string); itemType == "custom_tool_call" {
				ctcAdded = ev
				break
			}
		}
	}
	if ctcAdded == nil {
		t.Fatalf("no output_item.added event with item.type=custom_tool_call found; got: %+v", addedEvents)
	}

	item := ctcAdded["item"].(map[string]any)
	if item["call_id"] != "call_custom1" {
		t.Errorf("item.call_id = %v, want %q", item["call_id"], "call_custom1")
	}
	if item["name"] != "my_custom" {
		t.Errorf("item.name = %v, want %q", item["name"], "my_custom")
	}
}
