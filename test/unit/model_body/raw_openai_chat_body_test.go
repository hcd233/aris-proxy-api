package model_body

import (
	"strings"
	"testing"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
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
	if util.HashJSONBodyExcludingTopLevelModel(raw) != util.HashJSONBodyExcludingTopLevelModel(body) {
		t.Fatalf("raw body fields other than model must be preserved\nraw: %s\nbody: %s", string(raw), bodyStr)
	}
	if req.Model != "exposed-chat-model" {
		t.Fatalf("request model must remain exposed model, got: %s", req.Model)
	}
}
