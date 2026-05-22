package model_body

import (
	"strings"
	"testing"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
)

func TestMarshalOpenAIChatCompletionBodyForModel_UsesUpstreamModelWithoutMutatingRequest(t *testing.T) {
	req := &dto.OpenAIChatCompletionReq{
		Model: "exposed-chat-model",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "hi"}},
		},
	}

	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-chat-model")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"model":"upstream-chat-model"`) {
		t.Fatalf("upstream body must use upstream model, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, `"model":"exposed-chat-model"`) {
		t.Fatalf("upstream body must not use exposed model, got: %s", bodyStr)
	}
	if req.Model != "exposed-chat-model" {
		t.Fatalf("request model must remain exposed model, got: %s", req.Model)
	}
}

func TestMarshalOpenAIResponseBodyForModel_UsesUpstreamModelWithoutMutatingRequest(t *testing.T) {
	req := &dto.OpenAICreateResponseReq{Model: lo.ToPtr("exposed-response-model")}

	body := proxyutil.MarshalOpenAIResponseBodyForModel(req, "upstream-response-model")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"model":"upstream-response-model"`) {
		t.Fatalf("upstream body must use upstream model, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, `"model":"exposed-response-model"`) {
		t.Fatalf("upstream body must not use exposed model, got: %s", bodyStr)
	}
	if lo.FromPtr(req.Model) != "exposed-response-model" {
		t.Fatalf("request model must remain exposed model, got: %s", lo.FromPtr(req.Model))
	}
}

func TestMarshalAnthropicMessageBodyForModel_UsesUpstreamModelWithoutMutatingRequest(t *testing.T) {
	req := &dto.AnthropicCreateMessageReq{
		Model:     "exposed-anthropic-model",
		MaxTokens: 1024,
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "hi"}},
		},
	}

	body := proxyutil.MarshalAnthropicMessageBodyForModel(req, "upstream-anthropic-model")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"model":"upstream-anthropic-model"`) {
		t.Fatalf("upstream body must use upstream model, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, `"model":"exposed-anthropic-model"`) {
		t.Fatalf("upstream body must not use exposed model, got: %s", bodyStr)
	}
	if req.Model != "exposed-anthropic-model" {
		t.Fatalf("request model must remain exposed model, got: %s", req.Model)
	}
}

func TestMarshalAnthropicCountTokensBodyForModel_UsesUpstreamModelWithoutMutatingRequest(t *testing.T) {
	req := &dto.AnthropicCountTokensReq{
		Model: "exposed-count-model",
		Messages: []*dto.AnthropicMessageParam{
			{Role: "user", Content: &dto.AnthropicMessageContent{Text: "hi"}},
		},
	}

	body := proxyutil.MarshalAnthropicCountTokensBodyForModel(req, "upstream-count-model")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"model":"upstream-count-model"`) {
		t.Fatalf("upstream body must use upstream model, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, `"model":"exposed-count-model"`) {
		t.Fatalf("upstream body must not use exposed model, got: %s", bodyStr)
	}
	if req.Model != "exposed-count-model" {
		t.Fatalf("request model must remain exposed model, got: %s", req.Model)
	}
}
