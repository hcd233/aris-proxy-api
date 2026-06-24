// Package unified_response verifies the UnifiedMessage conversion path used by
// the OpenAI Response API storage flow. The service combines `instructions`,
// request `input`, and response `output` into a single list of UnifiedMessage
// objects that are persisted via the existing MessageStoreTask pipeline.
package unified_response

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/bytedance/sonic"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type conversionCase struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	RequestBody  sonic.NoCopyRawMessage `json:"request_body"`
	ResponseBody sonic.NoCopyRawMessage `json:"response_body"`
}

func stringPtr(value string) *string {
	return &value
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

func parseBodies(t *testing.T, tc conversionCase) (req *dto.OpenAICreateResponseReq, rsp *dto.OpenAICreateResponseRsp) {
	t.Helper()
	var request dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(tc.RequestBody, &request); err != nil {
		t.Fatalf("unmarshal request_body: %v", err)
	}
	var response dto.OpenAICreateResponseRsp
	if err := sonic.Unmarshal(tc.ResponseBody, &response); err != nil {
		t.Fatalf("unmarshal response_body: %v", err)
	}
	return &request, &response
}

// buildConversation mimics the service-layer orchestration: it merges
// instructions/input/output into a single list of UnifiedMessage exactly the
// way openAIService.storeFromResponseRsp does in production code, giving the
// tests an end-to-end view of the store path.
func buildConversation(t *testing.T, req *dto.OpenAICreateResponseReq, rsp *dto.OpenAICreateResponseRsp) []*vo.UnifiedMessage {
	t.Helper()
	var msgs []*vo.UnifiedMessage

	if req.Instructions != nil && *req.Instructions != "" {
		msgs = append(msgs, &vo.UnifiedMessage{
			Role:    enum.RoleSystem,
			Content: &vo.UnifiedContent{Text: *req.Instructions},
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
			msgs = append(msgs, &vo.UnifiedMessage{
				Role:    enum.RoleUser,
				Content: &vo.UnifiedContent{Text: req.Input.Text},
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestFromResponseAPITool_Mcp verifies that an MCP tool is converted to a
// UnifiedTool (not silently dropped), so it gets persisted to the tools table.
func TestFromResponseAPITool_Mcp(t *testing.T) {
	t.Parallel()
	mcpTool := &dto.ResponseTool{
		Type: enum.ResponseToolTypeMcp,
		Mcp: &dto.ResponseToolMcp{
			Type:        enum.ResponseToolTypeMcp,
			ServerLabel: "my-mcp-server",
		},
	}
	tool := dto.FromResponseAPITool(mcpTool)
	if tool == nil {
		t.Fatal("expected non-nil UnifiedTool for MCP tool, got nil")
	}
	if tool.Name != "my-mcp-server" {
		t.Errorf("Name = %q, want %q", tool.Name, "my-mcp-server")
	}
	wantDesc := fmt.Sprintf(constant.ChatCompletionConvertToolDescMCPTemplate, "my-mcp-server")
	if tool.Description != wantDesc {
		t.Errorf("Description = %q, want %q", tool.Description, wantDesc)
	}
}

// TestFromResponseAPITools_NamespaceFlatten verifies that namespace tools
// (which carry MCP sub-tools) are flattened into independent UnifiedTools so
// every sub-tool gets persisted to the tools table, matching the converter
// chain's {namespace}__{subTool} naming.
func TestFromResponseAPITools_NamespaceFlatten(t *testing.T) {
	t.Parallel()
	tools := []*dto.ResponseTool{{
		Type: enum.ResponseToolTypeNamespace,
		Namespace: &dto.ResponseToolNamespace{
			Type: enum.ResponseToolTypeNamespace,
			Name: "github",
			Tools: []*dto.ResponseNamespaceTool{
				{Name: "list_repos", Type: enum.ResponseToolTypeFunction},
				{Name: "create_issue", Type: enum.ResponseToolTypeFunction},
			},
		},
	}}
	got := dto.FromResponseAPITools(tools)
	if len(got) != 2 {
		t.Fatalf("expected 2 flattened tools, got %d", len(got))
	}
	sep := constant.NamespaceToolSeparator
	if got[0].Name != "github"+sep+"list_repos" {
		t.Errorf("tool[0].Name = %q", got[0].Name)
	}
	if got[1].Name != "github"+sep+"create_issue" {
		t.Errorf("tool[1].Name = %q", got[1].Name)
	}
}

func TestFromResponseAPI_StringInput(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestFillResponseTerminalOutputUsesAccumulatedItems(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"type":"response.completed","response":{"id":"resp_x","object":"response","status":"completed","output":[],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}`)
	accumulated := []*dto.ResponseInputItem{
		{Type: stringPtr(enum.ResponseInputItemTypeReasoning), ID: stringPtr("rs_1")},
		{Type: stringPtr(enum.ResponseInputItemTypeFunctionCall), ID: stringPtr("fc_1"), Name: stringPtr("exec"), Arguments: stringPtr("{}")},
	}

	patched, changed, err := proxyutil.FillResponseTerminalOutput(payload, accumulated)
	if err != nil {
		t.Fatalf("fill terminal output: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}

	var ev dto.ResponseStreamTerminalEvent
	if err := sonic.Unmarshal(patched, &ev); err != nil {
		t.Fatalf("unmarshal patched terminal event: %v", err)
	}
	if ev.Response == nil {
		t.Fatal("Response should not be nil")
	}
	if len(ev.Response.Output) != 2 {
		t.Fatalf("len(Output) = %d, want 2", len(ev.Response.Output))
	}
	if ev.Response.Output[0].Type == nil || *ev.Response.Output[0].Type != enum.ResponseInputItemTypeReasoning {
		t.Errorf("Output[0].Type = %v, want reasoning", ev.Response.Output[0].Type)
	}
	if ev.Response.Output[1].Type == nil || *ev.Response.Output[1].Type != enum.ResponseInputItemTypeFunctionCall {
		t.Errorf("Output[1].Type = %v, want function_call", ev.Response.Output[1].Type)
	}
	if ev.Response.Usage == nil || ev.Response.Usage.TotalTokens != 3 {
		t.Errorf("Usage mismatch: %+v", ev.Response.Usage)
	}
}

func TestFillResponseTerminalOutputKeepsExistingOutput(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"type":"response.completed","response":{"id":"resp_x","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}}`)
	accumulated := []*dto.ResponseInputItem{{Type: stringPtr(enum.ResponseInputItemTypeReasoning), ID: stringPtr("rs_1")}}

	patched, changed, err := proxyutil.FillResponseTerminalOutput(payload, accumulated)
	if err != nil {
		t.Fatalf("fill terminal output: %v", err)
	}
	if changed {
		t.Fatal("changed = true, want false")
	}
	if !bytes.Equal(patched, payload) {
		t.Errorf("patched payload changed unexpectedly")
	}
}

// TestResponseStreamTerminalEvent_ParseFailed and Incomplete verify the
// same typed wrapper also handles the two other terminal events
// (response.failed, response.incomplete). Both carry the final Response
// object and must populate Usage + diagnostic fields (error /
// incomplete_details) so audit accounting is not lost on in-band failure.
func TestResponseStreamTerminalEvent_ParseFailed(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"type":"response.failed","response":{"id":"resp_f","object":"response","status":"failed","output":[],"error":{"code":"server_error","message":"upstream model unavailable"},"usage":{"input_tokens":4,"output_tokens":0,"total_tokens":4}}}`)
	var ev dto.ResponseStreamTerminalEvent
	if err := sonic.Unmarshal(payload, &ev); err != nil {
		t.Fatalf("unmarshal failed event: %v", err)
	}
	if ev.Type != "response.failed" {
		t.Errorf("Type = %q, want response.failed", ev.Type)
	}
	if ev.Response == nil || ev.Response.Status != "failed" {
		t.Fatalf("Response mismatch: %+v", ev.Response)
	}
	if ev.Response.Error == nil || ev.Response.Error.Message != "upstream model unavailable" {
		t.Errorf("Error mismatch: %+v", ev.Response.Error)
	}
	if ev.Response.Usage == nil || ev.Response.Usage.InputTokens != 4 {
		t.Errorf("Usage mismatch: %+v", ev.Response.Usage)
	}
}

func TestResponseStreamTerminalEvent_ParseIncomplete(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"type":"response.incomplete","response":{"id":"resp_i","object":"response","status":"incomplete","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"partial"}]}],"incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":3,"output_tokens":9,"total_tokens":12}}}`)
	var ev dto.ResponseStreamTerminalEvent
	if err := sonic.Unmarshal(payload, &ev); err != nil {
		t.Fatalf("unmarshal incomplete event: %v", err)
	}
	if ev.Type != "response.incomplete" {
		t.Errorf("Type = %q, want response.incomplete", ev.Type)
	}
	if ev.Response == nil || ev.Response.IncompleteDetails == nil {
		t.Fatalf("Response/IncompleteDetails mismatch: %+v", ev.Response)
	}
	if ev.Response.IncompleteDetails.Reason != "max_output_tokens" {
		t.Errorf("Reason = %q, want max_output_tokens", ev.Response.IncompleteDetails.Reason)
	}
}

// TestIsResponseAPIDeltaEvent verifies the delta-event classifier used by
// the service layer to measure time-to-first-token. All of the metadata
// framing events (response.created / response.in_progress / *.added /
// *.done) must not count; every generated-token event (regardless of
// modality) must.
func TestIsResponseAPIDeltaEvent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		event string
		want  bool
	}{
		{"response.created", false},
		{"response.in_progress", false},
		{"response.output_item.added", false},
		{"response.content_part.added", false},
		{"response.output_text.done", false},
		{"response.output_item.done", false},
		{"response.completed", false},
		{"response.failed", false},
		{"response.incomplete", false},

		{"response.output_text.delta", true},
		{"response.reasoning_text.delta", true},
		{"response.reasoning_summary_text.delta", true},
		{"response.function_call_arguments.delta", true},
		{"response.custom_tool_call_input.delta", true},
		{"response.audio.delta", true},
	}
	for _, tc := range cases {
		if got := proxyutil.IsResponseAPIDeltaEvent(tc.event); got != tc.want {
			t.Errorf("IsResponseAPIDeltaEvent(%q) = %v, want %v", tc.event, got, tc.want)
		}
	}
}

// TestIsResponseAPITerminalEvent covers the three terminal events that
// carry the final Response object. Everything else must report false so
// the service doesn't try to unmarshal an intermediate event body as a
// ResponseStreamTerminalEvent.
func TestIsResponseAPITerminalEvent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		event string
		want  bool
	}{
		{"response.completed", true},
		{"response.failed", true},
		{"response.incomplete", true},

		{"response.created", false},
		{"response.in_progress", false},
		{"response.output_text.delta", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := proxyutil.IsResponseAPITerminalEvent(tc.event); got != tc.want {
			t.Errorf("IsResponseAPITerminalEvent(%q) = %v, want %v", tc.event, got, tc.want)
		}
	}
}

// TestSetErrorFromResponseStatus_Failed covers the case where HTTP
// transport returned 200 but the response object carried status=failed
// with an error payload; the reason must land on the audit task.
func TestSetErrorFromResponseStatus_Failed(t *testing.T) {
	t.Parallel()
	tc := findCase(t, loadCases(t), "failed_response")
	_, rsp := parseBodies(t, tc)

	task := &dto.ModelCallAuditTask{}
	task.SetTokensFromResponseUsage(rsp)
	task.SetErrorFromResponseStatus(rsp)

	if task.InputTokens != 2 {
		t.Errorf("InputTokens = %d, want 2", task.InputTokens)
	}
	want := "response.failed: upstream model unavailable"
	if task.ErrorMessage != want {
		t.Errorf("ErrorMessage = %q, want %q", task.ErrorMessage, want)
	}
}

// TestSetErrorFromResponseStatus_Incomplete verifies the reason from
// incomplete_details (e.g. max_output_tokens) is also surfaced.
func TestSetErrorFromResponseStatus_Incomplete(t *testing.T) {
	t.Parallel()
	tc := findCase(t, loadCases(t), "incomplete_with_output")
	_, rsp := parseBodies(t, tc)

	task := &dto.ModelCallAuditTask{}
	task.SetErrorFromResponseStatus(rsp)

	want := "response.incomplete: max_output_tokens"
	if task.ErrorMessage != want {
		t.Errorf("ErrorMessage = %q, want %q", task.ErrorMessage, want)
	}
}

// TestSetErrorFromResponseStatus_PreservesTransportError asserts the
// helper never overwrites an existing ErrorMessage (set by a real
// transport-level error extracted upstream), matching the intent of
// distinguishing transport vs. in-band failures.
func TestSetErrorFromResponseStatus_PreservesTransportError(t *testing.T) {
	t.Parallel()
	tc := findCase(t, loadCases(t), "failed_response")
	_, rsp := parseBodies(t, tc)

	task := &dto.ModelCallAuditTask{ErrorMessage: "http timeout"}
	task.SetErrorFromResponseStatus(rsp)
	if task.ErrorMessage != "http timeout" {
		t.Errorf("ErrorMessage overwritten: got %q, want %q", task.ErrorMessage, "http timeout")
	}
}

// TestSetErrorFromResponseStatus_CompletedIsNoop asserts a healthy
// response never produces an ErrorMessage.
func TestSetErrorFromResponseStatus_CompletedIsNoop(t *testing.T) {
	t.Parallel()
	tc := findCase(t, loadCases(t), "text_in_text_out")
	_, rsp := parseBodies(t, tc)

	task := &dto.ModelCallAuditTask{}
	task.SetErrorFromResponseStatus(rsp)
	if task.ErrorMessage != "" {
		t.Errorf("ErrorMessage = %q, want empty", task.ErrorMessage)
	}
}

// TestFromResponseAPI_IncompleteWithOutputPersists asserts that an
// in-band `incomplete` response with partial Output still produces
// UnifiedMessages. This matches /chat/completions, which stores
// completions even when finish_reason=length.
func TestFromResponseAPI_IncompleteWithOutputPersists(t *testing.T) {
	t.Parallel()
	tc := findCase(t, loadCases(t), "incomplete_with_output")
	req, rsp := parseBodies(t, tc)

	// Precondition: status must be incomplete so we're really testing the
	// incomplete path (otherwise the assertion is vacuous).
	if rsp.Status != enum.ResponseStatus("incomplete") {
		t.Fatalf("fixture status = %q, want incomplete", rsp.Status)
	}

	msgs := buildConversation(t, req, rsp)
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2 (user + partial assistant)", len(msgs))
	}
	if msgs[0].Role != enum.RoleUser {
		t.Errorf("msgs[0] role = %q, want user", msgs[0].Role)
	}
	if msgs[1].Role != enum.RoleAssistant {
		t.Errorf("msgs[1] role = %q, want assistant", msgs[1].Role)
	}
	if msgs[1].Content == nil || len(msgs[1].Content.Parts) != 1 || msgs[1].Content.Parts[0].Text != "Once upon a time," {
		t.Errorf("partial assistant content mismatch: %+v", msgs[1].Content)
	}
}
