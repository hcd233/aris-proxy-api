package message_checksum

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
)

// testCase represents raw JSON structure aligned with fixtures/cases.json
type testCase struct {
	Name             string          `json:"name"`
	Role             string          `json:"role"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	Content          string          `json:"content,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
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

// loadSchemaAwareCases loads test cases from fixtures/schema_aware_cases.json
func loadSchemaAwareCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/schema_aware_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/schema_aware_cases.json: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/schema_aware_cases.json: %v", err)
	}
	return cases
}

// loadToolSchemas loads tool schemas from fixtures/tool_schemas.json
func loadToolSchemas(t *testing.T) vo.ToolSchemaMap {
	t.Helper()
	data, err := os.ReadFile("./fixtures/tool_schemas.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/tool_schemas.json: %v", err)
	}
	var schemas map[string]*vo.JSONSchemaProperty
	if err := sonic.Unmarshal(data, &schemas); err != nil {
		t.Fatalf("failed to unmarshal fixtures/tool_schemas.json: %v", err)
	}
	return vo.ToolSchemaMap(schemas)
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

// toUnifiedMessage converts a testCase to vo.UnifiedMessage
func toUnifiedMessage(t *testing.T, tc testCase) *vo.UnifiedMessage {
	t.Helper()
	msg := &vo.UnifiedMessage{
		Role:             tc.Role,
		ReasoningContent: tc.ReasoningContent,
		ToolCallID:       tc.ToolCallID,
	}
	if tc.Content != "" {
		msg.Content = &vo.UnifiedContent{Text: tc.Content}
	}
	if len(tc.ToolCalls) > 0 {
		msg.ToolCalls = make([]*vo.UnifiedToolCall, len(tc.ToolCalls))
		for i, call := range tc.ToolCalls {
			msg.ToolCalls[i] = &vo.UnifiedToolCall{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: call.Arguments,
			}
		}
	}
	return msg
}

// ==================== Original Tests (nil schema) ====================

func TestComputeMessageChecksum_DifferentKeyOrder(t *testing.T) {
	t.Parallel()
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tcA := findCase(t, cases, tt.caseA)
			tcB := findCase(t, cases, tt.caseB)
			msgA := toUnifiedMessage(t, tcA)
			msgB := toUnifiedMessage(t, tcB)

			checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
			checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

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
	t.Parallel()
	cases := loadCases(t)

	tcA := findCase(t, cases, "tool_call_id_ignored_a")
	tcB := findCase(t, cases, "tool_call_id_ignored_b")
	msgA := toUnifiedMessage(t, tcA)
	msgB := toUnifiedMessage(t, tcB)

	checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
	checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

	t.Logf("checksumA (ID=call_001): %s", checksumA)
	t.Logf("checksumB (ID=call_999): %s", checksumB)

	if checksumA != checksumB {
		t.Errorf("ComputeMessageChecksum() should ignore ToolCall ID: got %s and %s", checksumA, checksumB)
	}
}

func TestComputeMessageChecksum_DifferentToolCallIDOnMessage(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)

	tcA := findCase(t, cases, "different_tool_call_id_on_message_a")
	tcB := findCase(t, cases, "different_tool_call_id_on_message_b")
	msgA := toUnifiedMessage(t, tcA)
	msgB := toUnifiedMessage(t, tcB)

	checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
	checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

	t.Logf("checksumA (ToolCallID=call_001): %s", checksumA)
	t.Logf("checksumB (ToolCallID=call_999): %s", checksumB)

	if checksumA == checksumB {
		t.Errorf("ComputeMessageChecksum() should include ToolCallID: expected different checksums, both got %s", checksumA)
	}
}

func TestComputeMessageChecksum_DifferentMessages(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)

	tcA := findCase(t, cases, "different_messages_a")
	tcB := findCase(t, cases, "different_messages_b")
	msgA := toUnifiedMessage(t, tcA)
	msgB := toUnifiedMessage(t, tcB)

	checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
	checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

	t.Logf("checksumA: %s", checksumA)
	t.Logf("checksumB: %s", checksumB)

	if checksumA == checksumB {
		t.Errorf("ComputeMessageChecksum() should produce different checksums for different messages, both got %s", checksumA)
	}
}

func TestComputeMessageChecksum_EmptyToolCalls(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)

	tc := findCase(t, cases, "empty_tool_calls")
	msg := toUnifiedMessage(t, tc)

	checksum := vo.ComputeMessageChecksum(msg, "", nil)
	t.Logf("checksum: %s", checksum)

	if checksum == "" {
		t.Errorf("ComputeMessageChecksum() returned empty string for message without tool calls")
	}
}

func TestComputeMessageChecksum_MultipleToolCallsKeyOrder(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)

	tcA := findCase(t, cases, "multiple_tool_calls_key_order_a")
	tcB := findCase(t, cases, "multiple_tool_calls_key_order_b")
	msgA := toUnifiedMessage(t, tcA)
	msgB := toUnifiedMessage(t, tcB)

	checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
	checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

	t.Logf("checksumA: %s", checksumA)
	t.Logf("checksumB: %s", checksumB)

	if checksumA != checksumB {
		t.Errorf("ComputeMessageChecksum() mismatch with multiple tool calls: got %s and %s", checksumA, checksumB)
	}
}

// ==================== Schema-Aware Tests ====================

func TestComputeMessageChecksum_SchemaDefaultRemoved(t *testing.T) {
	t.Parallel()
	cases := loadSchemaAwareCases(t)
	schemas := loadToolSchemas(t)

	tcA := findCase(t, cases, "schema_default_bool_removed_a")
	tcB := findCase(t, cases, "schema_default_bool_removed_b")
	msgA := toUnifiedMessage(t, tcA)
	msgB := toUnifiedMessage(t, tcB)

	checksumA := vo.ComputeMessageChecksum(msgA, "", schemas)
	checksumB := vo.ComputeMessageChecksum(msgB, "", schemas)

	t.Logf("caseA args: %s", tcA.ToolCalls[0].Arguments)
	t.Logf("caseB args: %s", tcB.ToolCalls[0].Arguments)
	t.Logf("checksumA: %s, checksumB: %s", checksumA, checksumB)

	if checksumA != checksumB {
		t.Errorf("ComputeMessageChecksum() with schema should produce same checksum when optional field equals default: got %s and %s", checksumA, checksumB)
	}
}

func TestComputeMessageChecksum_SchemaNonDefaultKept(t *testing.T) {
	t.Parallel()
	cases := loadSchemaAwareCases(t)
	schemas := loadToolSchemas(t)

	tcA := findCase(t, cases, "schema_non_default_kept_a")
	tcB := findCase(t, cases, "schema_non_default_kept_b")
	msgA := toUnifiedMessage(t, tcA)
	msgB := toUnifiedMessage(t, tcB)

	checksumA := vo.ComputeMessageChecksum(msgA, "", schemas)
	checksumB := vo.ComputeMessageChecksum(msgB, "", schemas)

	t.Logf("caseA args (no replace_all): %s", tcA.ToolCalls[0].Arguments)
	t.Logf("caseB args (replace_all:true): %s", tcB.ToolCalls[0].Arguments)
	t.Logf("checksumA: %s, checksumB: %s", checksumA, checksumB)

	if checksumA == checksumB {
		t.Errorf("ComputeMessageChecksum() with schema should produce different checksum when optional field differs from default, both got %s", checksumA)
	}
}

func TestComputeMessageChecksum_SchemaRequiredFieldKept(t *testing.T) {
	t.Parallel()
	cases := loadSchemaAwareCases(t)
	schemas := loadToolSchemas(t)

	tcA := findCase(t, cases, "schema_required_default_kept_a")
	tcB := findCase(t, cases, "schema_required_default_kept_b")
	msgA := toUnifiedMessage(t, tcA)
	msgB := toUnifiedMessage(t, tcB)

	checksumA := vo.ComputeMessageChecksum(msgA, "", schemas)
	checksumB := vo.ComputeMessageChecksum(msgB, "", schemas)

	t.Logf("caseA args (verbose:false, required): %s", tcA.ToolCalls[0].Arguments)
	t.Logf("caseB args (no verbose, required field): %s", tcB.ToolCalls[0].Arguments)
	t.Logf("checksumA: %s, checksumB: %s", checksumA, checksumB)

	if checksumA == checksumB {
		t.Errorf("ComputeMessageChecksum() should NOT remove required fields even if equal to default, both got %s", checksumA)
	}
}

func TestComputeMessageChecksum_NoSchemaFallback(t *testing.T) {
	t.Parallel()
	cases := loadSchemaAwareCases(t)

	tcA := findCase(t, cases, "no_schema_different_a")
	tcB := findCase(t, cases, "no_schema_different_b")
	msgA := toUnifiedMessage(t, tcA)
	msgB := toUnifiedMessage(t, tcB)

	checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
	checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

	t.Logf("caseA args (no replace_all): %s", tcA.ToolCalls[0].Arguments)
	t.Logf("caseB args (replace_all:false): %s", tcB.ToolCalls[0].Arguments)
	t.Logf("checksumA: %s, checksumB: %s", checksumA, checksumB)

	if checksumA == checksumB {
		t.Errorf("ComputeMessageChecksum() without schema should NOT normalize default fields, both got %s", checksumA)
	}
}

func TestComputeMessageChecksum_SchemaMultipleDefaultsRemoved(t *testing.T) {
	t.Parallel()
	cases := loadSchemaAwareCases(t)
	schemas := loadToolSchemas(t)

	tcA := findCase(t, cases, "schema_multiple_defaults_removed")
	tcB := findCase(t, cases, "schema_multiple_defaults_absent")
	msgA := toUnifiedMessage(t, tcA)
	msgB := toUnifiedMessage(t, tcB)

	checksumA := vo.ComputeMessageChecksum(msgA, "", schemas)
	checksumB := vo.ComputeMessageChecksum(msgB, "", schemas)

	t.Logf("caseA args (with defaults): %s", tcA.ToolCalls[0].Arguments)
	t.Logf("caseB args (without defaults): %s", tcB.ToolCalls[0].Arguments)
	t.Logf("checksumA: %s, checksumB: %s", checksumA, checksumB)

	if checksumA != checksumB {
		t.Errorf("ComputeMessageChecksum() with schema should remove multiple optional fields at default values: got %s and %s", checksumA, checksumB)
	}
}

func TestComputeMessageChecksum_ReasoningContentSwap(t *testing.T) {
	t.Parallel()

	msgA := &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		Content:          &vo.UnifiedContent{Text: ""},
		ReasoningContent: "a",
	}
	msgB := &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		Content:          &vo.UnifiedContent{Text: "a"},
		ReasoningContent: "",
	}

	checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
	checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

	t.Logf("checksumA (rc=a, c=empty): %s", checksumA)
	t.Logf("checksumB (rc=empty, c=a): %s", checksumB)

	if checksumA != checksumB {
		t.Errorf("ComputeMessageChecksum should swap reasoning_content into empty content: got %s and %s", checksumA, checksumB)
	}
}

func TestComputeMessageChecksum_ReasoningContentBothNonEmpty(t *testing.T) {
	t.Parallel()

	msgA := &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		Content:          &vo.UnifiedContent{Text: "a"},
		ReasoningContent: "b",
	}
	msgB := &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		Content:          &vo.UnifiedContent{Text: "b"},
		ReasoningContent: "a",
	}

	checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
	checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

	t.Logf("checksumA (rc=b, c=a): %s", checksumA)
	t.Logf("checksumB (rc=a, c=b): %s", checksumB)

	if checksumA == checksumB {
		t.Errorf("ComputeMessageChecksum should produce different checksums when both content and reasoning_content are non-empty: both got %s", checksumA)
	}
}

func TestComputeMessageChecksum_DifferentContentStillDiffers(t *testing.T) {
	t.Parallel()

	msgA := &vo.UnifiedMessage{
		Role:             "assistant",
		Content:          &vo.UnifiedContent{Text: "Hello"},
		ReasoningContent: "thinking A",
	}
	msgB := &vo.UnifiedMessage{
		Role:             "assistant",
		Content:          &vo.UnifiedContent{Text: "World"},
		ReasoningContent: "thinking B",
	}

	csA := vo.ComputeMessageChecksum(msgA, "", nil)
	csB := vo.ComputeMessageChecksum(msgB, "", nil)

	if csA == csB {
		t.Errorf("ComputeMessageChecksum should produce different checksums for different content: both got %s", csA)
	}
}

func TestComputeMessageChecksum_ModelIncluded(t *testing.T) {
	t.Parallel()

	msg := &vo.UnifiedMessage{
		Role:    enum.RoleAssistant,
		Content: &vo.UnifiedContent{Text: "hello"},
	}

	checksumA := vo.ComputeMessageChecksum(msg, "gpt-4", nil)
	checksumB := vo.ComputeMessageChecksum(msg, "claude-3", nil)

	t.Logf("checksumA (model=gpt-4): %s", checksumA)
	t.Logf("checksumB (model=claude-3): %s", checksumB)

	if checksumA == checksumB {
		t.Errorf("ComputeMessageChecksum should include model: expected different checksums, both got %s", checksumA)
	}
}
