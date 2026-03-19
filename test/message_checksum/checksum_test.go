package message_checksum

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// testCase represents raw JSON structure aligned with fixtures/cases.json
type testCase struct {
	Name             string          `json:"name"`
	Role             string          `json:"role"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        []*testToolCall `json:"tool_calls,omitempty"`
}

type testToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// loadCases loads test cases from fixtures/cases.json
func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

// findCase finds a test case by name, fatals if not found
func findCase(t *testing.T, cases []testCase, name string) testCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return testCase{}
}

// toUnifiedMessage converts a testCase to dto.UnifiedMessage
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

	tests := []struct {
		name  string
		caseA string
		caseB string
	}{
		{"2-key arguments with different key order", "2key_args_order_a", "2key_args_order_b"},
		{"6-key arguments with different key order", "6key_args_order_a", "6key_args_order_b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tcA := findCase(t, cases, tt.caseA)
			tcB := findCase(t, cases, tt.caseB)
			msgA := toUnifiedMessage(t, tcA)
			msgB := toUnifiedMessage(t, tcB)

			checksumA := util.ComputeMessageChecksum(msgA)
			checksumB := util.ComputeMessageChecksum(msgB)

			t.Logf("caseA=%s arguments: %s", tt.caseA, tcA.ToolCalls[0].Arguments)
			t.Logf("caseB=%s arguments: %s", tt.caseB, tcB.ToolCalls[0].Arguments)
			t.Logf("checksumA: %s, checksumB: %s", checksumA, checksumB)

			if checksumA != checksumB {
				t.Errorf("ComputeMessageChecksum() mismatch: caseA=%s, caseB=%s, want same checksum", checksumA, checksumB)
			}
		})
	}
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

	checksum1 := util.ComputeMessageChecksum(msg1)
	checksum2 := util.ComputeMessageChecksum(msg2)

	t.Logf("checksum1 (ID=call_001): %s", checksum1)
	t.Logf("checksum2 (ID=call_999): %s", checksum2)

	if checksum1 != checksum2 {
		t.Errorf("ComputeMessageChecksum() should ignore ToolCall ID: got %s and %s", checksum1, checksum2)
	}
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

	checksum1 := util.ComputeMessageChecksum(msg1)
	checksum2 := util.ComputeMessageChecksum(msg2)

	t.Logf("checksum1 (ToolCallID=call_001): %s", checksum1)
	t.Logf("checksum2 (ToolCallID=call_999): %s", checksum2)

	if checksum1 != checksum2 {
		t.Errorf("ComputeMessageChecksum() should ignore ToolCallID: got %s and %s", checksum1, checksum2)
	}
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

	checksum1 := util.ComputeMessageChecksum(msg1)
	checksum2 := util.ComputeMessageChecksum(msg2)

	t.Logf("checksum1: %s", checksum1)
	t.Logf("checksum2: %s", checksum2)

	if checksum1 == checksum2 {
		t.Errorf("ComputeMessageChecksum() should produce different checksums for different messages, both got %s", checksum1)
	}
}

func TestComputeMessageChecksum_EmptyToolCalls(t *testing.T) {
	msg := &dto.UnifiedMessage{
		Role:    "assistant",
		Content: &dto.UnifiedContent{Text: "hello"},
	}

	checksum := util.ComputeMessageChecksum(msg)
	t.Logf("checksum: %s", checksum)

	if checksum == "" {
		t.Errorf("ComputeMessageChecksum() returned empty string for message without tool calls")
	}
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

	checksum1 := util.ComputeMessageChecksum(msg1)
	checksum2 := util.ComputeMessageChecksum(msg2)

	t.Logf("checksum1: %s", checksum1)
	t.Logf("checksum2: %s", checksum2)

	if checksum1 != checksum2 {
		t.Errorf("ComputeMessageChecksum() mismatch with multiple tool calls: got %s and %s", checksum1, checksum2)
	}
}
