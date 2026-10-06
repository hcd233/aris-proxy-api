// Package prefill_pricing 定价导入查询用例的单元测试
package prefill_pricing

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/application/model/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type fakeQuoteProvider struct {
	quote port.PricingQuote
	ok    bool
	err   error
}

func (f *fakeQuoteProvider) Quote(_ context.Context, _ string) (port.PricingQuote, bool, error) {
	return f.quote, f.ok, f.err
}

func TestPrefillPricingTiersToRules(t *testing.T) {
	t.Parallel()
	h := query.NewPrefillPricingHandler(&fakeQuoteProvider{
		ok: true,
		quote: port.PricingQuote{Tiers: []port.PricingTier{
			{ContextMin: 0, Input: 1.25, Output: 10},
			{ContextMin: 200000, Input: 2.5, Output: 15},
		}},
	})
	rsp, err := h.Handle(t.Context(), port.PrefillPricingQuery{UpstreamModel: " gemini-2.5-pro "})
	if err != nil || !rsp.Found {
		t.Fatalf("found case: %+v err=%v", rsp, err)
	}
	if rsp.Currency != enum.CurrencyUSD || len(rsp.Rules) != 2 {
		t.Fatalf("result = %+v", rsp)
	}
	// 首档 [0, 200000)，末档归为无条件默认规则（0/0）
	first := rsp.Rules[0]
	if first.ContextMin != 0 || first.ContextMax != 200000 || first.InputPrice != 1.25 {
		t.Fatalf("first rule = %+v", first)
	}
	last := rsp.Rules[1]
	if last.ContextMin != 0 || last.ContextMax != 0 || last.InputPrice != 2.5 {
		t.Fatalf("last rule (default) = %+v", last)
	}
}

func TestPrefillPricingSingleTierBecomesDefault(t *testing.T) {
	t.Parallel()
	h := query.NewPrefillPricingHandler(&fakeQuoteProvider{
		ok:    true,
		quote: port.PricingQuote{Tiers: []port.PricingTier{{ContextMin: 0, Input: 3}}},
	})
	rsp, _ := h.Handle(t.Context(), port.PrefillPricingQuery{UpstreamModel: "m"})
	if !rsp.Found || len(rsp.Rules) != 1 || rsp.Rules[0].InputPrice != 3 {
		t.Fatalf("single tier = %+v", rsp)
	}
}

func TestPrefillPricingDegradesOnMissOrError(t *testing.T) {
	t.Parallel()
	// 拉取失败降级为未命中，不报错（录入不被阻塞）
	h := query.NewPrefillPricingHandler(&fakeQuoteProvider{err: ierr.New(ierr.ErrInternal, "boom")})
	rsp, err := h.Handle(t.Context(), port.PrefillPricingQuery{UpstreamModel: "x"})
	if err != nil || rsp.Found {
		t.Fatalf("error case must degrade: %+v err=%v", rsp, err)
	}
	// 未命中
	h2 := query.NewPrefillPricingHandler(&fakeQuoteProvider{ok: false})
	rsp2, _ := h2.Handle(t.Context(), port.PrefillPricingQuery{UpstreamModel: "x"})
	if rsp2.Found {
		t.Fatalf("miss case: %+v", rsp2)
	}
	// 空模型名短路
	rsp3, _ := h.Handle(t.Context(), port.PrefillPricingQuery{UpstreamModel: "  "})
	if rsp3.Found {
		t.Fatal("blank model should miss")
	}
}
