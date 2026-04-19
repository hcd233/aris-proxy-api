package converter

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/converter"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
)

// testCase 测试用例结构
type testCase struct {
	Name                string                                   `json:"name"`
	Description         string                                   `json:"description"`
	OpenAIRequest       *dto.OpenAIChatCompletionReq             `json:"openai_request"`
	AnthropicRequest    *dto.AnthropicCreateMessageReq           `json:"anthropic_request"`
	OpenAIResponse      *dto.OpenAIChatCompletion                `json:"openai_response"`
	AnthropicResponse   *dto.AnthropicMessage                    `json:"anthropic_response"`
	OpenAIToolChoice    *dto.OpenAIChatCompletionToolChoiceParam `json:"openai_tool_choice"`
	AnthropicToolChoice *dto.AnthropicToolChoice                 `json:"anthropic_tool_choice"`
}

func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

func findCase(t *testing.T, cases []testCase, name string) testCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return testCase{}
}

// ==================== OpenAI -> Anthropic (AnthropicProtocolConverter) ====================

func TestAnthropicProtocolConverter_FromOpenAIRequest_SimpleText(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "simple_text_message")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromOpenAIRequest(tc.OpenAIRequest)
	if err != nil {
		t.Fatalf("FromOpenAIRequest() error: %v", err)
	}

	// 检查 model
	if result.Model != tc.OpenAIRequest.Model {
		t.Errorf("Model = %q, want %q", result.Model, tc.OpenAIRequest.Model)
	}

	// 检查 max_tokens
	if result.MaxTokens != *tc.OpenAIRequest.MaxCompletionTokens {
		t.Errorf("MaxTokens = %d, want %d", result.MaxTokens, *tc.OpenAIRequest.MaxCompletionTokens)
	}

	// 检查 system 被提取
	if result.System == nil {
		t.Fatal("System should not be nil")
	}
	if result.System.Text != "You are a helpful assistant." {
		t.Errorf("System.Text = %q, want %q", result.System.Text, "You are a helpful assistant.")
	}

	// 检查 messages 不包含 system
	for _, msg := range result.Messages {
		if msg.Role == enum.RoleSystem || msg.Role == enum.RoleDeveloper {
			t.Error("Messages should not contain system/developer role after extraction")
		}
	}

	// 检查 temperature
	if result.Temperature == nil || *result.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", result.Temperature)
	}

	// 检查消息数量：user, assistant, user（不含 system）
	if len(result.Messages) != 3 {
		t.Errorf("len(Messages) = %d, want 3", len(result.Messages))
	}
}

func TestAnthropicProtocolConverter_FromOpenAIRequest_ToolCallFlow(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "tool_call_flow")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromOpenAIRequest(tc.OpenAIRequest)
	if err != nil {
		t.Fatalf("FromOpenAIRequest() error: %v", err)
	}

	// 检查工具定义
	if len(result.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(result.Tools))
	}
	if result.Tools[0].Name != "get_weather" {
		t.Errorf("Tools[0].Name = %q, want %q", result.Tools[0].Name, "get_weather")
	}
	if result.Tools[0].InputSchema == nil {
		t.Error("Tools[0].InputSchema should not be nil")
	}

	// 检查 tool_choice
	if result.ToolChoice == nil {
		t.Fatal("ToolChoice should not be nil")
	}
	if result.ToolChoice.Type != "auto" {
		t.Errorf("ToolChoice.Type = %q, want %q", result.ToolChoice.Type, "auto")
	}

	// 检查 assistant 消息的 tool_use 块
	found := false
	for _, msg := range result.Messages {
		if msg.Role == enum.RoleAssistant && msg.Content != nil && len(msg.Content.Blocks) > 0 {
			for _, block := range msg.Content.Blocks {
				if block.Type == enum.AnthropicContentBlockTypeToolUse {
					found = true
					if block.Name != "get_weather" {
						t.Errorf("tool_use block Name = %q, want %q", block.Name, "get_weather")
					}
					if block.ID != "call_abc123" {
						t.Errorf("tool_use block ID = %q, want %q", block.ID, "call_abc123")
					}
				}
			}
		}
	}
	if !found {
		t.Error("Expected to find a tool_use block in assistant message")
	}

	// 检查 tool_result 在 user 消息中
	foundToolResult := false
	for _, msg := range result.Messages {
		if msg.Role == enum.RoleUser && msg.Content != nil && len(msg.Content.Blocks) > 0 {
			for _, block := range msg.Content.Blocks {
				if block.Type == enum.AnthropicContentBlockTypeToolResult {
					foundToolResult = true
					if block.ToolUseID != "call_abc123" {
						t.Errorf("tool_result ToolUseID = %q, want %q", block.ToolUseID, "call_abc123")
					}
				}
			}
		}
	}
	if !foundToolResult {
		t.Error("Expected to find a tool_result block in user message")
	}
}

func TestAnthropicProtocolConverter_FromOpenAIRequest_StopSequences(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "stop_sequences")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromOpenAIRequest(tc.OpenAIRequest)
	if err != nil {
		t.Fatalf("FromOpenAIRequest() error: %v", err)
	}

	if len(result.StopSequences) != 2 {
		t.Fatalf("len(StopSequences) = %d, want 2", len(result.StopSequences))
	}
	if result.StopSequences[0] != "END" || result.StopSequences[1] != "STOP" {
		t.Errorf("StopSequences = %v, want [END, STOP]", result.StopSequences)
	}
}

func TestAnthropicProtocolConverter_FromOpenAIRequest_ToolChoiceRequired(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "tool_choice_required")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromOpenAIRequest(tc.OpenAIRequest)
	if err != nil {
		t.Fatalf("FromOpenAIRequest() error: %v", err)
	}

	if result.ToolChoice == nil {
		t.Fatal("ToolChoice should not be nil")
	}
	if result.ToolChoice.Type != "any" {
		t.Errorf("ToolChoice.Type = %q, want %q", result.ToolChoice.Type, "any")
	}
}

func TestAnthropicProtocolConverter_FromOpenAIRequest_ImageContent(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "image_content")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromOpenAIRequest(tc.OpenAIRequest)
	if err != nil {
		t.Fatalf("FromOpenAIRequest() error: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(result.Messages))
	}

	msg := result.Messages[0]
	if msg.Content == nil || len(msg.Content.Blocks) < 2 {
		t.Fatal("Expected at least 2 content blocks")
	}

	// 检查 text block
	if msg.Content.Blocks[0].Type != enum.AnthropicContentBlockTypeText {
		t.Errorf("Blocks[0].Type = %q, want %q", msg.Content.Blocks[0].Type, enum.AnthropicContentBlockTypeText)
	}

	// 检查 image block
	if msg.Content.Blocks[1].Type != enum.AnthropicContentBlockTypeImage {
		t.Errorf("Blocks[1].Type = %q, want %q", msg.Content.Blocks[1].Type, enum.AnthropicContentBlockTypeImage)
	}
	if msg.Content.Blocks[1].Source == nil {
		t.Fatal("Image source should not be nil")
	}
	if msg.Content.Blocks[1].Source.Type != "base64" {
		t.Errorf("Image source type = %q, want %q", msg.Content.Blocks[1].Source.Type, "base64")
	}
	if msg.Content.Blocks[1].Source.MediaType != "image/png" {
		t.Errorf("Image media type = %q, want %q", msg.Content.Blocks[1].Source.MediaType, "image/png")
	}
}

// ==================== Anthropic -> OpenAI (OpenAIProtocolConverter) ====================

func TestOpenAIProtocolConverter_FromAnthropicRequest_SimpleText(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "simple_text_message")

	conv := converter.OpenAIProtocolConverter{}
	result, err := conv.FromAnthropicRequest(tc.AnthropicRequest)
	if err != nil {
		t.Fatalf("FromAnthropicRequest() error: %v", err)
	}

	// 检查 model
	if result.Model != tc.AnthropicRequest.Model {
		t.Errorf("Model = %q, want %q", result.Model, tc.AnthropicRequest.Model)
	}

	// 检查 max_completion_tokens
	if result.MaxCompletionTokens == nil || *result.MaxCompletionTokens != tc.AnthropicRequest.MaxTokens {
		t.Errorf("MaxCompletionTokens = %v, want %d", result.MaxCompletionTokens, tc.AnthropicRequest.MaxTokens)
	}

	// 检查 system 消息被创建
	if len(result.Messages) == 0 {
		t.Fatal("Messages should not be empty")
	}
	if result.Messages[0].Role != enum.RoleSystem {
		t.Errorf("Messages[0].Role = %q, want %q", result.Messages[0].Role, enum.RoleSystem)
	}
	if result.Messages[0].Content == nil || result.Messages[0].Content.Text != "You are a helpful assistant." {
		t.Error("System message content mismatch")
	}

	// 检查 temperature
	if result.Temperature == nil || *result.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", result.Temperature)
	}

	// messages: system + user + assistant + user = 4
	if len(result.Messages) != 4 {
		t.Errorf("len(Messages) = %d, want 4", len(result.Messages))
	}
}

func TestOpenAIProtocolConverter_FromAnthropicRequest_ToolCallFlow(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "tool_call_flow")

	conv := converter.OpenAIProtocolConverter{}
	result, err := conv.FromAnthropicRequest(tc.AnthropicRequest)
	if err != nil {
		t.Fatalf("FromAnthropicRequest() error: %v", err)
	}

	// 检查工具定义
	if len(result.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(result.Tools))
	}
	if result.Tools[0].Type != enum.ToolTypeFunction {
		t.Errorf("Tools[0].Type = %q, want %q", result.Tools[0].Type, enum.ToolTypeFunction)
	}
	if result.Tools[0].Function == nil {
		t.Fatal("Tools[0].Function should not be nil")
	}
	if result.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tools[0].Function.Name = %q, want %q", result.Tools[0].Function.Name, "get_weather")
	}

	// 检查 tool_choice
	if result.ToolChoice == nil {
		t.Fatal("ToolChoice should not be nil")
	}
	if result.ToolChoice.Mode != enum.ToolChoiceAuto {
		t.Errorf("ToolChoice.Mode = %q, want %q", result.ToolChoice.Mode, enum.ToolChoiceAuto)
	}

	// 检查 assistant 消息有 tool_calls
	foundToolCall := false
	for _, msg := range result.Messages {
		if msg.Role == enum.RoleAssistant && len(msg.ToolCalls) > 0 {
			foundToolCall = true
			if msg.ToolCalls[0].Function.Name != "get_weather" {
				t.Errorf("ToolCalls[0].Function.Name = %q, want %q", msg.ToolCalls[0].Function.Name, "get_weather")
			}
		}
	}
	if !foundToolCall {
		t.Error("Expected to find tool_calls in assistant message")
	}

	// 检查 tool 消息
	foundToolMsg := false
	for _, msg := range result.Messages {
		if msg.Role == enum.RoleTool {
			foundToolMsg = true
			if msg.ToolCallID != "call_abc123" {
				t.Errorf("ToolCallID = %q, want %q", msg.ToolCallID, "call_abc123")
			}
		}
	}
	if !foundToolMsg {
		t.Error("Expected to find a tool role message")
	}
}

func TestOpenAIProtocolConverter_FromAnthropicRequest_ImageContent(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "image_content")

	conv := converter.OpenAIProtocolConverter{}
	result, err := conv.FromAnthropicRequest(tc.AnthropicRequest)
	if err != nil {
		t.Fatalf("FromAnthropicRequest() error: %v", err)
	}

	// 找到 user 消息
	if len(result.Messages) < 1 {
		t.Fatal("Expected at least 1 message")
	}

	userMsg := result.Messages[0]
	if userMsg.Content == nil {
		t.Fatal("User message content should not be nil")
	}

	// 应该有 Parts（多部分内容）
	if len(userMsg.Content.Parts) < 2 {
		t.Fatalf("Expected at least 2 content parts, got %d", len(userMsg.Content.Parts))
	}

	// 检查 text part
	if userMsg.Content.Parts[0].Type != enum.ContentPartTypeText {
		t.Errorf("Parts[0].Type = %q, want %q", userMsg.Content.Parts[0].Type, enum.ContentPartTypeText)
	}

	// 检查 image part
	if userMsg.Content.Parts[1].Type != enum.ContentPartTypeImageURL {
		t.Errorf("Parts[1].Type = %q, want %q", userMsg.Content.Parts[1].Type, enum.ContentPartTypeImageURL)
	}
	if userMsg.Content.Parts[1].ImageURL == nil {
		t.Fatal("Image URL should not be nil")
	}
}

// ==================== Response Conversion Tests ====================

func TestAnthropicProtocolConverter_ToOpenAIResponse_WithReasoning(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "reasoning_content")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.ToOpenAIResponse(tc.AnthropicResponse)
	if err != nil {
		t.Fatalf("ToOpenAIResponse() error: %v", err)
	}

	if result.ID != tc.AnthropicResponse.ID {
		t.Errorf("ID = %q, want %q", result.ID, tc.AnthropicResponse.ID)
	}
	if result.Object != "chat.completion" {
		t.Errorf("Object = %q, want %q", result.Object, "chat.completion")
	}

	if len(result.Choices) != 1 {
		t.Fatalf("len(Choices) = %d, want 1", len(result.Choices))
	}

	choice := result.Choices[0]
	if choice.FinishReason != enum.FinishReasonStop {
		t.Errorf("FinishReason = %q, want %q", choice.FinishReason, enum.FinishReasonStop)
	}

	if choice.Message == nil {
		t.Fatal("Message should not be nil")
	}
	if choice.Message.Content == nil || choice.Message.Content.Text != "x = -1" {
		t.Errorf("Message.Content.Text = %v, want %q", choice.Message.Content, "x = -1")
	}
	if choice.Message.ReasoningContent != "This is a perfect square: (x+1)^2 = 0" {
		t.Errorf("Message.ReasoningContent = %q, want %q", choice.Message.ReasoningContent, "This is a perfect square: (x+1)^2 = 0")
	}

	// 检查 usage
	if result.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("Usage.PromptTokens = %d, want 10", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 20 {
		t.Errorf("Usage.CompletionTokens = %d, want 20", result.Usage.CompletionTokens)
	}
}

func TestAnthropicProtocolConverter_ToOpenAIResponse_WithToolUse(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "tool_use_response")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.ToOpenAIResponse(tc.AnthropicResponse)
	if err != nil {
		t.Fatalf("ToOpenAIResponse() error: %v", err)
	}

	if len(result.Choices) != 1 {
		t.Fatalf("len(Choices) = %d, want 1", len(result.Choices))
	}

	choice := result.Choices[0]
	if choice.FinishReason != enum.FinishReasonToolCalls {
		t.Errorf("FinishReason = %q, want %q", choice.FinishReason, enum.FinishReasonToolCalls)
	}

	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(choice.Message.ToolCalls))
	}

	tc0 := choice.Message.ToolCalls[0]
	if tc0.ID != "call_xyz" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", tc0.ID, "call_xyz")
	}
	if tc0.Function == nil {
		t.Fatal("ToolCalls[0].Function should not be nil")
	}
	if tc0.Function.Name != "search" {
		t.Errorf("ToolCalls[0].Function.Name = %q, want %q", tc0.Function.Name, "search")
	}
}

func TestOpenAIProtocolConverter_ToAnthropicResponse_WithReasoning(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "reasoning_content")

	conv := converter.OpenAIProtocolConverter{}
	result, err := conv.ToAnthropicResponse(tc.OpenAIResponse)
	if err != nil {
		t.Fatalf("ToAnthropicResponse() error: %v", err)
	}

	if result.ID != tc.OpenAIResponse.ID {
		t.Errorf("ID = %q, want %q", result.ID, tc.OpenAIResponse.ID)
	}
	if result.Type != "message" {
		t.Errorf("Type = %q, want %q", result.Type, "message")
	}
	if result.Role != enum.RoleAssistant {
		t.Errorf("Role = %q, want %q", result.Role, enum.RoleAssistant)
	}

	// 检查 content blocks：应有 thinking + text
	if len(result.Content) < 2 {
		t.Fatalf("len(Content) = %d, want >= 2", len(result.Content))
	}

	// 第一个应该是 thinking
	if result.Content[0].Type != enum.AnthropicContentBlockTypeThinking {
		t.Errorf("Content[0].Type = %q, want %q", result.Content[0].Type, enum.AnthropicContentBlockTypeThinking)
	}
	if result.Content[0].Thinking != "This is a perfect square: (x+1)^2 = 0" {
		t.Errorf("Content[0].Thinking = %q, want %q", result.Content[0].Thinking, "This is a perfect square: (x+1)^2 = 0")
	}

	// 第二个应该是 text
	if result.Content[1].Type != enum.AnthropicContentBlockTypeText {
		t.Errorf("Content[1].Type = %q, want %q", result.Content[1].Type, enum.AnthropicContentBlockTypeText)
	}
	if result.Content[1].Text != "x = -1" {
		t.Errorf("Content[1].Text = %q, want %q", result.Content[1].Text, "x = -1")
	}

	// 检查 stop_reason
	if result.StopReason == nil || *result.StopReason != "end_turn" {
		t.Errorf("StopReason = %v, want %q", result.StopReason, "end_turn")
	}

	// 检查 usage
	if result.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if result.Usage.InputTokens != 10 {
		t.Errorf("Usage.InputTokens = %d, want 10", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 20 {
		t.Errorf("Usage.OutputTokens = %d, want 20", result.Usage.OutputTokens)
	}
}

func TestOpenAIProtocolConverter_ToAnthropicResponse_WithToolUse(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "tool_use_response")

	conv := converter.OpenAIProtocolConverter{}
	result, err := conv.ToAnthropicResponse(tc.OpenAIResponse)
	if err != nil {
		t.Fatalf("ToAnthropicResponse() error: %v", err)
	}

	if result.StopReason == nil || *result.StopReason != "tool_use" {
		t.Errorf("StopReason = %v, want %q", result.StopReason, "tool_use")
	}

	// 检查 content 中有 tool_use block
	foundToolUse := false
	for _, block := range result.Content {
		if block.Type == enum.AnthropicContentBlockTypeToolUse {
			foundToolUse = true
			if block.ID != "call_xyz" {
				t.Errorf("tool_use ID = %q, want %q", block.ID, "call_xyz")
			}
			if block.Name != "search" {
				t.Errorf("tool_use Name = %q, want %q", block.Name, "search")
			}
			query, ok := block.Input["query"]
			if !ok || query != "hello" {
				t.Errorf("tool_use Input = %v, want {query: hello}", block.Input)
			}
		}
	}
	if !foundToolUse {
		t.Error("Expected to find a tool_use content block")
	}
}

// ==================== Roundtrip Tests ====================

func TestRoundtrip_OpenAIRequest_ToAnthropic_BackToOpenAI(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "simple_text_message")

	anthropicConv := converter.AnthropicProtocolConverter{}
	openAIConv := converter.OpenAIProtocolConverter{}

	// OpenAI -> Anthropic
	anthropicReq, err := anthropicConv.FromOpenAIRequest(tc.OpenAIRequest)
	if err != nil {
		t.Fatalf("FromOpenAIRequest() error: %v", err)
	}

	// Anthropic -> OpenAI
	openAIReq, err := openAIConv.FromAnthropicRequest(anthropicReq)
	if err != nil {
		t.Fatalf("FromAnthropicRequest() error: %v", err)
	}

	// 验证关键字段保留
	if openAIReq.Model != tc.OpenAIRequest.Model {
		t.Errorf("Roundtrip Model = %q, want %q", openAIReq.Model, tc.OpenAIRequest.Model)
	}

	if openAIReq.MaxCompletionTokens == nil || *openAIReq.MaxCompletionTokens != *tc.OpenAIRequest.MaxCompletionTokens {
		t.Errorf("Roundtrip MaxCompletionTokens = %v, want %d", openAIReq.MaxCompletionTokens, *tc.OpenAIRequest.MaxCompletionTokens)
	}

	// 验证 system 消息恢复
	if len(openAIReq.Messages) == 0 || openAIReq.Messages[0].Role != enum.RoleSystem {
		t.Error("Roundtrip should restore system message")
	}
}

func TestRoundtrip_AnthropicRequest_ToOpenAI_BackToAnthropic(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "simple_text_message")

	openAIConv := converter.OpenAIProtocolConverter{}
	anthropicConv := converter.AnthropicProtocolConverter{}

	// Anthropic -> OpenAI
	openAIReq, err := openAIConv.FromAnthropicRequest(tc.AnthropicRequest)
	if err != nil {
		t.Fatalf("FromAnthropicRequest() error: %v", err)
	}

	// OpenAI -> Anthropic
	anthropicReq, err := anthropicConv.FromOpenAIRequest(openAIReq)
	if err != nil {
		t.Fatalf("FromOpenAIRequest() error: %v", err)
	}

	// 验证关键字段保留
	if anthropicReq.Model != tc.AnthropicRequest.Model {
		t.Errorf("Roundtrip Model = %q, want %q", anthropicReq.Model, tc.AnthropicRequest.Model)
	}

	if anthropicReq.MaxTokens != tc.AnthropicRequest.MaxTokens {
		t.Errorf("Roundtrip MaxTokens = %d, want %d", anthropicReq.MaxTokens, tc.AnthropicRequest.MaxTokens)
	}

	// 验证 system 恢复
	if anthropicReq.System == nil {
		t.Fatal("Roundtrip should restore system")
	}
	if anthropicReq.System.Text != "You are a helpful assistant." {
		t.Errorf("Roundtrip System.Text = %q, want %q", anthropicReq.System.Text, "You are a helpful assistant.")
	}

	// 验证消息数量不含 system
	if len(anthropicReq.Messages) != 3 {
		t.Errorf("Roundtrip len(Messages) = %d, want 3", len(anthropicReq.Messages))
	}
}

func TestRoundtrip_OpenAIResponse_ToAnthropic_BackToOpenAI(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "reasoning_content")

	openAIConv := converter.OpenAIProtocolConverter{}
	anthropicConv := converter.AnthropicProtocolConverter{}

	// OpenAI -> Anthropic
	anthropicResp, err := openAIConv.ToAnthropicResponse(tc.OpenAIResponse)
	if err != nil {
		t.Fatalf("ToAnthropicResponse() error: %v", err)
	}

	// Anthropic -> OpenAI
	openAIResp, err := anthropicConv.ToOpenAIResponse(anthropicResp)
	if err != nil {
		t.Fatalf("ToOpenAIResponse() error: %v", err)
	}

	if len(openAIResp.Choices) != 1 {
		t.Fatalf("Roundtrip len(Choices) = %d, want 1", len(openAIResp.Choices))
	}

	msg := openAIResp.Choices[0].Message
	if msg.Content == nil || msg.Content.Text != "x = -1" {
		t.Errorf("Roundtrip Content = %v, want 'x = -1'", msg.Content)
	}
	if msg.ReasoningContent != "This is a perfect square: (x+1)^2 = 0" {
		t.Errorf("Roundtrip ReasoningContent = %q, want %q", msg.ReasoningContent, "This is a perfect square: (x+1)^2 = 0")
	}
}

// ==================== SSE Conversion Tests ====================

func TestOpenAIProtocolConverter_ToAnthropicSSEResponse(t *testing.T) {
	conv := converter.OpenAIProtocolConverter{}

	chunk := &dto.OpenAIChatCompletionChunk{
		ID:      "chatcmpl-test",
		Object:  "chat.completion.chunk",
		Created: 1700000000,
		Model:   "gpt-4",
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{
				Content: "Hello",
			},
		}},
	}

	events, err := conv.ToAnthropicSSEResponse(chunk, true, "gpt-4", converter.NewSSEContentBlockTracker())
	if err != nil {
		t.Fatalf("ToAnthropicSSEResponse() error: %v", err)
	}

	// isFirst=true 应该有 message_start 事件
	foundMessageStart := false
	foundTextDelta := false
	for _, event := range events {
		if event.Event == enum.AnthropicSSEEventTypeMessageStart {
			foundMessageStart = true
		}
		if event.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
			foundTextDelta = true
		}
	}
	if !foundMessageStart {
		t.Error("Expected message_start event for isFirst=true")
	}
	if !foundTextDelta {
		t.Error("Expected content_block_delta event for text content")
	}
}

func TestAnthropicProtocolConverter_ToOpenAISSEResponse_TextDelta(t *testing.T) {
	conv := converter.AnthropicProtocolConverter{}

	deltaPayload := &dto.AnthropicSSEContentBlockDelta{
		Index: 0,
		Delta: dto.AnthropicSSEContentBlockDeltaPayload{
			Type: enum.AnthropicDeltaTypeTextDelta,
			Text: "Hello world",
		},
	}
	data := lo.Must1(sonic.Marshal(deltaPayload))

	event := dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeContentBlockDelta,
		Data:  data,
	}

	chunks, err := conv.ToOpenAISSEResponse(event, "claude-3-5-sonnet", "chatcmpl-test")
	if err != nil {
		t.Fatalf("ToOpenAISSEResponse() error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}

	chunk := chunks[0]
	if chunk.Model != "claude-3-5-sonnet" {
		t.Errorf("Model = %q, want %q", chunk.Model, "claude-3-5-sonnet")
	}
	if len(chunk.Choices) != 1 {
		t.Fatalf("len(Choices) = %d, want 1", len(chunk.Choices))
	}
	if chunk.Choices[0].Delta.Content != "Hello world" {
		t.Errorf("Delta.Content = %q, want %q", chunk.Choices[0].Delta.Content, "Hello world")
	}
}

func TestOpenAIProtocolConverter_FromAnthropicRequest_ToolNoParameters(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "tool_no_parameters")

	conv := converter.OpenAIProtocolConverter{}
	result, err := conv.FromAnthropicRequest(tc.AnthropicRequest)
	if err != nil {
		t.Fatalf("FromAnthropicRequest() error: %v", err)
	}

	// 检查工具定义
	if len(result.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(result.Tools))
	}
	if result.Tools[0].Function.Name != "CronList" {
		t.Errorf("Tools[0].Function.Name = %q, want %q", result.Tools[0].Function.Name, "CronList")
	}

	// 关键验证：对于无参数工具，parameters 应该为 nil
	// OpenAI 要求无参数工具直接省略 parameters 字段
	if result.Tools[0].Function.Parameters != nil {
		t.Errorf("Tools[0].Function.Parameters should be nil for empty object schema, got %v", result.Tools[0].Function.Parameters)
	}

	t.Logf("Empty tool parameters correctly set to nil")
}

func TestAnthropicProtocolConverter_ToOpenAISSEResponse_MessageDelta(t *testing.T) {
	conv := converter.AnthropicProtocolConverter{}

	deltaPayload := &dto.AnthropicSSEMessageDelta{
		Delta: dto.AnthropicSSEMessageDeltaPayload{
			StopReason: lo.ToPtr("end_turn"),
		},
		Usage: &dto.AnthropicUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}
	data := lo.Must1(sonic.Marshal(deltaPayload))

	event := dto.AnthropicSSEEvent{
		Event: enum.AnthropicSSEEventTypeMessageDelta,
		Data:  data,
	}

	chunks, err := conv.ToOpenAISSEResponse(event, "claude-3-5-sonnet", "chatcmpl-test")
	if err != nil {
		t.Fatalf("ToOpenAISSEResponse() error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}

	chunk := chunks[0]
	if chunk.Choices[0].FinishReason != enum.FinishReasonStop {
		t.Errorf("FinishReason = %q, want %q", chunk.Choices[0].FinishReason, enum.FinishReasonStop)
	}
	if chunk.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if chunk.Usage.PromptTokens != 100 {
		t.Errorf("Usage.PromptTokens = %d, want 100", chunk.Usage.PromptTokens)
	}
}

// ==================== Response API -> Anthropic Tests ====================

// responseTestCase Response API 测试用例结构
type responseTestCase struct {
	Name                string                                   `json:"name"`
	Description         string                                   `json:"description"`
	OpenAIResponseReq  *dto.OpenAICreateResponseReq            `json:"openai_response_request"`
	AnthropicRequest    *dto.AnthropicCreateMessageReq           `json:"anthropic_request"`
}

func loadResponseCases(t *testing.T) []responseTestCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/response_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/response_cases.json: %v", err)
	}
	var cases []responseTestCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/response_cases.json: %v", err)
	}
	return cases
}

func findResponseCase(t *testing.T, cases []responseTestCase, name string) responseTestCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("response test case %q not found in fixtures", name)
	return responseTestCase{}
}

func TestAnthropicProtocolConverter_FromResponseAPIRequest_SimpleText(t *testing.T) {
	allCases := loadResponseCases(t)
	tc := findResponseCase(t, allCases, "response_simple_text")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromResponseAPIRequest(tc.OpenAIResponseReq)
	if err != nil {
		t.Fatalf("FromResponseAPIRequest() error: %v", err)
	}

	// 检查 model
	if result.Model != tc.OpenAIResponseReq.Model {
		t.Errorf("Model = %q, want %q", result.Model, tc.OpenAIResponseReq.Model)
	}

	// 检查 messages: instructions 作为 system 消息 + user 消息
	if len(result.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(result.Messages))
	}

	// 第一条是 system（来自 instructions）
	if result.Messages[0].Role != enum.RoleSystem {
		t.Errorf("Messages[0].Role = %q, want %q", result.Messages[0].Role, enum.RoleSystem)
	}
	if result.Messages[0].Content == nil || result.Messages[0].Content.Text != "You are a helpful assistant." {
		t.Errorf("Messages[0].Content.Text = %q, want %q", result.Messages[0].Content.Text, "You are a helpful assistant.")
	}

	// 第二条是 user
	if result.Messages[1].Role != enum.RoleUser {
		t.Errorf("Messages[1].Role = %q, want %q", result.Messages[1].Role, enum.RoleUser)
	}

	// 检查 max_tokens
	if result.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d, want 1024", result.MaxTokens)
	}
}

func TestAnthropicProtocolConverter_FromResponseAPIRequest_ReasoningEffort(t *testing.T) {
	allCases := loadResponseCases(t)

	tests := []struct {
		caseName       string
		effort         string
		wantThinking   string
	}{
		{"low", "low", enum.AnthropicThinkingTypeLow},
		{"medium", "medium", enum.AnthropicThinkingTypeMedium},
		{"high", "high", enum.AnthropicThinkingTypeHigh},
		{"xhigh", "xhigh", enum.AnthropicThinkingTypeHigh},
		{"minimal", "minimal", enum.AnthropicThinkingTypeMinimal},
		{"none", "none", enum.AnthropicThinkingTypeDisabled},
	}

	for _, tt := range tests {
		t.Run(tt.effort, func(t *testing.T) {
			tc := findResponseCase(t, allCases, "response_reasoning_effort_"+tt.effort)

			conv := converter.AnthropicProtocolConverter{}
			result, err := conv.FromResponseAPIRequest(tc.OpenAIResponseReq)
			if err != nil {
				t.Fatalf("FromResponseAPIRequest() error: %v", err)
			}

			if result.Thinking == nil {
				t.Fatal("Thinking should not be nil")
			}
			if result.Thinking.Type != tt.wantThinking {
				t.Errorf("Thinking.Type = %q, want %q", result.Thinking.Type, tt.wantThinking)
			}
		})
	}
}

func TestAnthropicProtocolConverter_FromResponseAPIRequest_DeveloperRoleMappedToSystem(t *testing.T) {
	allCases := loadResponseCases(t)
	tc := findResponseCase(t, allCases, "response_developer_role_mapped_to_system")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromResponseAPIRequest(tc.OpenAIResponseReq)
	if err != nil {
		t.Fatalf("FromResponseAPIRequest() error: %v", err)
	}

	// developer 角色应该被映射为 system
	if len(result.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(result.Messages))
	}

	// 第一条是 developer -> system
	if result.Messages[0].Role != enum.RoleSystem {
		t.Errorf("Messages[0].Role = %q, want %q (developer mapped to system)", result.Messages[0].Role, enum.RoleSystem)
	}

	// 第二条是 user
	if result.Messages[1].Role != enum.RoleUser {
		t.Errorf("Messages[1].Role = %q, want %q", result.Messages[1].Role, enum.RoleUser)
	}
}

func TestAnthropicProtocolConverter_FromResponseAPIRequest_TextFormatJSONSchema(t *testing.T) {
	allCases := loadResponseCases(t)
	tc := findResponseCase(t, allCases, "response_text_format_json_schema")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromResponseAPIRequest(tc.OpenAIResponseReq)
	if err != nil {
		t.Fatalf("FromResponseAPIRequest() error: %v", err)
	}

	if result.OutputConfig == nil {
		t.Fatal("OutputConfig should not be nil")
	}
	if result.OutputConfig.Format == nil {
		t.Fatal("OutputConfig.Format should not be nil")
	}
	if result.OutputConfig.Format.Type != enum.ResponseFormatTypeJSONSchema {
		t.Errorf("OutputConfig.Format.Type = %q, want %q", result.OutputConfig.Format.Type, enum.ResponseFormatTypeJSONSchema)
	}
}

func TestAnthropicProtocolConverter_FromResponseAPIRequest_TextFormatJSONObject(t *testing.T) {
	allCases := loadResponseCases(t)
	tc := findResponseCase(t, allCases, "response_text_format_json_object")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromResponseAPIRequest(tc.OpenAIResponseReq)
	if err != nil {
		t.Fatalf("FromResponseAPIRequest() error: %v", err)
	}

	if result.OutputConfig == nil {
		t.Fatal("OutputConfig should not be nil")
	}
	// json_object 格式的 OutputConfig 存在但 Format 为 nil
	// 这是因为 convertResponseOutputFormat 只在有 schema 时才设置 Format
	if result.OutputConfig.Format != nil {
		t.Errorf("OutputConfig.Format = %v, want nil for json_object", result.OutputConfig.Format)
	}
}

func TestAnthropicProtocolConverter_FromResponseAPIRequest_FunctionCall(t *testing.T) {
	allCases := loadResponseCases(t)
	tc := findResponseCase(t, allCases, "response_function_call")

	conv := converter.AnthropicProtocolConverter{}
	result, err := conv.FromResponseAPIRequest(tc.OpenAIResponseReq)
	if err != nil {
		t.Fatalf("FromResponseAPIRequest() error: %v", err)
	}

	// 检查工具定义
	if len(result.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(result.Tools))
	}
	if result.Tools[0].Name != "get_weather" {
		t.Errorf("Tools[0].Name = %q, want %q", result.Tools[0].Name, "get_weather")
	}

	// 检查 tool_choice
	if result.ToolChoice == nil {
		t.Fatal("ToolChoice should not be nil")
	}
	if result.ToolChoice.Type != "auto" {
		t.Errorf("ToolChoice.Type = %q, want %q", result.ToolChoice.Type, "auto")
	}
}

func TestAnthropicProtocolConverter_FromResponseAPIRequest_NoReasoning(t *testing.T) {
	// 测试没有 reasoning 配置时 Thinking 为 nil
	conv := converter.AnthropicProtocolConverter{}
	req := &dto.OpenAICreateResponseReq{
		Model: "gpt-4o",
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{
				{
					Type:   enum.ResponseInputItemTypeMessage,
					Role:   enum.RoleUser,
					Content: &dto.ResponseInputMessageContent{
						Parts: []*dto.ResponseInputContent{
							{Type: enum.ResponseContentTypeInputText, Text: "Hello"},
						},
					},
				},
			},
		},
	}

	result, err := conv.FromResponseAPIRequest(req)
	if err != nil {
		t.Fatalf("FromResponseAPIRequest() error: %v", err)
	}

	if result.Thinking != nil {
		t.Errorf("Thinking = %v, want nil when no reasoning config", result.Thinking)
	}
}

func TestAnthropicProtocolConverter_FromResponseAPIRequest_EmptyInput(t *testing.T) {
	// 测试空 input 时不崩溃
	conv := converter.AnthropicProtocolConverter{}
	req := &dto.OpenAICreateResponseReq{
		Model: "gpt-4o",
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{},
		},
	}

	result, err := conv.FromResponseAPIRequest(req)
	if err != nil {
		t.Fatalf("FromResponseAPIRequest() error: %v", err)
	}

	if len(result.Messages) != 0 {
		t.Errorf("len(Messages) = %d, want 0", len(result.Messages))
	}
}
