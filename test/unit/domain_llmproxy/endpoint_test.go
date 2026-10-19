package domain_llmproxy

import (
	"errors"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

func TestCreateEndpoint_AllowsSingleProtocolEndpoint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                        string
		openaiBaseURL               string
		anthropicBaseURL            string
		supportOpenAIChatCompletion bool
		supportOpenAIResponse       bool
		supportAnthropicMessage     bool
		supportOpenAIDecision       bool
	}{
		{
			name:                        "openai_only_chat",
			openaiBaseURL:               "https://api.openai.com",
			supportOpenAIChatCompletion: true,
		},
		{
			name:                    "anthropic_only_message",
			anthropicBaseURL:        "https://api.anthropic.com",
			supportAnthropicMessage: true,
		},
		{
			name:                  "openai_only_decision",
			openaiBaseURL:         "https://api.openai.com",
			supportOpenAIDecision: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ep, err := aggregate.CreateEndpoint(
				1,
				tc.name,
				tc.openaiBaseURL,
				tc.anthropicBaseURL,
				"sk-test",
				tc.supportOpenAIChatCompletion,
				tc.supportOpenAIResponse,
				tc.supportAnthropicMessage,
				tc.supportOpenAIDecision,
				enum.EndpointAuthAPIKey,
			)
			if err != nil {
				t.Fatalf("CreateEndpoint() error: %v", err)
			}
			if ep == nil {
				t.Fatal("CreateEndpoint() returned nil endpoint")
			}
		})
	}
}

func TestCreateEndpoint_RejectsMissingSupportedProtocolBaseURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                        string
		openaiBaseURL               string
		anthropicBaseURL            string
		supportOpenAIChatCompletion bool
		supportOpenAIResponse       bool
		supportAnthropicMessage     bool
		supportOpenAIDecision       bool
	}{
		{
			name:                        "missing_openai_chat_base_url",
			supportOpenAIChatCompletion: true,
		},
		{
			name:                  "missing_openai_response_base_url",
			supportOpenAIResponse: true,
		},
		{
			name:                    "missing_anthropic_message_base_url",
			supportAnthropicMessage: true,
		},
		{
			name:                  "missing_openai_decision_base_url",
			supportOpenAIDecision: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := aggregate.CreateEndpoint(
				1,
				tc.name,
				tc.openaiBaseURL,
				tc.anthropicBaseURL,
				"sk-test",
				tc.supportOpenAIChatCompletion,
				tc.supportOpenAIResponse,
				tc.supportAnthropicMessage,
				tc.supportOpenAIDecision,
				enum.EndpointAuthAPIKey,
			)
			if !errors.Is(err, ierr.ErrValidation) {
				t.Fatalf("CreateEndpoint() error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestCreateEndpoint_RejectsBothEmptyBaseURL(t *testing.T) {
	t.Parallel()
	_, err := aggregate.CreateEndpoint(
		1, "test", "", "", "sk-test",
		false, false, false, false,
		enum.EndpointAuthAPIKey,
	)
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("CreateEndpoint() error = %v, want ErrValidation", err)
	}
}

func TestCreateEndpoint_RejectsNoCapability(t *testing.T) {
	t.Parallel()
	_, err := aggregate.CreateEndpoint(
		1, "test", "https://api.openai.com", "https://api.anthropic.com", "sk-test",
		false, false, false, false,
		enum.EndpointAuthAPIKey,
	)
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("CreateEndpoint() error = %v, want ErrValidation", err)
	}
}

// TestUpdate_RejectsCapabilityWithoutBaseURL 回归 2026-10-12 tokenhub 事故
// （traceId f2646f81-6b07-4f29-967c-32e391661d1b / 764ef7d1-9956-4f30-aedb-1ae75bac7e28）：
// anthropic baseURL 为空时打开 Anthropic 能力，更新必须被拒绝且不落到聚合根上。
// 放行即写库，而仓库写全部字段，之后该端点每次读取都失败（列表、代理、修复入口全挂）。
func TestUpdate_RejectsCapabilityWithoutBaseURL(t *testing.T) {
	t.Parallel()
	ep := aggregate.RehydrateEndpoint(6, "tokenhub", "https://tokenhub.example.com/v1", "", "sk-test", true, true, false, false, enum.EndpointAuthAPIKey)

	err := ep.Update(lo.ToPtr("tokenhub"), nil, nil, nil, nil, nil, lo.ToPtr(true), nil)
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("Update() error = %v, want ErrValidation", err)
	}
	if ep.SupportAnthropicMessage() {
		t.Fatal("被拒绝的更新不得写回聚合根")
	}
	if ep.OpenaiBaseURL() != "https://tokenhub.example.com/v1" || ep.Name() != "tokenhub" {
		t.Fatalf("被拒绝的更新不得改动其他字段: name=%q openai=%q", ep.Name(), ep.OpenaiBaseURL())
	}

	if err := ep.Update(nil, nil, lo.ToPtr("https://tokenhub.example.com/v1"), nil, nil, nil, lo.ToPtr(true), nil); err != nil {
		t.Fatalf("补齐 anthropic baseURL 后应允许开启能力: %v", err)
	}
	if !ep.SupportAnthropicMessage() || ep.AnthropicBaseURL() != "https://tokenhub.example.com/v1" {
		t.Fatal("补齐 baseURL 的更新应生效")
	}
}

// TestUpdate_RejectsClearingBaseURLWhileCapabilityOn 能力开着时清空 baseURL 同样拒绝。
func TestUpdate_RejectsClearingBaseURLWhileCapabilityOn(t *testing.T) {
	t.Parallel()
	ep := aggregate.RehydrateEndpoint(1, "ep", "https://o.example.com", "https://a.example.com", "sk-test", true, false, true, false, enum.EndpointAuthAPIKey)

	if err := ep.Update(nil, nil, lo.ToPtr(""), nil, nil, nil, nil, nil); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("Update() error = %v, want ErrValidation", err)
	}
	if ep.AnthropicBaseURL() != "https://a.example.com" {
		t.Fatalf("被拒绝的更新不得清空 baseURL, got %q", ep.AnthropicBaseURL())
	}
}

// TestRehydrateEndpoint_ToleratesInconsistentRow 读路径恢复不做校验：
// 库侧直改出的坏行必须仍读得出，否则连接口都进不去，只能手改数据库。
func TestRehydrateEndpoint_ToleratesInconsistentRow(t *testing.T) {
	t.Parallel()
	ep := aggregate.RehydrateEndpoint(6, "tokenhub", "https://tokenhub.example.com/v1", "", "sk-test", true, true, true, false, enum.EndpointAuthAPIKey)

	if ep.AggregateID() != 6 || ep.Name() != "tokenhub" || ep.APIKey() != "sk-test" {
		t.Fatalf("恢复应忠实拷贝持久化状态: id=%d name=%q", ep.AggregateID(), ep.Name())
	}
	if ep.AnthropicBaseURL() != "" || !ep.SupportAnthropicMessage() {
		t.Fatalf("不一致字段应原样恢复: anthropic=%q message=%v", ep.AnthropicBaseURL(), ep.SupportAnthropicMessage())
	}
}
