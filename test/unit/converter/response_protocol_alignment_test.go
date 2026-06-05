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
