package compression

import (
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestAnthropicLocatorStringContent(t *testing.T) {
	t.Parallel()
	locator := &compression.AnthropicMessagesLocator{}
	dispatcher := compression.NewDispatcher()

	body := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "Check these errors"},
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_123",
						"content":     `[{"name":"error","code":500},{"name":"warn","code":0},{"name":"error","code":503},{"name":"info","code":200}]`,
					},
				},
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

	// 验证 text block 未被修改
	var result map[string]any
	sonic.Unmarshal(newBody, &result)
	messages := result["messages"].([]any)
	msg := messages[0].(map[string]any)
	blocks := msg["content"].([]any)
	textBlock := blocks[0].(map[string]any)
	if textBlock["text"] != "Check these errors" {
		t.Error("text block should not be modified")
	}
}

func TestAnthropicLocatorArrayContent(t *testing.T) {
	t.Parallel()
	locator := &compression.AnthropicMessagesLocator{}
	dispatcher := compression.NewDispatcher()

	body := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_456",
						"content": []any{
							map[string]any{"type": "text", "text": `[{"name":"error","code":500,"msg":"db failed"},{"name":"warn","code":0,"msg":"ok"},{"name":"error","code":503,"msg":"timeout"},{"name":"info","code":200,"msg":"healthy"},{"name":"debug","code":100,"msg":"trace"}]`},
						},
					},
				},
			},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, stats := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody == nil {
		t.Fatal("expected compressed body for array content")
	}
	if stats.ItemsCompressed == 0 {
		t.Error("expected at least 1 item compressed")
	}
}

func TestAnthropicLocatorNoToolResult(t *testing.T) {
	t.Parallel()
	locator := &compression.AnthropicMessagesLocator{}
	dispatcher := compression.NewDispatcher()

	body := map[string]any{
		"model": "claude-sonnet-4-5-20250929",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
				},
			},
		},
	}
	bodyBytes, _ := sonic.Marshal(body)

	newBody, _ := locator.LocateAndCompress(bodyBytes, dispatcher, 10)
	if newBody != nil {
		t.Error("body without tool_result should return nil")
	}
}
