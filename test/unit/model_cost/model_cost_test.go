// Package model_cost 模型成本排行查询用例的单元测试
package model_cost

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// fakeCostRepo 满足 modelCostHandler 对仓储的最小依赖面
type fakeCostRepo struct {
	points []*modelcall.ModelCostPoint
}

func (f *fakeCostRepo) QueryModelCost(_ context.Context, _ []uint, _, _ time.Time) ([]*modelcall.ModelCostPoint, error) {
	return f.points, nil
}

type fakeKeyLookup struct{ ids []uint }

func (f *fakeKeyLookup) LookupIDsByUserID(context.Context, uint) ([]uint, error) {
	return f.ids, nil
}

func TestModelCostConvertsMicroToDisplay(t *testing.T) {
	t.Parallel()
	repo := &fakeCostRepo{points: []*modelcall.ModelCostPoint{{
		ModelID:                "gpt",
		Currency:               "USD",
		InputCostMicro:         1000,
		OutputCostMicro:        2000,
		CacheCreationCostMicro: 300,
		CacheReadCostMicro:     20,
	}}}
	h := query.NewModelCostHandler(repo)
	items, err := h.Handle(t.Context(), query.ModelCostQuery{})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	got := items[0]
	if got.InputCost != 0.001 || got.OutputCost != 0.002 || got.CacheCreationCost != 0.0003 || got.CacheReadCost != 0.00002 {
		t.Fatalf("dims = %+v", got)
	}
	// 总费用 = 四维合计 = 3320 微单位 = 0.00332
	if got.TotalCost != 0.00332 {
		t.Fatalf("TotalCost = %v, want 0.00332", got.TotalCost)
	}
}

func TestModelCostByUserNoKeysReturnsEmpty(t *testing.T) {
	t.Parallel()
	repo := &fakeCostRepo{points: []*modelcall.ModelCostPoint{{ModelID: "gpt"}}}
	h := query.NewModelCostByUserHandler(repo, &fakeKeyLookup{ids: nil})
	items, err := h.Handle(t.Context(), query.ModelCostByUserQuery{UserID: 7})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// 名下无 Key：空结果，绝不能退化为全量（越权）
	if len(items) != 0 {
		t.Fatalf("must return empty for user without keys, got %+v", items)
	}
}
