package converter

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// responseAlignmentCase 用于断言 Response → ChatCompletion 字段对齐的固定夹具。
type responseAlignmentCase struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Request     *dto.OpenAICreateResponseReq `json:"request"`
}

func loadAlignmentCases(t *testing.T) []responseAlignmentCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/response_alignment.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/response_alignment.json: %v", err)
	}
	var cases []responseAlignmentCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/response_alignment.json: %v", err)
	}
	return cases
}

func findAlignmentCase(t *testing.T, cases []responseAlignmentCase, name string) responseAlignmentCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("alignment case %q not found", name)
	return responseAlignmentCase{}
}

// TestResponseProtocolConverter_FromResponseRequest_FieldAlignment 验证 chat completions
// 转 response 链路下，所有 Chat 协议有对等字段的 Response 请求字段都被透传。
func TestResponseProtocolConverter_FromResponseRequest_FieldAlignment(t *testing.T) {
	t.Parallel()
	tc := findAlignmentCase(t, loadAlignmentCases(t), "full_codex_request")

	conv := &converter.ResponseProtocolConverter{}
	chatReq, err := conv.FromResponseRequest(tc.Request)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}

	if chatReq.Model != lo.FromPtr(tc.Request.Model) {
		t.Errorf("Model not aligned: got %q, want %q", chatReq.Model, lo.FromPtr(tc.Request.Model))
	}
	if lo.FromPtr(chatReq.Stream) != lo.FromPtr(tc.Request.Stream) {
		t.Errorf("Stream not aligned: got %v, want %v", chatReq.Stream, tc.Request.Stream)
	}
	if lo.FromPtr(chatReq.Temperature) != lo.FromPtr(tc.Request.Temperature) {
		t.Errorf("Temperature not aligned: got %v, want %v", chatReq.Temperature, tc.Request.Temperature)
	}
	if lo.FromPtr(chatReq.TopP) != lo.FromPtr(tc.Request.TopP) {
		t.Errorf("TopP not aligned")
	}
	if lo.FromPtr(chatReq.TopLogprobs) != lo.FromPtr(tc.Request.TopLogprobs) {
		t.Errorf("TopLogprobs not aligned")
	}
	if lo.FromPtr(chatReq.ParallelToolCalls) != lo.FromPtr(tc.Request.ParallelToolCalls) {
		t.Errorf("ParallelToolCalls not aligned: got %v, want %v",
			lo.FromPtr(chatReq.ParallelToolCalls), lo.FromPtr(tc.Request.ParallelToolCalls))
	}
	if lo.FromPtr(chatReq.PromptCacheKey) != lo.FromPtr(tc.Request.PromptCacheKey) {
		t.Errorf("PromptCacheKey not aligned: got %q, want %q",
			lo.FromPtr(chatReq.PromptCacheKey), lo.FromPtr(tc.Request.PromptCacheKey))
	}
	if chatReq.PromptCacheRetention != lo.FromPtr(tc.Request.PromptCacheRetention) {
		t.Errorf("PromptCacheRetention not aligned: got %q, want %q",
			chatReq.PromptCacheRetention, lo.FromPtr(tc.Request.PromptCacheRetention))
	}
	if lo.FromPtr(chatReq.SafetyIdentifier) != lo.FromPtr(tc.Request.SafetyIdentifier) {
		t.Errorf("SafetyIdentifier not aligned")
	}
	if chatReq.ServiceTier != lo.FromPtr(tc.Request.ServiceTier) {
		t.Errorf("ServiceTier not aligned: got %q, want %q",
			chatReq.ServiceTier, lo.FromPtr(tc.Request.ServiceTier))
	}
	if lo.FromPtr(chatReq.Store) != lo.FromPtr(tc.Request.Store) {
		t.Errorf("Store not aligned")
	}
	if lo.FromPtr(chatReq.User) != lo.FromPtr(tc.Request.User) {
		t.Errorf("User not aligned")
	}
	if lo.FromPtr(chatReq.MaxCompletionTokens) != int(lo.FromPtr(tc.Request.MaxOutputTokens)) {
		t.Errorf("MaxCompletionTokens not aligned: got %v, want %v",
			lo.FromPtr(chatReq.MaxCompletionTokens), lo.FromPtr(tc.Request.MaxOutputTokens))
	}
	if chatReq.ReasoningEffort != lo.FromPtr(tc.Request.Reasoning.Effort) {
		t.Errorf("ReasoningEffort not aligned: got %q, want %q",
			chatReq.ReasoningEffort, lo.FromPtr(tc.Request.Reasoning.Effort))
	}
	if chatReq.Verbosity != lo.FromPtr(tc.Request.Text.Verbosity) {
		t.Errorf("Verbosity not aligned: got %q, want %q",
			chatReq.Verbosity, lo.FromPtr(tc.Request.Text.Verbosity))
	}
	if chatReq.Metadata == nil {
		t.Errorf("Metadata not aligned: got nil")
	} else if chatReq.Metadata["k"] != "v" {
		t.Errorf("Metadata not aligned: got %v", chatReq.Metadata)
	}
	if chatReq.StreamOptions == nil {
		t.Fatalf("StreamOptions not aligned: got nil")
	}
	if lo.FromPtr(chatReq.StreamOptions.IncludeObfuscation) != lo.FromPtr(tc.Request.StreamOptions.IncludeObfuscation) {
		t.Errorf("StreamOptions.IncludeObfuscation not aligned")
	}
}

// TestResponseProtocolConverter_FromResponseRequest_OmitsEmpty 验证未提供的 Response
// 字段不会在 ChatCompletion 上写出零值，避免污染 prompt cache 的字节稳定序列化。
func TestResponseProtocolConverter_FromResponseRequest_OmitsEmpty(t *testing.T) {
	t.Parallel()
	tc := findAlignmentCase(t, loadAlignmentCases(t), "minimal_request")

	conv := &converter.ResponseProtocolConverter{}
	chatReq, err := conv.FromResponseRequest(tc.Request)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}

	if chatReq.ParallelToolCalls != nil {
		t.Errorf("ParallelToolCalls should remain nil, got %v", *chatReq.ParallelToolCalls)
	}
	if chatReq.PromptCacheKey != nil {
		t.Errorf("PromptCacheKey should remain nil, got %q", *chatReq.PromptCacheKey)
	}
	if chatReq.Store != nil {
		t.Errorf("Store should remain nil, got %v", *chatReq.Store)
	}
	if chatReq.ReasoningEffort != "" {
		t.Errorf("ReasoningEffort should be empty, got %q", chatReq.ReasoningEffort)
	}
	if chatReq.Verbosity != "" {
		t.Errorf("Verbosity should be empty, got %q", chatReq.Verbosity)
	}
	if chatReq.ServiceTier != "" {
		t.Errorf("ServiceTier should be empty, got %q", chatReq.ServiceTier)
	}
	if chatReq.PromptCacheRetention != "" {
		t.Errorf("PromptCacheRetention should be empty, got %q", chatReq.PromptCacheRetention)
	}
	if chatReq.StreamOptions != nil {
		t.Errorf("StreamOptions should be nil, got %v", chatReq.StreamOptions)
	}
	if chatReq.ResponseFormat != nil {
		t.Errorf("ResponseFormat should be nil")
	}
	// 至少有 user message
	if len(chatReq.Messages) == 0 {
		t.Fatalf("Messages should contain converted input, got empty")
	}
	if chatReq.Messages[0].Role != enum.RoleUser {
		t.Errorf("first message role got %q, want %q", chatReq.Messages[0].Role, enum.RoleUser)
	}
}

// TestResponseProtocolConverter_FromResponseRequest_SkipsEmptyAssistant 验证
// Response API input 中的空 assistant 消息（无 content 也无 tool_calls）在转换为
// Chat Completions 时被跳过，避免上游返回 400: "Invalid assistant message: content or tool_calls must be set"。
func TestResponseProtocolConverter_FromResponseRequest_SkipsEmptyAssistant(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}

	req := &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("deepseek-v4-flash"),
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{
				{
					Type: lo.ToPtr(enum.ResponseInputItemTypeMessage),
					Role: lo.ToPtr(enum.RoleUser),
					Content: &dto.ResponseInputMessageContent{
						Text: "Hello",
					},
				},
				{
					Type:    lo.ToPtr(enum.ResponseInputItemTypeMessage),
					Role:    lo.ToPtr(enum.RoleAssistant),
					Content: nil,
				},
			},
		},
	}

	chatReq, err := conv.FromResponseRequest(req)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}

	assistantCount := 0
	for _, msg := range chatReq.Messages {
		if msg.Role == enum.RoleAssistant {
			assistantCount++
		}
	}
	if assistantCount > 0 {
		t.Errorf("expected 0 assistant messages, got %d", assistantCount)
	}
}

// TestResponseProtocolConverter_FromResponseRequest_ReasoningSkipped 验证
// reasoning item 在转换时被跳过，因为它代表模型内部思维过程，
// 在 Chat Completions 格式中无对应的请求消息类型。
func TestResponseProtocolConverter_FromResponseRequest_ReasoningSkipped(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}

	req := &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("deepseek-v4-flash"),
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{
				{
					Type: lo.ToPtr(enum.ResponseInputItemTypeMessage),
					Role: lo.ToPtr(enum.RoleUser),
					Content: &dto.ResponseInputMessageContent{
						Text: "Hello",
					},
				},
				{
					Type: lo.ToPtr(enum.ResponseInputItemTypeReasoning),
					Summary: []*dto.ResponseReasoningSummary{{
						Text: "Let me think about this...",
						Type: enum.ResponseContentTypeSummaryText,
					}},
				},
			},
		},
	}

	chatReq, err := conv.FromResponseRequest(req)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}

	for _, msg := range chatReq.Messages {
		if msg.Role == enum.RoleAssistant {
			t.Error("reasoning item should not produce assistant messages")
		}
	}
}

// TestResponseProtocolConverter_FromResponseRequest_MergesConsecutiveAssistant 验证
// 连续的 assistant 消息（来自 function_call + message/assistant）被合并为一条，
// 避免 Chat Completions API 的 invariant 违反：tool_calls assistant 后必须紧跟 tool 响应。
func TestResponseProtocolConverter_FromResponseRequest_MergesConsecutiveAssistant(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}

	req := &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("deepseek-v4-flash"),
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{
				{
					Type: lo.ToPtr(enum.ResponseInputItemTypeMessage),
					Role: lo.ToPtr(enum.RoleUser),
					Content: &dto.ResponseInputMessageContent{
						Text: "run a command",
					},
				},
				{
					Type:      lo.ToPtr(enum.ResponseInputItemTypeFunctionCall),
					CallID:    lo.ToPtr("call_123"),
					Name:      lo.ToPtr("exec_command"),
					Arguments: lo.ToPtr(`{"command":"ls"}`),
				},
				{
					Type: lo.ToPtr(enum.ResponseInputItemTypeMessage),
					Role: lo.ToPtr(enum.RoleAssistant),
					Content: &dto.ResponseInputMessageContent{
						Text: "Let me run that command",
					},
				},
				{
					Type:   lo.ToPtr(enum.ResponseInputItemTypeFunctionCallOutput),
					CallID: lo.ToPtr("call_123"),
					Output: &dto.ResponseInputItemOutput{
						Text: "file1 file2",
					},
				},
			},
		},
	}

	chatReq, err := conv.FromResponseRequest(req)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}

	assistantCount := 0
	var assistantWithToolCalls *dto.OpenAIChatCompletionMessageParam
	for _, msg := range chatReq.Messages {
		if msg.Role == enum.RoleAssistant {
			assistantCount++
			if len(msg.ToolCalls) > 0 {
				assistantWithToolCalls = msg
			}
		}
	}
	if assistantCount != 1 {
		t.Errorf("expected 1 merged assistant message, got %d", assistantCount)
	}
	if assistantWithToolCalls == nil {
		t.Fatal("merged assistant message should have ToolCalls")
	}
	if assistantWithToolCalls.Content == nil || assistantWithToolCalls.Content.Text == "" {
		t.Error("merged assistant message should have Content from the text message")
	}
}
