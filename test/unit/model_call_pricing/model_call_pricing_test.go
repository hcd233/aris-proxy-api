// Package model_call_pricing 审计计费粘连的单元测试（PriceModelCall）
package model_call_pricing

import (
	"net/http"
	"testing"

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
	free, err := vo.NewPricing(enum.CurrencyCNY, []vo.PricingRule{{}})
	if err != nil {
		t.Fatalf("NewPricing(free) error = %v", err)
	}
	task3 := &dto.ModelCallAuditTask{UpstreamStatusCode: http.StatusOK, InputTokens: 1000}
	usecase.PriceModelCall(task3, free)
	if task3.CostMicro == nil || *task3.CostMicro != 0 {
		t.Fatalf("free model cost = %v, want 0", task3.CostMicro)
	}
	if task3.PricingCurrency != string(enum.CurrencyCNY) {
		t.Fatalf("PricingCurrency = %q", task3.PricingCurrency)
	}
}
