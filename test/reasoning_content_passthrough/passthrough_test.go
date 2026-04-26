package reasoning_content_passthrough

import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

type testCase struct {
	Name                string        `json:"name"`
	Description         string        `json:"description"`
	Messages            []testMessage `json:"messages"`
	ShouldHaveReasoning bool          `json:"shouldHaveReasoning"`
}

type testMessage struct {
	Role             string `json:"role"`
	Content          any    `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	ToolCalls        []any  `json:"tool_calls,omitempty"`
}

func loadCases(t *testing.T) []testCase {
	t.Helper()
	return []testCase{
		{
			Name:        "assistant_has_reasoning_content",
			Description: "Assistant message with reasoning_content and tool_calls",
			Messages: []testMessage{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hello"},
				{
					Role:             "assistant",
					Content:          "",
					ReasoningContent: "This is my reasoning for the answer with 158 chars...",
					ToolCalls:        []any{map[string]any{"id": "call_123", "type": "function"}},
				},
			},
			ShouldHaveReasoning: true,
		},
		{
			Name:        "assistant_no_reasoning",
			Description: "Assistant message without reasoning_content",
			Messages: []testMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
			ShouldHaveReasoning: false,
		},
	}
}

func TestReasoningContentPreservedInSerializedBody(t *testing.T) {
	for _, tc := range loadCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			reqBody := buildReqBody(t, tc.Messages)
			marshaled, err := sonic.Marshal(reqBody)
			if err != nil {
				t.Fatalf("sonic.Marshal error: %v", err)
			}
			bodyStr := string(marshaled)
			hasRC := strings.Contains(bodyStr, "reasoning_content")

			if tc.ShouldHaveReasoning && !hasRC {
				t.Errorf("expected reasoning_content in serialized body, got none\nBody: %s", bodyStr)
			}
			if !tc.ShouldHaveReasoning && hasRC {
				t.Errorf("unexpected reasoning_content in serialized body\nBody: %s", bodyStr)
			}
		})
	}
}

func TestReasoningContentWithToolCallsPreserved(t *testing.T) {
	reqBody := &dto.OpenAIChatCompletionReq{
		Model: "test-model",
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{
				Role:    enum.RoleUser,
				Content: &dto.OpenAIMessageContent{Text: "Hello"},
			},
			{
				Role:             enum.RoleAssistant,
				Content:          &dto.OpenAIMessageContent{Text: ""},
				ReasoningContent: "some reasoning",
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{
					{ID: "call_1", Type: enum.ToolTypeFunction, Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{Name: "test", Arguments: "{}"}},
				},
			},
		},
	}

	marshaled, err := sonic.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	bodyStr := string(marshaled)

	if !strings.Contains(bodyStr, "reasoning_content") {
		t.Fatalf("reasoning_content should be preserved in serialized body: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "tool_calls") {
		t.Fatalf("tool_calls should be preserved in serialized body: %s", bodyStr)
	}
}

func buildReqBody(t *testing.T, msgs []testMessage) *dto.OpenAIChatCompletionReq {
	t.Helper()
	req := &dto.OpenAIChatCompletionReq{Model: "test-model"}
	for _, m := range msgs {
		msg := &dto.OpenAIChatCompletionMessageParam{
			Role:             enum.Role(m.Role),
			ReasoningContent: m.ReasoningContent,
		}
		if m.Content != nil {
			if s, ok := m.Content.(string); ok {
				msg.Content = &dto.OpenAIMessageContent{Text: s}
			}
		}
		req.Messages = append(req.Messages, msg)
	}
	return req
}
