package model_body

import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestMarshalOpenAIChatCompletionBodyForModel_IncludesThinkingParam(t *testing.T) {
	t.Parallel()
	req := &dto.OpenAIChatCompletionReq{
		Model: "deepseek-v4-flash",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "hi"}},
		},
		Thinking: &dto.OpenAIThinkingParam{Type: "enabled"},
	}

	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-chat-model")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"thinking":{"type":"enabled"}`) {
		t.Fatalf("upstream body must include thinking param, got: %s", bodyStr)
	}
}

func TestMarshalOpenAIChatCompletionBodyForModel_OmitsThinkingParamWhenNil(t *testing.T) {
	t.Parallel()
	req := &dto.OpenAIChatCompletionReq{
		Model: "deepseek-v4-flash",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "hi"}},
		},
	}

	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-chat-model")
	bodyStr := string(body)

	if strings.Contains(bodyStr, `"thinking"`) {
		t.Fatalf("upstream body must not include thinking param when nil, got: %s", bodyStr)
	}
}

func TestUnmarshalOpenAIChatCompletionReq_WithThinkingParam(t *testing.T) {
	t.Parallel()
	raw := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"enabled"}}`

	req := &dto.OpenAIChatCompletionReq{}
	if err := sonic.Unmarshal([]byte(raw), req); err != nil {
		t.Fatalf("failed to unmarshal with thinking param: %v", err)
	}

	if req.Thinking == nil {
		t.Fatal("thinking param must not be nil")
	}
	if req.Thinking.Type != "enabled" {
		t.Fatalf("expected thinking type 'enabled', got: %s", req.Thinking.Type)
	}
}

func TestUnmarshalOpenAIChatCompletionReq_WithoutThinkingParam(t *testing.T) {
	t.Parallel()
	raw := `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}]}`

	req := &dto.OpenAIChatCompletionReq{}
	if err := sonic.Unmarshal([]byte(raw), req); err != nil {
		t.Fatalf("failed to unmarshal without thinking param: %v", err)
	}

	if req.Thinking != nil {
		t.Fatal("thinking param must be nil when not provided")
	}
}
