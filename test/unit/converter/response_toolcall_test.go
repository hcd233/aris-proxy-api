package converter

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// TestResponseProtocolConverter_ToResponseResponse_FunctionCall 验证非流式路径
// ChatCompletion 含工具调用时，转换为 Response API 输出应生成独立的 function_call item，
// 携带 type/call_id/name/arguments/status 字段，与 docs/openai/create_response.md 一致。
func TestResponseProtocolConverter_ToResponseResponse_FunctionCall(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}
	conv.SetToolTypeMap(map[string]string{
		"get_weather": enum.ResponseToolTypeFunction,
	})

	callID := "call_abc123"
	completion := &dto.OpenAIChatCompletion{
		ID:    "chatcmpl-1",
		Model: "gpt-test",
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Index: 0,
			Message: &dto.OpenAIChatCompletionMessageParam{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					ID:   lo.ToPtr(callID),
					Type: enum.ToolTypeFunction,
					Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
						Name:      "get_weather",
						Arguments: `{"location":"Boston, MA","unit":"celsius"}`,
					},
				}},
			},
			FinishReason: enum.FinishReasonToolCalls,
		}},
		Usage: &dto.OpenAICompletionUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}

	rsp, err := conv.ToResponseResponse(completion)
	if err != nil {
		t.Fatalf("ToResponseResponse() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("ToResponseResponse() returned nil")
	}

	var fcItem *dto.ResponseInputItem
	for _, item := range rsp.Output {
		if item != nil && item.Type != nil && *item.Type == enum.ResponseInputItemTypeFunctionCall {
			fcItem = item
			break
		}
	}
	if fcItem == nil {
		t.Fatalf("expected a function_call item in output, got %+v", rsp.Output)
	}

	if fcItem.CallID == nil || *fcItem.CallID != callID {
		t.Errorf("CallID = %v, want %q", fcItem.CallID, callID)
	}
	if fcItem.Name == nil || *fcItem.Name != "get_weather" {
		t.Errorf("Name = %v, want %q", fcItem.Name, "get_weather")
	}
	if fcItem.Arguments == nil || *fcItem.Arguments != `{"location":"Boston, MA","unit":"celsius"}` {
		t.Errorf("Arguments = %v, want the JSON string", fcItem.Arguments)
	}
}

// TestResponseProtocolConverter_ToResponseResponse_CustomToolCall 验证 custom tool call
// 转换为 custom_tool_call item。
func TestResponseProtocolConverter_ToResponseResponse_CustomToolCall(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}
	conv.SetToolTypeMap(map[string]string{
		"my_custom_tool": enum.ResponseToolTypeCustom,
	})

	callID := "call_custom1"
	completion := &dto.OpenAIChatCompletion{
		ID:    "chatcmpl-2",
		Model: "gpt-test",
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Index: 0,
			Message: &dto.OpenAIChatCompletionMessageParam{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					ID:   lo.ToPtr(callID),
					Type: enum.ToolTypeCustom,
					Custom: &dto.OpenAIChatCompletionMessageCustomToolCall{
						Name:  "my_custom_tool",
						Input: "some input text",
					},
				}},
			},
			FinishReason: enum.FinishReasonToolCalls,
		}},
	}

	rsp, err := conv.ToResponseResponse(completion)
	if err != nil {
		t.Fatalf("ToResponseResponse() error: %v", err)
	}

	var ctcItem *dto.ResponseInputItem
	for _, item := range rsp.Output {
		if item != nil && item.Type != nil && *item.Type == enum.ResponseInputItemTypeCustomToolCall {
			ctcItem = item
			break
		}
	}
	if ctcItem == nil {
		t.Fatalf("expected a custom_tool_call item in output, got %+v", rsp.Output)
	}

	if ctcItem.CallID == nil || *ctcItem.CallID != callID {
		t.Errorf("CallID = %v, want %q", ctcItem.CallID, callID)
	}
	if ctcItem.Name == nil || *ctcItem.Name != "my_custom_tool" {
		t.Errorf("Name = %v, want %q", ctcItem.Name, "my_custom_tool")
	}
	// custom tool call uses Input field, not Arguments
	if ctcItem.Input == nil || *ctcItem.Input != "some input text" {
		t.Errorf("Input = %v, want %q", ctcItem.Input, "some input text")
	}
}

// TestResponseProtocolConverter_ToResponseResponse_TextAndToolCall 验证同时有文本和工具调用时，
// 输出应包含 function_call item 和 message item（文本），顺序为 tool calls 先于 text。
func TestResponseProtocolConverter_ToResponseResponse_TextAndToolCall(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}
	conv.SetToolTypeMap(map[string]string{
		"get_weather": enum.ResponseToolTypeFunction,
	})

	completion := &dto.OpenAIChatCompletion{
		ID:    "chatcmpl-3",
		Model: "gpt-test",
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Index: 0,
			Message: &dto.OpenAIChatCompletionMessageParam{
				Role:    enum.RoleAssistant,
				Content: &dto.OpenAIMessageContent{Text: "Let me check the weather."},
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					ID:   lo.ToPtr("call_1"),
					Type: enum.ToolTypeFunction,
					Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
						Name:      "get_weather",
						Arguments: `{"location":"NYC"}`,
					},
				}},
			},
			FinishReason: enum.FinishReasonToolCalls,
		}},
	}

	rsp, err := conv.ToResponseResponse(completion)
	if err != nil {
		t.Fatalf("ToResponseResponse() error: %v", err)
	}

	if len(rsp.Output) < 2 {
		t.Fatalf("expected at least 2 output items (function_call + message), got %d", len(rsp.Output))
	}

	// 第一个应该是 function_call
	first := rsp.Output[0]
	if first.Type == nil || *first.Type != enum.ResponseInputItemTypeFunctionCall {
		t.Errorf("first output item type = %v, want %q", first.Type, enum.ResponseInputItemTypeFunctionCall)
	}

	// 应该有一个 message item 包含文本
	var msgItem *dto.ResponseInputItem
	for _, item := range rsp.Output {
		if item != nil && item.Type != nil && *item.Type == enum.ResponseInputItemTypeMessage {
			msgItem = item
			break
		}
	}
	if msgItem == nil {
		t.Fatal("expected a message item with text content, not found")
	}
}
