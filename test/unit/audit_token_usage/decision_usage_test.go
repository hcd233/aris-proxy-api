package audit_token_usage

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestSetTokensFromDecisionUsage_NormalizesNetInput(t *testing.T) {
	t.Parallel()

	rsp := &dto.OpenAIDecisionRsp{
		Usage: &dto.OpenAIDecisionUsage{
			InputTokens:  42,
			OutputTokens: 7,
			TotalTokens:  49,
			InputTokensDetails: &dto.DecisionInputTokensDetails{
				CachedTokens:     12,
				CacheWriteTokens: 5,
			},
		},
	}

	var task dto.ModelCallAuditTask
	task.SetTokensFromDecisionUsage(rsp)

	if task.InputTokens != 25 {
		t.Fatalf("InputTokens = %d, want 25 (42-12-5)", task.InputTokens)
	}
	if task.OutputTokens != 7 {
		t.Fatalf("OutputTokens = %d, want 7", task.OutputTokens)
	}
	if task.CacheReadInputTokens != 12 {
		t.Fatalf("CacheReadInputTokens = %d, want 12", task.CacheReadInputTokens)
	}
	if task.CacheCreationInputTokens != 5 {
		t.Fatalf("CacheCreationInputTokens = %d, want 5", task.CacheCreationInputTokens)
	}
	sum := task.InputTokens + task.CacheReadInputTokens + task.CacheCreationInputTokens
	if sum != rsp.Usage.InputTokens {
		t.Fatalf("four-dim invariant broken: %d != %d", sum, rsp.Usage.InputTokens)
	}
}

func TestSetTokensFromDecisionUsage_ClampsAtZero(t *testing.T) {
	t.Parallel()

	rsp := &dto.OpenAIDecisionRsp{
		Usage: &dto.OpenAIDecisionUsage{
			InputTokens: 3,
			InputTokensDetails: &dto.DecisionInputTokensDetails{
				CachedTokens:     10,
				CacheWriteTokens: 4,
			},
		},
	}

	var task dto.ModelCallAuditTask
	task.SetTokensFromDecisionUsage(rsp)

	if task.InputTokens != 0 {
		t.Fatalf("InputTokens = %d, want 0", task.InputTokens)
	}
}

func TestSetTokensFromDecisionUsage_NilSafety(t *testing.T) {
	t.Parallel()

	var task dto.ModelCallAuditTask
	task.SetTokensFromDecisionUsage(nil)
	task.SetTokensFromDecisionUsage(&dto.OpenAIDecisionRsp{})

	if task.InputTokens != 0 || task.OutputTokens != 0 {
		t.Fatalf("task = %+v, want zero", task)
	}
}
