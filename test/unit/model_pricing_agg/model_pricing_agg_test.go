// Package model_pricing_agg 模型定价挂载的单元测试（聚合 UpdatePricing/Pricing）
package model_pricing_agg

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

func TestModelUpdatePricing(t *testing.T) {
	t.Parallel()
	m, err := aggregate.CreateModel(1, vo.EndpointAlias("gpt"), "gpt-4", 2, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel() error = %v", err)
	}
	if m.Pricing().IsPriced() {
		t.Fatalf("new model should be unpriced")
	}
	p, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{InputMicro: 1_000_000}})
	if err != nil {
		t.Fatalf("NewPricing() error = %v", err)
	}
	m.UpdatePricing(p)
	if got := m.Pricing().Currency(); got != enum.CurrencyUSD {
		t.Fatalf("Currency() = %v", got)
	}
	// 清空定价（未计价）
	m.UpdatePricing(vo.Pricing{})
	if m.Pricing().IsPriced() {
		t.Fatalf("pricing should be cleared")
	}
}
