package model_body

import (
	"strings"
	"testing"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

func TestMarshalOpenAIChatCompletionBodyForModel_UsesUpstreamModelWithoutMutatingRequest(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestMarshalOpenAIResponseBodyForModel_NormalizesInputItemSummaryWithoutMutatingRequest(t *testing.T) {
	t.Parallel()
	summary := []*dto.ResponseReasoningSummary{{
		Type: enum.ResponseContentTypeSummaryText,
		Text: "thinking summary",
	}}
	messageSummary := []*dto.ResponseReasoningSummary{{
		Type: enum.ResponseContentTypeSummaryText,
		Text: "invalid message summary",
	}}
	req := &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("gpt-5.5"),
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{
				{
					Type:    lo.ToPtr(enum.ResponseInputItemTypeMessage),
					Role:    lo.ToPtr(enum.RoleUser),
					Summary: &messageSummary,
					Content: &dto.ResponseInputMessageContent{
						Text: "Hello",
					},
				},
				{
					Type:    lo.ToPtr(enum.ResponseInputItemTypeReasoning),
					Status:  lo.ToPtr("completed"),
					Summary: &summary,
				},
				{
					Type:   lo.ToPtr(enum.ResponseInputItemTypeReasoning),
					Status: lo.ToPtr("completed"),
				},
			},
		},
	}

	body := proxyutil.MarshalOpenAIResponseBodyForModel(req, "upstream-model")
	bodyStr := string(body)

	if strings.Contains(bodyStr, `"invalid message summary"`) {
		t.Fatalf("serialized body must not include message item summary, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"summary":[{"text":"thinking summary","type":"summary_text"}]`) {
		t.Fatalf("serialized body must keep reasoning item summary, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"summary":[]`) {
		t.Fatalf("serialized body must include empty summary for reasoning item without summary, got: %s", bodyStr)
	}
	if req.Input.Items[0].Summary == nil || len(*req.Input.Items[0].Summary) != 1 {
		t.Fatalf("request message item summary must remain unchanged")
	}
	if req.Input.Items[1].Summary == nil || len(*req.Input.Items[1].Summary) != 1 {
		t.Fatalf("request reasoning item summary must remain unchanged")
	}
	if req.Input.Items[2].Summary != nil {
		t.Fatalf("request reasoning item without summary must remain nil")
	}
}

func TestMarshalAnthropicMessageBodyForModel_UsesUpstreamModelWithoutMutatingRequest(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
