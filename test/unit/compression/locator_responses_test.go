package compression

import (
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestResponsesLocatorCompressFunctionCallOutput(t *testing.T) {
	t.Parallel()
	locator := &compression.OpenAIResponsesLocator{}
	dispatcher := compression.NewDispatcher()

	body := map[string]any{
		"model": "gpt-4o",
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "Search for errors"},
			map[string]any{"type": "function_call", "name": "search", "arguments": "{}"},
			map[string]any{
				"type":    "function_call_output",
				"call_id": "call_123",
				"output":  `[{"name":"error","code":500,"msg":"database connection failed"},{"name":"warn","code":0,"msg":"ok"},{"name":"error","code":503,"msg":"timeout"},{"name":"info","code":200,"msg":"healthy"},{"name":"debug","code":100,"msg":"trace"}]`,
			},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, stats := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody == nil {
		t.Fatal("expected compressed body")
	}
	if stats.ItemsCompressed == 0 {
		t.Error("expected at least 1 item compressed")
	}

	// 验证 message item 未被修改
	var result map[string]any
	sonic.Unmarshal(newBody, &result)
	input := result["input"].([]any)
	msgItem := input[0].(map[string]any)
	if msgItem["content"] != "Search for errors" {
		t.Error("message item should not be modified")
	}
}

func TestResponsesLocatorNoFunctionCallOutput(t *testing.T) {
	t.Parallel()
	locator := &compression.OpenAIResponsesLocator{}
	dispatcher := compression.NewDispatcher()

	body := map[string]any{
		"model": "gpt-4o",
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "hello"},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, _ := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody != nil {
		t.Error("body without function_call_output should return nil")
	}
}
