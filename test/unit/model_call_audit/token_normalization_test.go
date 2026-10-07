package model_call_audit

import (
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// TestSetTokensFromOpenAIUsage_NormalizesCachedInput 覆盖 OpenAI Chat 语义：
// prompt_tokens 含 cached_tokens，归一化后 InputTokens 为净输入、缓存读独立成维。
func TestSetTokensFromOpenAIUsage_NormalizesCachedInput(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{}
	task.SetTokensFromOpenAIUsage(&dto.OpenAICompletionUsage{
		PromptTokens:        1000,
		CompletionTokens:    50,
		PromptTokensDetails: &dto.OpenAIPromptTokensDetails{CachedTokens: lo.ToPtr(800)},
	})
	if task.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200 (prompt_tokens - cached)", task.InputTokens)
	}
	if task.CacheReadInputTokens != 800 {
		t.Errorf("CacheReadInputTokens = %d, want 800", task.CacheReadInputTokens)
	}
	if task.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", task.OutputTokens)
	}
}

// TestSetTokensFromOpenAIUsage_DeepSeekStyleCacheHit OpenAI 兼容上游的 DeepSeek 风格
// 字段（prompt_cache_hit_tokens，与 prompt_tokens 为包含关系）同样落净输入。
func TestSetTokensFromOpenAIUsage_DeepSeekStyleCacheHit(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{}
	task.SetTokensFromOpenAIUsage(&dto.OpenAICompletionUsage{
		PromptTokens:          1000,
		PromptCacheHitTokens:  lo.ToPtr(600),
		PromptCacheMissTokens: lo.ToPtr(400),
	})
	if task.InputTokens != 400 || task.CacheReadInputTokens != 600 {
		t.Errorf("tokens = (%d, %d), want (400, 600)", task.InputTokens, task.CacheReadInputTokens)
	}
}

// TestSetTokensFromOpenAIUsage_ClampsDirtyUsage cached 大于 prompt_tokens 的脏数据
// 不得产生负输入。
func TestSetTokensFromOpenAIUsage_ClampsDirtyUsage(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{}
	task.SetTokensFromOpenAIUsage(&dto.OpenAICompletionUsage{
		PromptTokens:        10,
		PromptTokensDetails: &dto.OpenAIPromptTokensDetails{CachedTokens: lo.ToPtr(30)},
	})
	if task.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", task.InputTokens)
	}
}

// TestSetTokensFromResponseUsage_NormalizesCachedInput Response API 的 input_tokens
// 同 OpenAI Chat，含 cached_tokens。
func TestSetTokensFromResponseUsage_NormalizesCachedInput(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{}
	task.SetTokensFromResponseUsage(&dto.OpenAICreateResponseRsp{
		Usage: &dto.ResponseUsage{
			InputTokens:        5,
			OutputTokens:       3,
			InputTokensDetails: &dto.ResponseInputTokensDetail{CachedTokens: 1},
		},
	})
	if task.InputTokens != 4 || task.CacheReadInputTokens != 1 {
		t.Errorf("tokens = (%d, %d), want (4, 1)", task.InputTokens, task.CacheReadInputTokens)
	}
	if task.OutputTokens != 3 {
		t.Errorf("OutputTokens = %d, want 3", task.OutputTokens)
	}
}

// TestSetTokensFromAnthropicUsage_OfficialSemantics Anthropic 官方 usage 的
// input_tokens 不含缓存两维，四维天然互斥，不得扣减。
func TestSetTokensFromAnthropicUsage_OfficialSemantics(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{}
	task.SetTokensFromAnthropicUsage(&dto.AnthropicMessage{
		Usage: &dto.AnthropicUsage{
			InputTokens:              100,
			OutputTokens:             20,
			CacheCreationInputTokens: lo.ToPtr(30),
			CacheReadInputTokens:     lo.ToPtr(50),
		},
	})
	if task.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100 (Anthropic input_tokens is already net)", task.InputTokens)
	}
	if task.CacheCreationInputTokens != 30 || task.CacheReadInputTokens != 50 {
		t.Errorf("cache tokens = (%d, %d), want (30, 50)", task.CacheCreationInputTokens, task.CacheReadInputTokens)
	}
}

// TestSetTokensFromAnthropicUsage_DeepSeekStyle Anthropic 协议下若上游返回 DeepSeek
// 风格成对字段（hit + miss），input_tokens 含命中量，需落净输入。
func TestSetTokensFromAnthropicUsage_DeepSeekStyle(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{}
	task.SetTokensFromAnthropicUsage(&dto.AnthropicMessage{
		Usage: &dto.AnthropicUsage{
			InputTokens:           1000,
			PromptCacheHitTokens:  lo.ToPtr(700),
			PromptCacheMissTokens: lo.ToPtr(300),
		},
	})
	if task.InputTokens != 300 || task.CacheReadInputTokens != 700 {
		t.Errorf("tokens = (%d, %d), want (300, 700)", task.InputTokens, task.CacheReadInputTokens)
	}
}

// TestSetTokensFromAnthropicUsage_HitWithoutMiss 只有 hit 字段（无 miss）时按
// Anthropic 官方语义处理，不做扣减。
func TestSetTokensFromAnthropicUsage_HitWithoutMiss(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{}
	task.SetTokensFromAnthropicUsage(&dto.AnthropicMessage{
		Usage: &dto.AnthropicUsage{
			InputTokens:          100,
			PromptCacheHitTokens: lo.ToPtr(40),
		},
	})
	if task.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100 (no miss field means Anthropic semantics)", task.InputTokens)
	}
	if task.CacheReadInputTokens != 40 {
		t.Errorf("CacheReadInputTokens = %d, want 40", task.CacheReadInputTokens)
	}
}
