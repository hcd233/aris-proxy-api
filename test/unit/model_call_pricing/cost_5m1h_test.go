package model_call_pricing

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestCostBreakdown5m1h(t *testing.T) {
	t.Parallel()
	rule := vo.PricingRule{
		InputMicro:         1_000_000, // 1.00/1M
		OutputMicro:        2_000_000,
		CacheCreateMicro:   1_250_000, // 5m 价
		CacheCreate1hMicro: 2_500_000,
		CacheReadMicro:     100_000,
	}
	b := rule.CostBreakdown(100, 50, 40, 20, 40)
	// 40×1.25 + 20×2.5 = 50 + 50 = 100（整除，无舍入歧义）
	if b.CacheCreateMicro != 100 {
		t.Fatalf("CacheCreateMicro = %d, want 100", b.CacheCreateMicro)
	}
	if b.Total() != b.InputMicro+b.OutputMicro+b.CacheCreateMicro+b.CacheReadMicro {
		t.Fatalf("Total inconsistent")
	}
}

func TestCostBreakdown1hFallback(t *testing.T) {
	t.Parallel()
	rule := vo.PricingRule{CacheCreateMicro: 1_250_000} // 1h 价未配置
	b := rule.CostBreakdown(0, 0, 8, 8, 0)
	// 1h 回落 5m 价：8×1.25 + 8×1.25 = 20（整除）
	if b.CacheCreateMicro != 20 {
		t.Fatalf("CacheCreateMicro = %d, want 20", b.CacheCreateMicro)
	}
}

func TestPriceModelCall5m1h(t *testing.T) {
	t.Parallel()
	pricing, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{
		InputMicro:         1_000_000,
		OutputMicro:        2_000_000,
		CacheCreateMicro:   1_250_000,
		CacheCreate1hMicro: 2_500_000,
		CacheReadMicro:     100_000,
	}})
	if err != nil {
		t.Fatalf("NewPricing: %v", err)
	}
	task := &dto.ModelCallAuditTask{
		InputTokens:                100,
		OutputTokens:               50,
		CacheCreationInputTokens:   30,
		CacheCreation1hInputTokens: 10,
		CacheReadInputTokens:       0,
		UpstreamStatusCode:         200,
		CreatedAt:                  time.Now(),
	}
	usecase.PriceModelCall(task, pricing)
	if task.CacheCreateCostMicro == nil {
		t.Fatal("CacheCreateCostMicro is nil")
	}
	// 20×1.25 + 10×2.5 = 25 + 25 = 50
	if *task.CacheCreateCostMicro != 50 {
		t.Fatalf("CacheCreateCostMicro = %d, want 50", *task.CacheCreateCostMicro)
	}
}

// TestPriceModelCallClamps1hToTotal 上游数据不一致（1h > 总量）时 5m 档不得为负抵扣费用。
func TestPriceModelCallClamps1hToTotal(t *testing.T) {
	t.Parallel()
	pricing, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{
		CacheCreateMicro:   1_250_000,
		CacheCreate1hMicro: 2_500_000,
	}})
	if err != nil {
		t.Fatalf("NewPricing: %v", err)
	}
	task := &dto.ModelCallAuditTask{
		CacheCreationInputTokens:   10,
		CacheCreation1hInputTokens: 30,
		UpstreamStatusCode:         200,
		CreatedAt:                  time.Now(),
	}
	usecase.PriceModelCall(task, pricing)
	// 1h 钳制为 10：10×2.5 = 25；未钳制时为 (-20)×1.25 + 30×2.5 = 50，且 5m 档为负
	if task.CacheCreateCostMicro == nil || *task.CacheCreateCostMicro != 25 {
		t.Fatalf("CacheCreateCostMicro = %v, want 25", task.CacheCreateCostMicro)
	}
}
