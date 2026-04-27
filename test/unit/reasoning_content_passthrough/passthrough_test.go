package reasoning_content_passthrough

import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type testCase struct {
	Name              string        `json:"name"`
	Description       string        `json:"description"`
	Messages          []testMessage `json:"messages"`
	ExpectedReasoning string        `json:"expectedReasoning"`
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
			ExpectedReasoning: "This is my reasoning for the answer with 158 chars...",
		},
		{
			Name:        "assistant_no_reasoning",
			Description: "Assistant message without reasoning_content",
			Messages: []testMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
			ExpectedReasoning: "",
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

			if tc.ExpectedReasoning != "" {
				expected := `"reasoning_content":"` + tc.ExpectedReasoning + `"`
				if !strings.Contains(bodyStr, expected) {
					t.Errorf("expected %s in serialized body\nBody: %s", expected, bodyStr)
				}
			} else {
				// When ExpectedReasoning is empty, omitempty omits the field entirely
				if strings.Contains(bodyStr, "reasoning_content") {
					t.Errorf("unexpected reasoning_content in serialized body (should be omitted by omitempty)\nBody: %s", bodyStr)
				}
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

func TestEnsureAssistantMessageReasoningContent(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "assistant_with_tool_calls_no_reasoning",
			input:    `{"messages":[{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function"}]}]}`,
			expected: `reasoning_content`,
		},
		{
			name:     "assistant_with_tool_calls_has_reasoning",
			input:    `{"messages":[{"role":"assistant","content":"","reasoning_content":"think","tool_calls":[{"id":"call_1"}]}]}`,
			expected: `"reasoning_content":"think"`,
		},
		{
			name:     "assistant_without_tool_calls",
			input:    `{"messages":[{"role":"assistant","content":"hi"}]}`,
			expected: ``,
		},
		{
			name:     "user_message_ignored",
			input:    `{"messages":[{"role":"user","content":"hi"}]}`,
			expected: ``,
		},
		{
			name:     "mixed_messages",
			input:    `{"messages":[{"role":"user","content":"hi"},{"role":"assistant","tool_calls":[{"id":"c1"}]},{"role":"assistant","content":"ok"}]}`,
			expected: `reasoning_content`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := util.EnsureAssistantMessageReasoningContent([]byte(tc.input))
			gotStr := string(got)
			if tc.expected != "" && !strings.Contains(gotStr, tc.expected) {
				t.Errorf("expected %q in output, got %s", tc.expected, gotStr)
			}
			// Verify valid JSON
			var v any
			if err := sonic.Unmarshal(got, &v); err != nil {
				t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, gotStr)
			}
		})
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
