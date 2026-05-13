// Package model_body 测试确保 marshal/unmarshal 往返不丢失数据
//
// 背景：traceID 7a7d9511-fc52-4346-ac77-f3b6a4b5303f 中 system message 在接收时为 35671 字符，
// 但在上游请求体中变为 30457 字符，丢失约 5214 字符。本测试套件验证 Go DTO 层的 marshal/unmarshal
// 往返是无损的，定位问题的根因不在 DTO 层。
package model_body

import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
)

// TestMarshalOpenAIChatCompletionBody_UpstreamModel 验证模型名正确替换且不修改原请求
func TestMarshalOpenAIChatCompletionBody_UpstreamModel(t *testing.T) {
	req := &dto.OpenAIChatCompletionReq{
		Model: "exposed-chat-model",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "hi"}},
		},
	}

	body := util.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-chat-model")
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

// TestMarshalOpenAIChatCompletionBody_SystemMessagePreserved 验证超长 system message 在 marshal 后完整保留
func TestMarshalOpenAIChatCompletionBody_SystemMessagePreserved(t *testing.T) {
	longText := strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*() ", 500)
	expectedLen := len(longText)

	req := &dto.OpenAIChatCompletionReq{
		Model: "test-model",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleSystem, Content: &dto.OpenAIMessageContent{Text: longText}},
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "hello"}},
		},
	}

	body := util.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-model")

	var unmarshalled dto.OpenAIChatCompletionReq
	if err := sonic.Unmarshal(body, &unmarshalled); err != nil {
		t.Fatalf("failed to unmarshal upstream body: %v", err)
	}

	if unmarshalled.Messages[0].Content == nil {
		t.Fatal("system message content is nil after round-trip")
	}

	gotLen := len(unmarshalled.Messages[0].Content.Text)
	if gotLen != expectedLen {
		t.Fatalf("system message LENGTH MISMATCH: original=%d, after-roundtrip=%d (lost %d chars)",
			expectedLen, gotLen, expectedLen-gotLen)
	}

	if unmarshalled.Messages[0].Content.Text != longText {
		t.Fatal("system message CONTENT MISMATCH after round-trip")
	}
}

// TestMarshalOpenAIChatCompletionBody_FullRoundTripRawJSON 验证从原始 JSON 到 struct 再回到 JSON 的完整往返
func TestMarshalOpenAIChatCompletionBody_FullRoundTripRawJSON(t *testing.T) {
	longText := strings.Repeat("A Very Long System Prompt That Must Survive JSON Serialization. ", 720)
	fullLength := len(longText)

	originalJSON := `{"model":"test-model","messages":[{"role":"system","content":"` + longText + `"},{"role":"user","content":"hello"}],"stream":true,"max_completion_tokens":32000}`

	var parsed dto.OpenAIChatCompletionReq
	if err := sonic.Unmarshal([]byte(originalJSON), &parsed); err != nil {
		t.Fatalf("failed to unmarshal original JSON: %v", err)
	}

	if parsed.Messages[0].Content == nil || parsed.Messages[0].Content.Text != longText {
		t.Fatalf("system message content lost during initial parse: got len=%d, want=%d",
			len(parsed.Messages[0].Content.Text), fullLength)
	}

	body := util.MarshalOpenAIChatCompletionBodyForModel(&parsed, "upstream-model")

	var roundTripped dto.OpenAIChatCompletionReq
	if err := sonic.Unmarshal(body, &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal round-tripped body: %v", err)
	}

	if roundTripped.Messages[0].Content == nil {
		t.Fatal("system message content is nil after full round-trip")
	}

	gotLen := len(roundTripped.Messages[0].Content.Text)
	if gotLen != fullLength {
		t.Fatalf("system message LENGTH MISMATCH after full round-trip:\n  original len=%d\n  after len=%d\n  lost=%d chars",
			fullLength, gotLen, fullLength-gotLen)
	}

	if roundTripped.Messages[0].Content.Text != longText {
		t.Fatalf("system message content changed after full round-trip")
	}
}

// TestSonicMapUnmarshal_PreservesLongStrings 验证 sonic 在 map[string]any 中也能完整保留超长字符串
// （LogMiddleware 和 Huma 使用了不同的 unmarshal 目标类型）
func TestSonicMapUnmarshal_PreservesLongStrings(t *testing.T) {
	longText := strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*() ", 500)
	fullLength := len(longText)

	rawJSON := `{"model":"test","messages":[{"role":"system","content":"` + longText + `"}]}`

	var asMap map[string]any
	if err := sonic.Unmarshal([]byte(rawJSON), &asMap); err != nil {
		t.Fatalf("failed to unmarshal into map: %v", err)
	}

	messages, ok := asMap["messages"].([]any)
	if !ok || len(messages) == 0 {
		t.Fatal("no messages in map")
	}

	firstMsg, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatal("first message is not a map")
	}

	content, ok := firstMsg["content"].(string)
	if !ok {
		t.Fatalf("content is not a string, got %T", firstMsg["content"])
	}

	if len(content) != fullLength {
		t.Fatalf("map[string]any unmarshal LENGTH MISMATCH: expected=%d, got=%d (lost %d chars)",
			fullLength, len(content), fullLength-len(content))
	}

	if content != longText {
		t.Fatal("map[string]any unmarshal CONTENT MISMATCH")
	}
}

// TestOpenAIChatCompletionReq_MessagesWithMixedContent 验证混合消息类型（string content / parts content）的往返
func TestOpenAIChatCompletionReq_MessagesWithMixedContent(t *testing.T) {
	text := "Hello, this is a system message with some content."
	req := &dto.OpenAIChatCompletionReq{
		Model: "test-model",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleSystem, Content: &dto.OpenAIMessageContent{Text: text}},
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Parts: []*dto.OpenAIChatCompletionContentPart{
				{Type: "text", Text: lo.ToPtr("user message")},
			}}},
		},
	}

	body := util.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-model")

	var unmarshalled dto.OpenAIChatCompletionReq
	if err := sonic.Unmarshal(body, &unmarshalled); err != nil {
		t.Fatalf("failed to unmarshal upstream body: %v", err)
	}

	if unmarshalled.Messages[0].Content == nil || unmarshalled.Messages[0].Content.Text != text {
		t.Fatalf("system message text not preserved, got: %v", unmarshalled.Messages[0].Content)
	}

	if unmarshalled.Messages[1].Content == nil || len(unmarshalled.Messages[1].Content.Parts) != 1 {
		t.Fatalf("user message parts not preserved, got: %v", unmarshalled.Messages[1].Content)
	}
}

// TestMarshalOpenAIChatCompletionBody_ComplexRequestPreserved 验证复杂请求体所有字段的往返
func TestMarshalOpenAIChatCompletionBody_ComplexRequestPreserved(t *testing.T) {
	longText := strings.Repeat("System instruction with important details. ", 200)

	req := &dto.OpenAIChatCompletionReq{
		Model: "test-model",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleSystem, Content: &dto.OpenAIMessageContent{Text: longText}},
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "my question"}},
		},
		MaxCompletionTokens: intPtr(4096),
		Stream:              boolPtr(true),
		ReasoningEffort:     enum.ReasoningEffortMedium,
	}

	body := util.MarshalOpenAIChatCompletionBodyForModel(req, "upstream-model")

	var unmarshalled dto.OpenAIChatCompletionReq
	if err := sonic.Unmarshal(body, &unmarshalled); err != nil {
		t.Fatalf("failed to unmarshal upstream body: %v", err)
	}

	if unmarshalled.Messages[0].Content == nil || unmarshalled.Messages[0].Content.Text != longText {
		t.Fatalf("system message not preserved")
	}

	if unmarshalled.MaxCompletionTokens == nil || *unmarshalled.MaxCompletionTokens != 4096 {
		t.Fatal("MaxCompletionTokens not preserved")
	}
	if unmarshalled.Stream == nil || *unmarshalled.Stream != true {
		t.Fatal("Stream not preserved")
	}
	if unmarshalled.ReasoningEffort != enum.ReasoningEffortMedium {
		t.Fatal("ReasoningEffort not preserved")
	}
}

func intPtr(i int) *int    { return &i }
func boolPtr(b bool) *bool { return &b }
