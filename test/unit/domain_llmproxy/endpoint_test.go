package domain_llmproxy

import (
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

func TestCreateEndpoint_AllowsSingleProtocolEndpoint(t *testing.T) {
	cases := []struct {
		name                        string
		openaiBaseURL               string
		anthropicBaseURL            string
		supportOpenAIChatCompletion bool
		supportOpenAIResponse       bool
		supportAnthropicMessage     bool
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ep, err := aggregate.CreateEndpoint(
				1,
				tc.name,
				tc.openaiBaseURL,
				tc.anthropicBaseURL,
				"sk-test",
				tc.supportOpenAIChatCompletion,
				tc.supportOpenAIResponse,
				tc.supportAnthropicMessage,
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
	cases := []struct {
		name                        string
		openaiBaseURL               string
		anthropicBaseURL            string
		supportOpenAIChatCompletion bool
		supportOpenAIResponse       bool
		supportAnthropicMessage     bool
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := aggregate.CreateEndpoint(
				1,
				tc.name,
				tc.openaiBaseURL,
				tc.anthropicBaseURL,
				"sk-test",
				tc.supportOpenAIChatCompletion,
				tc.supportOpenAIResponse,
				tc.supportAnthropicMessage,
			)
			if !errors.Is(err, ierr.ErrValidation) {
				t.Fatalf("CreateEndpoint() error = %v, want ErrValidation", err)
			}
		})
	}
}
