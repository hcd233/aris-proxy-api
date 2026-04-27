package model_call_audit

import (
	"context"
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// testCase mirrors the fixture structure
type testCase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Task        struct {
		ModelID                  uint   `json:"model_id"`
		Model                    string `json:"model"`
		UpstreamProvider         string `json:"upstream_provider"`
		APIProvider              string `json:"api_provider"`
		InputTokens              int    `json:"input_tokens"`
		OutputTokens             int    `json:"output_tokens"`
		CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
		FirstTokenLatencyMs      int64  `json:"first_token_latency_ms"`
		StreamDurationMs         int64  `json:"stream_duration_ms"`
		UpstreamStatusCode       int    `json:"upstream_status_code"`
		ErrorMessage             string `json:"error_message"`
	} `json:"task"`
}

func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	return cases
}

func TestModelCallAuditTask_Fields(t *testing.T) {
	cases := loadCases(t)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			task := &dto.ModelCallAuditTask{
				Ctx:                      context.Background(),
				ModelID:                  tc.Task.ModelID,
				Model:                    tc.Task.Model,
				UpstreamProvider:         tc.Task.UpstreamProvider,
				APIProvider:              tc.Task.APIProvider,
				InputTokens:              tc.Task.InputTokens,
				OutputTokens:             tc.Task.OutputTokens,
				CacheCreationInputTokens: tc.Task.CacheCreationInputTokens,
				CacheReadInputTokens:     tc.Task.CacheReadInputTokens,
				FirstTokenLatencyMs:      tc.Task.FirstTokenLatencyMs,
				StreamDurationMs:         tc.Task.StreamDurationMs,
				UpstreamStatusCode:       tc.Task.UpstreamStatusCode,
				ErrorMessage:             tc.Task.ErrorMessage,
			}

			if task.Model != tc.Task.Model {
				t.Errorf("Model = %q, want %q", task.Model, tc.Task.Model)
			}
			if task.UpstreamProvider != tc.Task.UpstreamProvider {
				t.Errorf("UpstreamProvider = %q, want %q", task.UpstreamProvider, tc.Task.UpstreamProvider)
			}
			if task.APIProvider != tc.Task.APIProvider {
				t.Errorf("APIProvider = %q, want %q", task.APIProvider, tc.Task.APIProvider)
			}
			if task.InputTokens != tc.Task.InputTokens {
				t.Errorf("InputTokens = %d, want %d", task.InputTokens, tc.Task.InputTokens)
			}
			if task.OutputTokens != tc.Task.OutputTokens {
				t.Errorf("OutputTokens = %d, want %d", task.OutputTokens, tc.Task.OutputTokens)
			}
			if task.CacheCreationInputTokens != tc.Task.CacheCreationInputTokens {
				t.Errorf("CacheCreationInputTokens = %d, want %d", task.CacheCreationInputTokens, tc.Task.CacheCreationInputTokens)
			}
			if task.CacheReadInputTokens != tc.Task.CacheReadInputTokens {
				t.Errorf("CacheReadInputTokens = %d, want %d", task.CacheReadInputTokens, tc.Task.CacheReadInputTokens)
			}
			if task.FirstTokenLatencyMs != tc.Task.FirstTokenLatencyMs {
				t.Errorf("FirstTokenLatencyMs = %d, want %d", task.FirstTokenLatencyMs, tc.Task.FirstTokenLatencyMs)
			}
			if task.StreamDurationMs != tc.Task.StreamDurationMs {
				t.Errorf("StreamDurationMs = %d, want %d", task.StreamDurationMs, tc.Task.StreamDurationMs)
			}
			if task.UpstreamStatusCode != tc.Task.UpstreamStatusCode {
				t.Errorf("UpstreamStatusCode = %d, want %d", task.UpstreamStatusCode, tc.Task.UpstreamStatusCode)
			}
			if task.ErrorMessage != tc.Task.ErrorMessage {
				t.Errorf("ErrorMessage = %q, want %q", task.ErrorMessage, tc.Task.ErrorMessage)
			}
		})
	}
}
