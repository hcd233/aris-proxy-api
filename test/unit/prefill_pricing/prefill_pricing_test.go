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

func TestPrefillPricingFound(t *testing.T) {
	t.Parallel()
	h := query.NewPrefillPricingHandler(&fakeQuoteProvider{quote: port.PricingQuote{Input: 3, Output: 15}, ok: true})
	rsp, err := h.Handle(t.Context(), port.PrefillPricingQuery{UpstreamModel: " gpt-4 "})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !rsp.Found {
		t.Fatalf("found case: %+v", rsp)
	}
	if rsp.Currency != enum.CurrencyUSD || rsp.InputPrice != 3 || rsp.OutputPrice != 15 {
		t.Fatalf("result = %+v", rsp)
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
