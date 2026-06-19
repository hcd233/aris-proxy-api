package compression

import (
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestOpenAIChatLocatorCompressToolOutput(t *testing.T) {
	t.Parallel()
	locator := &compression.OpenAIChatLocator{}
	dispatcher := compression.NewDispatcher()

	// 构造含 tool output 的 body
	body := map[string]any{
		"model": "gpt-4o",
		"messages": []any{
			map[string]any{"role": "system", "content": "You are a helpful assistant."},
			map[string]any{"role": "user", "content": "Search for errors"},
			map[string]any{"role": "assistant", "content": "Let me search."},
			map[string]any{
				"role":         "tool",
				"content":      `[{"name":"error","code":500,"msg":"database connection failed"},{"name":"warn","code":0,"msg":"ok"},{"name":"error","code":503,"msg":"timeout"},{"name":"info","code":200,"msg":"healthy"},{"name":"debug","code":100,"msg":"trace"}]`,
				"tool_call_id": "call_123",
			},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, stats := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody == nil {
		t.Fatal("expected compressed body, got nil")
	}
	if stats.ItemsCompressed == 0 {
		t.Error("expected at least 1 item compressed")
	}

	// 验证非 tool 消息未被修改
	var result map[string]any
	sonic.Unmarshal(newBody, &result)
	messages := result["messages"].([]any)
	sysMsg := messages[0].(map[string]any)
	if sysMsg["content"] != "You are a helpful assistant." {
		t.Error("system message should not be modified")
	}
}

func TestOpenAIChatLocatorNoToolOutput(t *testing.T) {
	t.Parallel()
	locator := &compression.OpenAIChatLocator{}
	dispatcher := compression.NewDispatcher()

	body := map[string]any{
		"model": "gpt-4o",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, _ := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody != nil {
		t.Error("body without tool output should return nil (no modification)")
	}
}

func TestOpenAIChatLocatorSmallToolOutputSkipped(t *testing.T) {
	t.Parallel()
	locator := &compression.OpenAIChatLocator{}
	dispatcher := compression.NewDispatcher()

	body := map[string]any{
		"model": "gpt-4o",
		"messages": []any{
			map[string]any{"role": "tool", "content": "ok"},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, stats := locator.LocateAndCompress(bodyBytes, dispatcher, 512)
	if newBody != nil {
		t.Error("small tool output should be skipped, no modification")
	}
	if stats.ItemsSkipped != 1 {
		t.Errorf("expected 1 skipped item, got %d", stats.ItemsSkipped)
	}
}
