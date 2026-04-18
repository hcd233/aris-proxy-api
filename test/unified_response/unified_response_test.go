// Package unified_response verifies the UnifiedMessage conversion path used by
// the OpenAI Response API storage flow. The service combines `instructions`,
// request `input`, and response `output` into a single list of UnifiedMessage
// objects that are persisted via the existing MessageStoreTask pipeline.
package unified_response

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

type conversionCase struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	RequestBody  sonic.NoCopyRawMessage `json:"request_body"`
	ResponseBody sonic.NoCopyRawMessage `json:"response_body"`
}

func loadCases(t *testing.T) []conversionCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []conversionCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

func findCase(t *testing.T, cases []conversionCase, name string) conversionCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not found", name)
	return conversionCase{}
}

func parseBodies(t *testing.T, tc conversionCase) (*dto.OpenAICreateResponseReq, *dto.OpenAICreateResponseRsp) {
	t.Helper()
	var req dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
		t.Fatalf("unmarshal request_body: %v", err)
	}
	var rsp dto.OpenAICreateResponseRsp
	if err := sonic.Unmarshal(tc.ResponseBody, &rsp); err != nil {
		t.Fatalf("unmarshal response_body: %v", err)
	}
	return &req, &rsp
}

// buildConversation mimics the service-layer orchestration: it merges
// instructions/input/output into a single list of UnifiedMessage exactly the
// way openAIService.storeFromResponseRsp does in production code, giving the
// tests an end-to-end view of the store path.
func buildConversation(t *testing.T, req *dto.OpenAICreateResponseReq, rsp *dto.OpenAICreateResponseRsp) []*dto.UnifiedMessage {
	t.Helper()
	var msgs []*dto.UnifiedMessage

	if req.Instructions != nil && *req.Instructions != "" {
		msgs = append(msgs, &dto.UnifiedMessage{
			Role:    enum.RoleSystem,
			Content: &dto.UnifiedContent{Text: *req.Instructions},
		})
	}

	if req.Input != nil {
		if len(req.Input.Items) > 0 {
			inputMsgs, err := dto.FromResponseAPIInputItems(req.Input.Items)
			if err != nil {
				t.Fatalf("convert input items: %v", err)
			}
			msgs = append(msgs, inputMsgs...)
		} else if req.Input.Text != "" {
			msgs = append(msgs, &dto.UnifiedMessage{
				Role:    enum.RoleUser,
				Content: &dto.UnifiedContent{Text: req.Input.Text},
			})
		}
	}

	outputMsgs, err := dto.FromResponseAPIOutputItems(rsp.Output)
	if err != nil {
		t.Fatalf("convert output items: %v", err)
	}
	msgs = append(msgs, outputMsgs...)
	return msgs
}

func TestFromResponseAPI_TextInTextOut(t *testing.T) {
	tc := findCase(t, loadCases(t), "text_in_text_out")
	req, rsp := parseBodies(t, tc)

	msgs := buildConversation(t, req, rsp)
	if len(msgs) != 3 {
		t.Fatalf("len(msgs) = %d, want 3 (system+user+assistant)", len(msgs))
	}
	if msgs[0].Role != enum.RoleSystem || msgs[0].Content == nil || msgs[0].Content.Text != "You are Codex." {
		t.Errorf("system message mismatch: %+v", msgs[0])
	}
	if msgs[1].Role != enum.RoleUser {
		t.Errorf("user role = %q, want user", msgs[1].Role)
	}
	if msgs[1].Content == nil || len(msgs[1].Content.Parts) != 1 || msgs[1].Content.Parts[0].Text != "Hello" {
		t.Errorf("user content mismatch: %+v", msgs[1].Content)
	}
	if msgs[2].Role != enum.RoleAssistant {
		t.Errorf("assistant role = %q, want assistant", msgs[2].Role)
	}
	if msgs[2].Content == nil || len(msgs[2].Content.Parts) != 1 || msgs[2].Content.Parts[0].Text != "Hi there!" {
		t.Errorf("assistant content mismatch: %+v", msgs[2].Content)
	}
}

func TestFromResponseAPI_ReasoningMergedIntoAssistant(t *testing.T) {
	tc := findCase(t, loadCases(t), "reasoning_then_message")
	req, rsp := parseBodies(t, tc)

	msgs := buildConversation(t, req, rsp)
	// user (from string input) + assistant (with merged reasoning)
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	if msgs[0].Role != enum.RoleUser || msgs[0].Content == nil || msgs[0].Content.Text != "ping" {
		t.Errorf("user message mismatch: %+v", msgs[0])
	}
	ai := msgs[1]
	if ai.Role != enum.RoleAssistant {
		t.Errorf("role = %q, want assistant", ai.Role)
	}
	if ai.ReasoningContent != "Thinking about ping." {
		t.Errorf("ReasoningContent = %q, want %q", ai.ReasoningContent, "Thinking about ping.")
	}
	if ai.Content == nil || len(ai.Content.Parts) != 1 || ai.Content.Parts[0].Text != "pong" {
		t.Errorf("assistant content mismatch: %+v", ai.Content)
	}
}

func TestFromResponseAPI_FunctionCallAndOutput(t *testing.T) {
	tc := findCase(t, loadCases(t), "function_call_and_output")
	req, rsp := parseBodies(t, tc)

	msgs := buildConversation(t, req, rsp)
	// user + assistant(tool_calls) + tool + assistant(final)
	if len(msgs) != 4 {
		t.Fatalf("len(msgs) = %d, want 4\n%+v", len(msgs), msgs)
	}

	if msgs[0].Role != enum.RoleUser {
		t.Errorf("msgs[0] role = %q, want user", msgs[0].Role)
	}

	toolCall := msgs[1]
	if toolCall.Role != enum.RoleAssistant || len(toolCall.ToolCalls) != 1 {
		t.Fatalf("expected assistant tool_calls message, got %+v", toolCall)
	}
	if toolCall.ToolCalls[0].ID != "call_abc" || toolCall.ToolCalls[0].Name != "get_weather" || toolCall.ToolCalls[0].Arguments != `{"city":"Shanghai"}` {
		t.Errorf("tool_call mismatch: %+v", toolCall.ToolCalls[0])
	}

	toolResult := msgs[2]
	if toolResult.Role != enum.RoleTool || toolResult.ToolCallID != "call_abc" {
		t.Fatalf("expected tool-role message with call_id, got %+v", toolResult)
	}
	if toolResult.Content == nil || toolResult.Content.Text != "Sunny, 25C" {
		t.Errorf("tool result content mismatch: %+v", toolResult.Content)
	}

	final := msgs[3]
	if final.Role != enum.RoleAssistant {
		t.Errorf("final role = %q, want assistant", final.Role)
	}

	// Tool conversion
	tool := dto.FromResponseAPITool(req.Tools[0])
	if tool == nil || tool.Name != "get_weather" {
		t.Errorf("tool conversion mismatch: %+v", tool)
	}
}

func TestFromResponseAPI_StringInput(t *testing.T) {
	tc := findCase(t, loadCases(t), "string_input")
	req, rsp := parseBodies(t, tc)

	msgs := buildConversation(t, req, rsp)
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	if msgs[0].Role != enum.RoleUser || msgs[0].Content == nil || msgs[0].Content.Text != "Hello" {
		t.Errorf("user mismatch: %+v", msgs[0])
	}
	if msgs[1].Role != enum.RoleAssistant {
		t.Errorf("assistant role = %q", msgs[1].Role)
	}
}

// TestResponseUsage_AuditTokens verifies token accounting uses the Response
// API usage block (including cached-input tokens).
func TestResponseUsage_AuditTokens(t *testing.T) {
	tc := findCase(t, loadCases(t), "reasoning_then_message")
	_, rsp := parseBodies(t, tc)

	task := &dto.ModelCallAuditTask{}
	task.SetTokensFromResponseUsage(rsp)
	if task.InputTokens != 5 {
		t.Errorf("InputTokens = %d, want 5", task.InputTokens)
	}
	if task.OutputTokens != 3 {
		t.Errorf("OutputTokens = %d, want 3", task.OutputTokens)
	}
	if task.CacheReadInputTokens != 1 {
		t.Errorf("CacheReadInputTokens = %d, want 1", task.CacheReadInputTokens)
	}
}

// TestResponseStreamTerminalEvent_Parse asserts the response.completed SSE
// data payload parses into the typed terminal event wrapper.
func TestResponseStreamTerminalEvent_Parse(t *testing.T) {
	payload := []byte(`{"type":"response.completed","response":{"id":"resp_x","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}`)
	var ev dto.ResponseStreamTerminalEvent
	if err := sonic.Unmarshal(payload, &ev); err != nil {
		t.Fatalf("unmarshal terminal event: %v", err)
	}
	if ev.Type != "response.completed" {
		t.Errorf("Type = %q, want response.completed", ev.Type)
	}
	if ev.Response == nil {
		t.Fatal("Response should not be nil")
	}
	if ev.Response.ID != "resp_x" {
		t.Errorf("Response.ID = %q, want resp_x", ev.Response.ID)
	}
	if ev.Response.Usage == nil || ev.Response.Usage.InputTokens != 2 {
		t.Errorf("Usage mismatch: %+v", ev.Response.Usage)
	}
}
