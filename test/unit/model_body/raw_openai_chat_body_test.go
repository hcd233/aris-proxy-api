package model_body

import (
	"strings"
	"testing"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
)

func TestMarshalRawOpenAIChatCompletionBodyForModel_PreservesUnknownFields(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"model":"exposed-chat-model","messages":[{"role":"user","content":"hi","unknown_message_field":{"keep":true}}],"unknown_top":{"nested":true},"null_field":null,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object","x-extra":false}}}]}`)

	body := proxyutil.MarshalRawOpenAIChatCompletionBodyForModel(raw, "upstream-chat-model")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"model":"upstream-chat-model"`) {
		t.Fatalf("upstream body must use upstream model, got: %s", bodyStr)
	}
}
