package model_body

import (
	"strings"
	"testing"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestMarshalRawOpenAIChatCompletionBodyForModel_PreservesUnknownFields(t *testing.T) {
	raw := []byte(`{"model":"exposed-chat-model","messages":[{"role":"user","content":"hi","unknown_message_field":{"keep":true}}],"unknown_top":{"nested":true},"null_field":null,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object","x-extra":false}}}]}`)
	req := &dto.OpenAIChatCompletionReq{
		Model: "exposed-chat-model",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "hi"}},
		},
	}

	body := proxyutil.MarshalRawOpenAIChatCompletionBodyForModel(raw, req, "upstream-chat-model")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"model":"upstream-chat-model"`) {
		t.Fatalf("upstream body must use upstream model, got: %s", bodyStr)
	}
	if req.Model != "exposed-chat-model" {
		t.Fatalf("request model must remain exposed model, got: %s", req.Model)
	}
}
