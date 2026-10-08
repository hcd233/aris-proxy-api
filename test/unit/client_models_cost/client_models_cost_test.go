// Package client_models_cost 客户端模型分发接口下发基础档单价的单元测试
package client_models_cost

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// detailsRepo 只实现 ListEnabledModelDetails，其余方法经嵌入接口满足方法集（不会被调用）
type detailsRepo struct {
	llmproxy.EndpointReadRepository
	details []*llmproxy.ModelDetailProjection
}

func (r *detailsRepo) ListEnabledModelDetails(context.Context, uint) ([]*llmproxy.ModelDetailProjection, error) {
	return r.details, nil
}

func TestListClientModelsCost(t *testing.T) {
	t.Parallel()
	priced, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{
		{TimeWindows: []vo.TimeWindow{{Start: "00:00", End: "08:00"}}, InputMicro: 1},
		{ContextMin: 200_000, InputMicro: 9_000_000},
		{ContextMax: 200_000, InputMicro: 1_250_000, OutputMicro: 10_000_000, CacheReadMicro: 125_000, CacheCreateMicro: 1_500_000},
	})
	if err != nil {
		t.Fatalf("NewPricing: %v", err)
	}
	repo := &detailsRepo{details: []*llmproxy.ModelDetailProjection{
		{Alias: "priced", Pricing: priced},
		{Alias: "free"},
	}}
	ctx := context.WithValue(t.Context(), constant.CtxKeyUserID, uint(1))
	rsp, err := usecase.NewListClientModels(repo).Handle(ctx)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// 基础档 = 起点 0 的无时段区间（跳过时段规则与高档位）
	c := rsp.Models[0].Cost
	if c == nil || c.Input != 1.25 || c.Output != 10 || c.CacheRead != 0.125 || c.CacheWrite != 1.5 {
		t.Fatalf("priced cost = %+v", c)
	}
	if rsp.Models[1].Cost != nil {
		t.Fatalf("unpriced model must omit cost, got %+v", rsp.Models[1].Cost)
	}
}
