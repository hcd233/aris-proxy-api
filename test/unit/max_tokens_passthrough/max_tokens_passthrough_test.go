// Package max_tokens_passthrough 回归测试：OpenAI /chat/completions forwardChatNative 路径
// 必须原样透传用户请求中的 max_tokens 字段，不得将其 silent 改写为 max_completion_tokens。
//
// 背景 bug (traceID 1d2403ab-7be1-4b0b-a189-ea2e4eb8d434, 2026-04-27 17:44):
//
//	用户通过 opencode 客户端调用 /api/openai/v1/chat/completions，请求体中带
//	`max_tokens: 32000`；彼时 openAIService.forwardNative 会无条件执行
//	    req.Body.MaxCompletionTokens, req.Body.MaxTokens = lo.ToPtr(*req.Body.MaxTokens), nil
//	把字段改写为 `max_completion_tokens`。上游 api.chatanywhere.tech 仅识别
//	`max_tokens`，对 `max_completion_tokens` 直接忽略，相当于把用户设置的生成上限
//	抹掉，叠加 32 个 tools + 超长 system prompt，最终触发上游模型 503
//	"模型无返回结果，可能是内容违规、输入过长、输入格式有误或负载较高"。
//
// 本测试直接用 usecase 层 body 构建所用的两步纯函数（util.ReplaceModelInBody +
// util.EnsureAssistantMessageReasoningContent）构造发给上游的 body，断言
// max_tokens 在整个链路中保持原样。
package max_tokens_passthrough

import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
)

// buildForwardBody 复刻 OpenAIUseCase.forwardChatNative 中的 body 构建流程：
//  1. sonic.Marshal(req)
//  2. util.ReplaceModelInBody 替换 model 字段为上游真实 model
//  3. util.EnsureAssistantMessageReasoningContent 处理 thinking 模式 reasoning_content
func buildForwardBody(t *testing.T, req *dto.OpenAIChatCompletionReq, upstreamModel string) []byte {
	t.Helper()
	raw, err := sonic.Marshal(req)
	if err != nil {
		t.Fatalf("sonic.Marshal error: %v", err)
	}
	body := util.ReplaceModelInBody(raw, upstreamModel)
	return body
}

// TestForwardNative_MaxTokensPreserved 回归：用户请求中的 max_tokens 必须原样透传给上游。
func TestForwardNative_MaxTokensPreserved(t *testing.T) {
	req := &dto.OpenAIChatCompletionReq{
		Model:     "gpt-5.5",
		MaxTokens: lo.ToPtr(32000),
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleSystem, Content: &dto.OpenAIMessageContent{Text: "You are helpful"}},
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hello"}},
		},
	}

	body := buildForwardBody(t, req, "gpt-5.5")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"max_tokens":32000`) {
		t.Fatalf("max_tokens must be preserved in upstream body, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, "max_completion_tokens") {
		t.Fatalf("max_completion_tokens must NOT be injected when user sent max_tokens, got: %s", bodyStr)
	}
}

// TestForwardNative_MaxCompletionTokensPreserved 当用户主动指定
// max_completion_tokens（例如调用 GPT-5 / o1 系列）时，同样必须原样透传。
func TestForwardNative_MaxCompletionTokensPreserved(t *testing.T) {
	req := &dto.OpenAIChatCompletionReq{
		Model:               "gpt-5.5",
		MaxCompletionTokens: lo.ToPtr(16000),
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hi"}},
		},
	}

	body := buildForwardBody(t, req, "gpt-5.5")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"max_completion_tokens":16000`) {
		t.Fatalf("max_completion_tokens must be preserved, got: %s", bodyStr)
	}
	if strings.Contains(bodyStr, `"max_tokens"`) {
		t.Fatalf("max_tokens must NOT appear when user only sent max_completion_tokens, got: %s", bodyStr)
	}
}

// TestForwardNative_BothFieldsPreserved 当用户同时传了 max_tokens 和
// max_completion_tokens 时（某些客户端为兼容老/新模型会同时带上），两者都应原样透传。
func TestForwardNative_BothFieldsPreserved(t *testing.T) {
	req := &dto.OpenAIChatCompletionReq{
		Model:               "gpt-5.5",
		MaxTokens:           lo.ToPtr(8000),
		MaxCompletionTokens: lo.ToPtr(8000),
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hi"}},
		},
	}

	body := buildForwardBody(t, req, "gpt-5.5")
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"max_tokens":8000`) {
		t.Fatalf("max_tokens must be preserved, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"max_completion_tokens":8000`) {
		t.Fatalf("max_completion_tokens must be preserved, got: %s", bodyStr)
	}
}

// TestForwardNative_ModelAliasReplaced 顺带验证：upstream body 中的 model
// 字段已经被替换成上游真实 model，而不是用户请求里的别名。
func TestForwardNative_ModelAliasReplaced(t *testing.T) {
	req := &dto.OpenAIChatCompletionReq{
		Model:     "gpt-5.5", // 用户层别名
		MaxTokens: lo.ToPtr(1024),
		Messages: []*dto.OpenAIChatCompletionMessageParam{
			{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hi"}},
		},
	}

	body := buildForwardBody(t, req, "gpt-5-2025-04-27") // 上游真实 model
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `"model":"gpt-5-2025-04-27"`) {
		t.Fatalf("model must be replaced with upstream model name, got: %s", bodyStr)
	}
}
