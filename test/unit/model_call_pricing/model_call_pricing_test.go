// Package model_call_pricing 审计计费粘连的单元测试（PriceModelCall）
package model_call_pricing

import (
	"net/http"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func pricedPricing(t *testing.T) vo.Pricing {
	t.Helper()
	p, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{
		{ContextMax: 200_000, InputMicro: 1_000_000, OutputMicro: 2_000_000},
		{InputMicro: 2_000_000, OutputMicro: 4_000_000},
	})
	if err != nil {
		t.Fatalf("NewPricing() error = %v", err)
	}
	return p
}

func TestPriceModelCall_SuccessPriced(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{
		UpstreamStatusCode:       http.StatusOK,
		InputTokens:              1000,
		OutputTokens:             500,
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     0,
	}
	usecase.PriceModelCall(task, pricedPricing(t))
	if task.CostMicro == nil {
		t.Fatalf("CostMicro should not be nil for priced success call")
	}
	// 区间命中 ≤200k 档：1000×1/1e6 + 500×2/1e6 = 1000+1000 = 2000 微单位
	if *task.CostMicro != 2000 {
		t.Fatalf("CostMicro = %d, want 2000", *task.CostMicro)
	}
	if task.PricingCurrency != string(enum.CurrencyUSD) {
		t.Fatalf("PricingCurrency = %q", task.PricingCurrency)
	}
	// 四维拆分：输入 1000×1e6/1e6=1000，输出 500×2e6/1e6=1000，缓存两维 0
	if task.InputCostMicro == nil || *task.InputCostMicro != 1000 {
		t.Fatalf("InputCostMicro = %v, want 1000", task.InputCostMicro)
	}
	if task.OutputCostMicro == nil || *task.OutputCostMicro != 1000 {
		t.Fatalf("OutputCostMicro = %v, want 1000", task.OutputCostMicro)
	}
	if task.CacheCreateCostMicro == nil || *task.CacheCreateCostMicro != 0 {
		t.Fatalf("CacheCreateCostMicro = %v, want 0", task.CacheCreateCostMicro)
	}
	if task.CacheReadCostMicro == nil || *task.CacheReadCostMicro != 0 {
		t.Fatalf("CacheReadCostMicro = %v, want 0", task.CacheReadCostMicro)
	}
	if *task.CostMicro != *task.InputCostMicro+*task.OutputCostMicro+*task.CacheCreateCostMicro+*task.CacheReadCostMicro {
		t.Fatalf("CostMicro must equal breakdown sum")
	}
}

func TestPriceModelCall_ContextTierJump(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{
		UpstreamStatusCode: http.StatusOK,
		InputTokens:        300_000,
	}
	usecase.PriceModelCall(task, pricedPricing(t))
	// 整段跳档：300k > 200k，全部按第二档（2_000_000 微单位/1M）→ 300_000×2_000_000/1e6 = 600_000 微单位
	if task.CostMicro == nil || *task.CostMicro != 600_000 {
		t.Fatalf("CostMicro = %v, want 600000", task.CostMicro)
	}
}

func TestPriceModelCall_FailureOrUnpriced(t *testing.T) {
	t.Parallel()
	// 失败调用不计费
	task := &dto.ModelCallAuditTask{UpstreamStatusCode: http.StatusBadGateway, InputTokens: 1000}
	usecase.PriceModelCall(task, pricedPricing(t))
	if task.CostMicro != nil {
		t.Fatalf("failed call must not be priced, got %v", *task.CostMicro)
	}
	// 未计价模型不计费
	task2 := &dto.ModelCallAuditTask{UpstreamStatusCode: http.StatusOK, InputTokens: 1000}
	usecase.PriceModelCall(task2, vo.Pricing{})
	if task2.CostMicro != nil {
		t.Fatalf("unpriced model must not be priced")
	}
	// 免费模型：计 0 并带币种快照
	free, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{}})
	if err != nil {
		t.Fatalf("NewPricing(free) error = %v", err)
	}
	task3 := &dto.ModelCallAuditTask{UpstreamStatusCode: http.StatusOK, InputTokens: 1000}
	usecase.PriceModelCall(task3, free)
	if task3.CostMicro == nil || *task3.CostMicro != 0 {
		t.Fatalf("free model cost = %v, want 0", task3.CostMicro)
	}
	if task3.PricingCurrency != string(enum.CurrencyUSD) {
		t.Fatalf("PricingCurrency = %q", task3.PricingCurrency)
	}
}

// cachedPricing 带缓存读单价的定价：输入 10 微单位/1M、缓存读 1 微单位/1M。
func cachedPricing(t *testing.T) vo.Pricing {
	t.Helper()
	p, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{
		{InputMicro: 10_000_000, CacheReadMicro: 1_000_000},
	})
	if err != nil {
		t.Fatalf("NewPricing(cached) error = %v", err)
	}
	return p
}

// TestPriceModelCall_OpenAICachedInputNotDoubleCharged 锁定 OpenAI 语义的计费：
// prompt_tokens 含 cached_tokens，归一化后缓存命中部分只按缓存读价计一次，
// 不得再被输入价重复计费。
func TestPriceModelCall_OpenAICachedInputNotDoubleCharged(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{UpstreamStatusCode: http.StatusOK}
	task.SetTokensFromOpenAIUsage(&dto.OpenAICompletionUsage{
		PromptTokens:        1000,
		PromptTokensDetails: &dto.OpenAIPromptTokensDetails{CachedTokens: lo.ToPtr(800)},
	})
	usecase.PriceModelCall(task, cachedPricing(t))
	// 净输入 200×10 + 缓存读 800×1 = 2800 微单位；重复计费会得到 1000×10 + 800×1 = 10800
	if task.CostMicro == nil || *task.CostMicro != 2800 {
		t.Fatalf("CostMicro = %v, want 2800", task.CostMicro)
	}
	if task.InputCostMicro == nil || *task.InputCostMicro != 2000 {
		t.Fatalf("InputCostMicro = %v, want 2000", task.InputCostMicro)
	}
	if task.CacheReadCostMicro == nil || *task.CacheReadCostMicro != 800 {
		t.Fatalf("CacheReadCostMicro = %v, want 800", task.CacheReadCostMicro)
	}
}

// TestPriceModelCall_PromptTotalDrivesTierWithCachedInput 跳档按四维总量
// （净输入 + 缓存读）：总量 280k 落第二档，净输入按第二档单价计费。
func TestPriceModelCall_PromptTotalDrivesTierWithCachedInput(t *testing.T) {
	t.Parallel()
	task := &dto.ModelCallAuditTask{
		UpstreamStatusCode:   http.StatusOK,
		InputTokens:          30_000,
		CacheReadInputTokens: 250_000,
	}
	usecase.PriceModelCall(task, pricedPricing(t))
	// 总量 280_000 > 200_000 → 第二档 2_000_000 微单位/1M；输入维 30_000×2 = 60_000
	if task.CostMicro == nil || *task.CostMicro != 60_000 {
		t.Fatalf("CostMicro = %v, want 60000", task.CostMicro)
	}
}
