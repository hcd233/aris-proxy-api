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

func TestPrefillSpecLastTierBoundedBySpecContextLength(t *testing.T) {
	t.Parallel()
	// gpt-6-astra 实测档位（openai 源）：平铺价 10/50/12.5/1 + tier size=272000（20/75/25/2），
	// limit.context=1050000。末档上界取 spec 自己的 ContextLength，区间平铺 [0, contextLength)。
	h := query.NewPrefillSpecHandler(&fakeSpecProvider{
		ok: true,
		spec: port.ModelSpec{
			ContextLength: 1050000,
			Quote: port.PricingQuote{Tiers: []port.PricingTier{
				{ContextMin: 0, Input: 10, Output: 50, CacheCreation: 12.5, CacheRead: 1},
				{ContextMin: 272000, Input: 20, Output: 75, CacheCreation: 25, CacheRead: 2},
			}},
		},
	})
	rsp, err := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "gpt-6-astra"})
	if err != nil || !rsp.Found {
		t.Fatalf("found case: %+v err=%v", rsp, err)
	}
	// 回归：仅两段（不再追加同价兜底默认规则），末档上界=1050000 而非表单旧值
	if len(rsp.Rules) != 2 {
		t.Fatalf("rules = %+v", rsp.Rules)
	}
	first := rsp.Rules[0]
	if first.ContextMin != 0 || first.ContextMax != 272000 || first.InputPrice != 10 {
		t.Fatalf("first rule = %+v", first)
	}
	last := rsp.Rules[1]
	if last.ContextMin != 272000 || last.ContextMax != 1050000 || last.InputPrice != 20 ||
		last.OutputPrice != 75 || last.CacheCreationPrice != 25 || last.CacheReadPrice != 2 {
		t.Fatalf("last rule = %+v", last)
	}
}

func TestPrefillSpecContextLengthNotBeyondLastTier(t *testing.T) {
	t.Parallel()
	quote := port.PricingQuote{Tiers: []port.PricingTier{
		{ContextMin: 0, Input: 2, Output: 10},
		{ContextMin: 272000, Input: 4, Output: 15},
	}}
	// spec.ContextLength 等于末档起点 / 小于末档起点 / 为 0：末档均退化为无条件默认规则（不展开）
	for _, cl := range []int64{272000, 200000, 0} {
		h := query.NewPrefillSpecHandler(&fakeSpecProvider{
			ok:   true,
			spec: port.ModelSpec{ContextLength: cl, Quote: quote},
		})
		rsp, err := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "m"})
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
	// 单档即使 spec.ContextLength 有值也不展开（全区间同价，一条默认规则即完整）
	h2 := query.NewPrefillSpecHandler(&fakeSpecProvider{
		ok: true,
		spec: port.ModelSpec{
			ContextLength: 1000000,
			Quote:         port.PricingQuote{Tiers: []port.PricingTier{{ContextMin: 0, Input: 3}}},
		},
	})
	rsp2, _ := h2.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "m"})
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
