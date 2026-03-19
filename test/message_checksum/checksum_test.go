package message_checksum

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// testCase 测试用例的原始 JSON 结构（与 cases.json 对齐）
type testCase struct {
	Role             string          `json:"role"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        []*testToolCall `json:"tool_calls,omitempty"`
}

type testToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// loadCases 从 cases.json 加载测试用例
func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read cases.json: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal cases.json: %v", err)
	}
	return cases
}

// toUnifiedMessage 将测试用例转换为 UnifiedMessage
func toUnifiedMessage(t *testing.T, tc testCase) *dto.UnifiedMessage {
	t.Helper()
	msg := &dto.UnifiedMessage{
		Role:             tc.Role,
		ReasoningContent: tc.ReasoningContent,
	}
	if len(tc.ToolCalls) > 0 {
		msg.ToolCalls = make([]*dto.UnifiedToolCall, len(tc.ToolCalls))
		for i, call := range tc.ToolCalls {
			msg.ToolCalls[i] = &dto.UnifiedToolCall{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: call.Arguments,
			}
		}
	}
	return msg
}

func TestComputeMessageChecksum_DifferentKeyOrder(t *testing.T) {
	cases := loadCases(t)
	if len(cases) < 4 {
		t.Fatalf("expected at least 4 cases, got %d", len(cases))
	}

	t.Run("2-key arguments with different key order should produce same checksum", func(t *testing.T) {
		msg1 := toUnifiedMessage(t, cases[0])
		msg2 := toUnifiedMessage(t, cases[1])

		checksum1 := util.ComputeMessageChecksum(msg1)
		checksum2 := util.ComputeMessageChecksum(msg2)

		t.Logf("message1 arguments: %s", cases[0].ToolCalls[0].Arguments)
		t.Logf("message2 arguments: %s", cases[1].ToolCalls[0].Arguments)
		t.Logf("checksum1: %s", checksum1)
		t.Logf("checksum2: %s", checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeMessageChecksum() mismatch: message1=%s, message2=%s, want same checksum", checksum1, checksum2)
		}
	})

	t.Run("6-key arguments with different key order should produce same checksum", func(t *testing.T) {
		msg1 := toUnifiedMessage(t, cases[2])
		msg2 := toUnifiedMessage(t, cases[3])

		checksum1 := util.ComputeMessageChecksum(msg1)
		checksum2 := util.ComputeMessageChecksum(msg2)

		t.Logf("message1 arguments: %s", cases[2].ToolCalls[0].Arguments)
		t.Logf("message2 arguments: %s", cases[3].ToolCalls[0].Arguments)
		t.Logf("checksum1: %s", checksum1)
		t.Logf("checksum2: %s", checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeMessageChecksum() mismatch: message1=%s, message2=%s, want same checksum", checksum1, checksum2)
		}
	})
}

func TestComputeMessageChecksum_ToolCallIDIgnored(t *testing.T) {
	msg1 := &dto.UnifiedMessage{
		Role:             "assistant",
		ReasoningContent: "thinking",
		ToolCalls: []*dto.UnifiedToolCall{
			{ID: "call_001", Name: "Bash", Arguments: `{"command":"ls"}`},
		},
	}
	msg2 := &dto.UnifiedMessage{
		Role:             "assistant",
		ReasoningContent: "thinking",
		ToolCalls: []*dto.UnifiedToolCall{
			{ID: "call_999", Name: "Bash", Arguments: `{"command":"ls"}`},
		},
	}

	t.Run("different ToolCall IDs should produce same checksum", func(t *testing.T) {
		checksum1 := util.ComputeMessageChecksum(msg1)
		checksum2 := util.ComputeMessageChecksum(msg2)

		t.Logf("checksum1 (ID=call_001): %s", checksum1)
		t.Logf("checksum2 (ID=call_999): %s", checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeMessageChecksum() should ignore ToolCall ID: got %s and %s", checksum1, checksum2)
		}
	})
}

func TestComputeMessageChecksum_DifferentToolCallIDOnMessage(t *testing.T) {
	msg1 := &dto.UnifiedMessage{
		Role:       "tool",
		ToolCallID: "call_001",
		Content:    &dto.UnifiedContent{Text: "result"},
	}
	msg2 := &dto.UnifiedMessage{
		Role:       "tool",
		ToolCallID: "call_999",
		Content:    &dto.UnifiedContent{Text: "result"},
	}

	t.Run("different ToolCallID on tool result messages should produce same checksum", func(t *testing.T) {
		checksum1 := util.ComputeMessageChecksum(msg1)
		checksum2 := util.ComputeMessageChecksum(msg2)

		t.Logf("checksum1 (ToolCallID=call_001): %s", checksum1)
		t.Logf("checksum2 (ToolCallID=call_999): %s", checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeMessageChecksum() should ignore ToolCallID: got %s and %s", checksum1, checksum2)
		}
	})
}

func TestComputeMessageChecksum_DifferentMessages(t *testing.T) {
	msg1 := &dto.UnifiedMessage{
		Role:             "assistant",
		ReasoningContent: "thinking about A",
		ToolCalls: []*dto.UnifiedToolCall{
			{ID: "call_001", Name: "Bash", Arguments: `{"command":"ls"}`},
		},
	}
	msg2 := &dto.UnifiedMessage{
		Role:             "assistant",
		ReasoningContent: "thinking about B",
		ToolCalls: []*dto.UnifiedToolCall{
			{ID: "call_001", Name: "Bash", Arguments: `{"command":"pwd"}`},
		},
	}

	t.Run("semantically different messages should produce different checksums", func(t *testing.T) {
		checksum1 := util.ComputeMessageChecksum(msg1)
		checksum2 := util.ComputeMessageChecksum(msg2)

		t.Logf("checksum1: %s", checksum1)
		t.Logf("checksum2: %s", checksum2)

		if checksum1 == checksum2 {
			t.Errorf("ComputeMessageChecksum() should produce different checksums for different messages, both got %s", checksum1)
		}
	})
}

func TestComputeMessageChecksum_EmptyToolCalls(t *testing.T) {
	msg := &dto.UnifiedMessage{
		Role:    "assistant",
		Content: &dto.UnifiedContent{Text: "hello"},
	}

	t.Run("message without tool calls should compute checksum normally", func(t *testing.T) {
		checksum := util.ComputeMessageChecksum(msg)
		t.Logf("checksum: %s", checksum)

		if checksum == "" {
			t.Errorf("ComputeMessageChecksum() returned empty string")
		}
	})
}

func TestComputeMessageChecksum_MultipleToolCallsKeyOrder(t *testing.T) {
	msg1 := &dto.UnifiedMessage{
		Role: "assistant",
		ToolCalls: []*dto.UnifiedToolCall{
			{ID: "call_1", Name: "Search", Arguments: `{"query":"hello","limit":10}`},
			{ID: "call_2", Name: "Fetch", Arguments: `{"url":"https://example.com","method":"GET"}`},
		},
	}
	msg2 := &dto.UnifiedMessage{
		Role: "assistant",
		ToolCalls: []*dto.UnifiedToolCall{
			{ID: "call_1", Name: "Search", Arguments: `{"limit":10,"query":"hello"}`},
			{ID: "call_2", Name: "Fetch", Arguments: `{"method":"GET","url":"https://example.com"}`},
		},
	}

	t.Run("multiple tool calls with different JSON key order should produce same checksum", func(t *testing.T) {
		checksum1 := util.ComputeMessageChecksum(msg1)
		checksum2 := util.ComputeMessageChecksum(msg2)

		t.Logf("checksum1: %s", checksum1)
		t.Logf("checksum2: %s", checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeMessageChecksum() mismatch with multiple tool calls: got %s and %s", checksum1, checksum2)
		}
	})
}
