// Package prefill_spec 模型规格导入查询用例的单元测试
package prefill_spec

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/application/model/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type fakeSpecProvider struct {
	spec port.ModelSpec
	ok   bool
	err  error
}

func (f *fakeSpecProvider) Describe(_ context.Context, _ string) (port.ModelSpec, bool, error) {
	return f.spec, f.ok, f.err
}

func TestPrefillSpecTiersDefaultWhenNoContextLength(t *testing.T) {
	t.Parallel()
	h := query.NewPrefillSpecHandler(&fakeSpecProvider{
		ok: true,
		spec: port.ModelSpec{Quote: port.PricingQuote{Tiers: []port.PricingTier{
			{ContextMin: 0, Input: 1.25, Output: 10},
			{ContextMin: 200000, Input: 2.5, Output: 15},
		}}},
	})
	rsp, err := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: " gemini-2.5-pro "})
	if err != nil || !rsp.Found {
		t.Fatalf("found case: %+v err=%v", rsp, err)
	}
	if rsp.Currency != enum.CurrencyUSD || len(rsp.Rules) != 2 {
		t.Fatalf("result = %+v", rsp)
	}
	// 未传 ContextLength：首档 [0, 200000)，末档归为无条件默认规则（0/0）
	first := rsp.Rules[0]
	if first.ContextMin != 0 || first.ContextMax != 200000 || first.InputPrice != 1.25 {
		t.Fatalf("first rule = %+v", first)
	}
	last := rsp.Rules[1]
	if last.ContextMin != 0 || last.ContextMax != 0 || last.InputPrice != 2.5 {
		t.Fatalf("last rule (default) = %+v", last)
	}
}

func TestPrefillSpecLastTierBoundedByContextLength(t *testing.T) {
	t.Parallel()
	// gpt-6.1-sol 实测档位：首档平铺价 + tier size=272000；模型上下文 1000000
	h := query.NewPrefillSpecHandler(&fakeSpecProvider{
		ok: true,
		spec: port.ModelSpec{Quote: port.PricingQuote{Tiers: []port.PricingTier{
			{ContextMin: 0, Input: 2, Output: 10, CacheCreation: 2.5, CacheRead: 0.1},
			{ContextMin: 272000, Input: 4, Output: 15, CacheCreation: 5, CacheRead: 0.2},
		}}},
	})
	rsp, err := h.Handle(t.Context(), port.PrefillSpecQuery{
		UpstreamModel: "gpt-6.1-sol",
		ContextLength: 1000000,
	})
	if err != nil || !rsp.Found {
		t.Fatalf("found case: %+v err=%v", rsp, err)
	}
	// 末档展开为 [272000, 1000000)，其后追加同价无条件默认规则兜底
	if len(rsp.Rules) != 3 {
		t.Fatalf("rules = %+v", rsp.Rules)
	}
	mid := rsp.Rules[1]
	if mid.ContextMin != 272000 || mid.ContextMax != 1000000 || mid.InputPrice != 4 || mid.OutputPrice != 15 {
		t.Fatalf("mid rule = %+v", mid)
	}
	fallback := rsp.Rules[2]
	if fallback.ContextMin != 0 || fallback.ContextMax != 0 {
		t.Fatalf("fallback must be unconditional default: %+v", fallback)
	}
	if fallback.InputPrice != mid.InputPrice || fallback.OutputPrice != mid.OutputPrice ||
		fallback.CacheCreationPrice != mid.CacheCreationPrice || fallback.CacheReadPrice != mid.CacheReadPrice {
		t.Fatalf("fallback price must equal last tier: %+v vs %+v", fallback, mid)
	}
}

func TestPrefillSpecContextLengthNotBeyondLastTier(t *testing.T) {
	t.Parallel()
	quote := port.PricingQuote{Tiers: []port.PricingTier{
		{ContextMin: 0, Input: 2, Output: 10},
		{ContextMin: 272000, Input: 4, Output: 15},
	}}
	// ContextLength 等于末档起点 / 小于末档起点 / 为 0：末档均退化为无条件默认规则（不展开）
	for _, cl := range []int64{272000, 200000, 0} {
		h := query.NewPrefillSpecHandler(&fakeSpecProvider{ok: true, spec: port.ModelSpec{Quote: quote}})
		rsp, err := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "m", ContextLength: cl})
		if err != nil || !rsp.Found || len(rsp.Rules) != 2 {
			t.Fatalf("contextLength=%d: %+v err=%v", cl, rsp, err)
		}
		last := rsp.Rules[1]
		if last.ContextMin != 0 || last.ContextMax != 0 {
			t.Fatalf("contextLength=%d: last rule must stay default: %+v", cl, last)
		}
	}
}

func TestPrefillSpecSingleTierBecomesDefault(t *testing.T) {
	t.Parallel()
	h := query.NewPrefillSpecHandler(&fakeSpecProvider{
		ok:   true,
		spec: port.ModelSpec{Quote: port.PricingQuote{Tiers: []port.PricingTier{{ContextMin: 0, Input: 3}}}},
	})
	rsp, _ := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "m"})
	if !rsp.Found || len(rsp.Rules) != 1 || rsp.Rules[0].InputPrice != 3 {
		t.Fatalf("single tier = %+v", rsp)
	}
	// 单档即使传入 ContextLength 也不展开（全区间同价，一条默认规则即完整）
	rsp2, _ := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "m", ContextLength: 1000000})
	if !rsp2.Found || len(rsp2.Rules) != 1 || rsp2.Rules[0].InputPrice != 3 {
		t.Fatalf("single tier with contextLength = %+v", rsp2)
	}
}

func TestPrefillSpecDegradesOnMissOrError(t *testing.T) {
	t.Parallel()
	// 拉取失败降级为未命中，不报错（录入不被阻塞）
	h := query.NewPrefillSpecHandler(&fakeSpecProvider{err: ierr.New(ierr.ErrInternal, "boom")})
	rsp, err := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "x"})
	if err != nil || rsp.Found {
		t.Fatalf("error case must degrade: %+v err=%v", rsp, err)
	}
	// 未命中
	h2 := query.NewPrefillSpecHandler(&fakeSpecProvider{ok: false})
	rsp2, _ := h2.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "x"})
	if rsp2.Found {
		t.Fatalf("miss case: %+v", rsp2)
	}
	// 空模型名短路
	rsp3, _ := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "  "})
	if rsp3.Found {
		t.Fatal("blank model should miss")
	}
}

func TestPrefillSpecFieldsAndModalities(t *testing.T) {
	t.Parallel()
	h := query.NewPrefillSpecHandler(&fakeSpecProvider{ok: true, spec: port.ModelSpec{
		ContextLength:   200000,
		MaxOutputTokens: 64000,
		InputModalities: []enum.InputModality{enum.InputModalityText, enum.InputModalityPDF},
		Quote:           port.PricingQuote{Tiers: []port.PricingTier{{ContextMin: 0, Input: 1, Output: 5}}},
	}})
	rsp, err := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "claude-sonnet-4-5"})
	if err != nil || !rsp.Found {
		t.Fatalf("found case: %+v err=%v", rsp, err)
	}
	if rsp.ContextLength != 200000 || rsp.MaxOutputTokens != 64000 {
		t.Fatalf("spec fields = %+v", rsp)
	}
	if len(rsp.InputModalities) != 2 || rsp.InputModalities[1] != enum.InputModalityPDF {
		t.Fatalf("modalities = %v", rsp.InputModalities)
	}
	if rsp.Currency != enum.CurrencyUSD || len(rsp.Rules) != 1 {
		t.Fatalf("pricing = %+v", rsp)
	}
}
