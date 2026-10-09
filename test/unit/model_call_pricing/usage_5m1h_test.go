package model_call_pricing

import (
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/dto/anthropic"
)

func TestSetTokensFromAnthropicUsage5m1h(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		usageJSON string
		wantTotal int
		want1h    int
	}{
		{
			name:      "新版 cache_creation 对象 5m+1h",
			usageJSON: `{"input_tokens":100,"output_tokens":10,"cache_creation_input_tokens":60,"cache_creation":{"ephemeral_5m_input_tokens":40,"ephemeral_1h_input_tokens":20}}`,
			wantTotal: 60,
			want1h:    20,
		},
		{
			name:      "旧版仅总量字段全部归 5m",
			usageJSON: `{"input_tokens":100,"output_tokens":10,"cache_creation_input_tokens":50}`,
			wantTotal: 50,
			want1h:    0,
		},
		{
			name:      "cache_creation 对象优先于总量字段",
			usageJSON: `{"input_tokens":100,"output_tokens":10,"cache_creation_input_tokens":999,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":30}}`,
			wantTotal: 60,
			want1h:    30,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var usage anthropic.AnthropicUsage
			if err := sonic.UnmarshalString(tt.usageJSON, &usage); err != nil {
				t.Fatalf("unmarshal usage: %v", err)
			}
			task := &dto.ModelCallAuditTask{}
			task.SetTokensFromAnthropicUsage(&anthropic.AnthropicMessage{Usage: &usage})
			if task.CacheCreationInputTokens != tt.wantTotal {
				t.Fatalf("CacheCreationInputTokens = %d, want %d", task.CacheCreationInputTokens, tt.wantTotal)
			}
			if task.CacheCreation1hInputTokens != tt.want1h {
				t.Fatalf("CacheCreation1hInputTokens = %d, want %d", task.CacheCreation1hInputTokens, tt.want1h)
			}
		})
	}
}
