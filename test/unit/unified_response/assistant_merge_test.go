package unified_response

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// TestFromResponseAPI_OutputMergesConsecutiveAssistant 验证 Response API output 中
// reasoning + 多个 function_call + message text 被合并为一条 UnifiedMessage，
// 携带 reasoning_content、全部 tool_calls 和 content。
func TestFromResponseAPI_OutputMergesConsecutiveAssistant(t *testing.T) {
	t.Parallel()
	tc := findCase(t, loadCases(t), "reasoning_function_calls_message_merged")
	_, rsp := parseBodies(t, tc)

	msgs, err := dto.FromResponseAPIOutputItems(rsp.Output)
	if err != nil {
		t.Fatalf("FromResponseAPIOutputItems() error: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1 (all assistant items merged)", len(msgs))
	}

	ai := msgs[0]
	if ai.Role != enum.RoleAssistant {
		t.Errorf("role = %q, want assistant", ai.Role)
	}
	if ai.ReasoningContent != "I need to call two tools." {
		t.Errorf("ReasoningContent = %q, want %q", ai.ReasoningContent, "I need to call two tools.")
	}
	if len(ai.ToolCalls) != 2 {
		t.Fatalf("len(ToolCalls) = %d, want 2", len(ai.ToolCalls))
	}
	if ai.ToolCalls[0].ID != "call_w" || ai.ToolCalls[0].Name != "get_weather" {
		t.Errorf("ToolCalls[0] mismatch: %+v", ai.ToolCalls[0])
	}
	if ai.ToolCalls[1].ID != "call_t" || ai.ToolCalls[1].Name != "get_time" {
		t.Errorf("ToolCalls[1] mismatch: %+v", ai.ToolCalls[1])
	}
	if ai.Content == nil || len(ai.Content.Parts) != 1 || ai.Content.Parts[0].Text != "Let me check both for you." {
		t.Errorf("Content mismatch: %+v", ai.Content)
	}
}

// TestFromResponseAPI_InputMergesConsecutiveFunctionCalls 验证 Response API input 中
// 连续的 function_call 被合并为一条 assistant 消息（多个 tool_calls），
// 后续的 function_call_output 保持为独立的 tool 消息，
// 再后续的 assistant message 保持独立。
func TestFromResponseAPI_InputMergesConsecutiveFunctionCalls(t *testing.T) {
	t.Parallel()
	tc := findCase(t, loadCases(t), "consecutive_function_calls_in_input")
	req, _ := parseBodies(t, tc)

	msgs, err := dto.FromResponseAPIInputItems(req.Input.Items)
	if err != nil {
		t.Fatalf("FromResponseAPIInputItems() error: %v", err)
	}

	// user + merged assistant(2 tool_calls) + tool + tool + assistant(text) = 5
	if len(msgs) != 5 {
		t.Fatalf("len(msgs) = %d, want 5\n%+v", len(msgs), msgs)
	}

	if msgs[0].Role != enum.RoleUser {
		t.Errorf("msgs[0] role = %q, want user", msgs[0].Role)
	}

	mergedAssistant := msgs[1]
	if mergedAssistant.Role != enum.RoleAssistant {
		t.Errorf("msgs[1] role = %q, want assistant", mergedAssistant.Role)
	}
	if len(mergedAssistant.ToolCalls) != 2 {
		t.Fatalf("len(ToolCalls) = %d, want 2 (merged)", len(mergedAssistant.ToolCalls))
	}
	if mergedAssistant.ToolCalls[0].ID != "call_w" || mergedAssistant.ToolCalls[1].ID != "call_t" {
		t.Errorf("ToolCall IDs mismatch: %+v", mergedAssistant.ToolCalls)
	}

	if msgs[2].Role != enum.RoleTool || msgs[2].ToolCallID != "call_w" {
		t.Errorf("msgs[2] mismatch: %+v", msgs[2])
	}
	if msgs[3].Role != enum.RoleTool || msgs[3].ToolCallID != "call_t" {
		t.Errorf("msgs[3] mismatch: %+v", msgs[3])
	}

	if msgs[4].Role != enum.RoleAssistant {
		t.Errorf("msgs[4] role = %q, want assistant", msgs[4].Role)
	}
}
