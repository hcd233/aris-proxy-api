// Package pricing_dto 定价 wire 换算与 DTO↔值对象映射的单元测试
package pricing_dto

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestPriceMicroRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []float64{0, 0.8, 4, 0.000001, 2.5, 1_000_000}
	for _, v := range cases {
		if got := dto.PriceDisplayFromMicro(dto.PriceMicroFromDisplay(v)); got != v {
			t.Fatalf("round trip %v -> %v", v, got)
		}
	}
	if got := dto.PriceMicroFromDisplay(0.5); got != 500_000 {
		t.Fatalf("0.5 display = %d micro", got)
	}
}

func TestPricingFromDTO(t *testing.T) {
	t.Parallel()
	// nil → 未计价
	p, err := port.PricingFromDTO(nil)
	if err != nil || p.IsPriced() {
		t.Fatalf("nil dto: p=%+v err=%v", p, err)
	}
	// 合法：条件规则 + 默认规则
	p, err = port.PricingFromDTO(&dto.PricingDTO{
		Currency: enum.CurrencyUSD,
		Rules: []dto.PricingRuleDTO{
			{TimeWindows: []dto.TimeWindowDTO{{Days: []int{1}, Start: "00:30", End: "08:30", Timezone: "UTC"}},
				ContextMax: 200000, InputPrice: 1},
			{InputPrice: 2},
		},
	})
	if err != nil || !p.IsPriced() {
		t.Fatalf("valid dto: err=%v", err)
	}
	if got := port.PricingToDTO(p); got == nil || got.Currency != enum.CurrencyUSD || len(got.Rules) != 2 {
		t.Fatalf("round trip dto = %+v", got)
	}
	// 非法：缺币种但有规则
	if _, err := port.PricingFromDTO(&dto.PricingDTO{Rules: []dto.PricingRuleDTO{{InputPrice: 1}}}); err == nil {
		t.Fatal("missing currency should be rejected")
	}
	// 合法：无默认规则但区间从 0 连续覆盖（末档可有界，超出回落末档价）
	if _, err := port.PricingFromDTO(&dto.PricingDTO{
		Currency: enum.CurrencyUSD,
		Rules:    []dto.PricingRuleDTO{{ContextMax: 200000, InputPrice: 1}},
	}); err != nil {
		t.Fatalf("contiguous bands without default should be accepted: %v", err)
	}
	// 非法：无默认规则且区间断档
	if _, err := port.PricingFromDTO(&dto.PricingDTO{
		Currency: enum.CurrencyUSD,
		Rules: []dto.PricingRuleDTO{
			{ContextMax: 100000, InputPrice: 1},
			{ContextMin: 200000, ContextMax: 300000, InputPrice: 2},
		},
	}); err == nil {
		t.Fatal("gapped bands without default should be rejected")
	}
	// 未计价（显式清空）
	p2, err := port.PricingFromDTO(&dto.PricingDTO{Currency: enum.CurrencyNone, Rules: []dto.PricingRuleDTO{}})
	if err != nil || p2.IsPriced() {
		t.Fatalf("explicit clear: p=%+v err=%v", p2, err)
	}
	// 非法：已废弃币种
	if _, err := port.PricingFromDTO(&dto.PricingDTO{
		Currency: enum.Currency("CNY"),
		Rules:    []dto.PricingRuleDTO{{InputPrice: 1}},
	}); err == nil {
		t.Fatal("deprecated CNY currency should be rejected")
	}
}
