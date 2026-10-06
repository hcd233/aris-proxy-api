// Package audit_cost 成本查询用例的单元测试（合计/趋势、分组分布）
package audit_cost

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// fakeCostRepo 同时满足合计与分布两组窄仓储依赖
type fakeCostRepo struct {
	totals    []*modelcall.CostTotal
	series    []*modelcall.CostPoint
	dist      []*modelcall.CostDistributionPoint
	relations map[uint]*modelcall.AuditRelation
}

func (f *fakeCostRepo) SumCostByCurrency(context.Context, []uint, time.Time, time.Time) ([]*modelcall.CostTotal, error) {
	return f.totals, nil
}

func (f *fakeCostRepo) QueryCostSeries(context.Context, []uint, time.Time, time.Time, enum.Granularity) ([]*modelcall.CostPoint, error) {
	return f.series, nil
}

func (f *fakeCostRepo) QueryCostDistribution(context.Context, []uint, enum.CostGroupBy, time.Time, time.Time) ([]*modelcall.CostDistributionPoint, error) {
	return f.dist, nil
}

func (f *fakeCostRepo) BatchGetRelations(_ context.Context, keyIDs []uint) (map[uint]*modelcall.AuditRelation, error) {
	out := map[uint]*modelcall.AuditRelation{}
	for _, id := range keyIDs {
		if rel, ok := f.relations[id]; ok {
			out[id] = rel
		}
	}
	return out, nil
}

type fakeKeyLookup struct{ ids []uint }

func (f *fakeKeyLookup) LookupIDsByUserID(context.Context, uint) ([]uint, error) {
	return f.ids, nil
}

func TestCostSummaryByUserNoKeysReturnsEmpty(t *testing.T) {
	t.Parallel()
	repo := &fakeCostRepo{totals: []*modelcall.CostTotal{{Currency: "USD", CostMicro: 1}}}
	h := query.NewCostSummaryByUserHandler(repo, &fakeKeyLookup{ids: nil})
	rsp, err := h.Handle(t.Context(), query.CostSummaryByUserQuery{UserID: 7})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// 名下无 Key：空结果，绝不能退化为全量（越权）
	if len(rsp.Totals) != 0 || len(rsp.Series) != 0 {
		t.Fatalf("must return empty for user without keys, got %+v", rsp)
	}
}

func TestCostDistributionMergesKeysToUser(t *testing.T) {
	t.Parallel()
	repo := &fakeCostRepo{
		dist: []*modelcall.CostDistributionPoint{
			{ID: "1", Currency: "USD", CostMicro: 100},
			{ID: "2", Currency: "USD", CostMicro: 250},
			{ID: "2", Currency: "CNY", CostMicro: 700},
			{ID: "3", Currency: "USD", CostMicro: 50},
		},
		relations: map[uint]*modelcall.AuditRelation{
			1: {APIKeyID: 1, APIKeyName: "k1", UserID: 10, UserName: "alice"},
			2: {APIKeyID: 2, APIKeyName: "k2", UserID: 10, UserName: "alice"},
			3: {APIKeyID: 3, APIKeyName: "k3", UserID: 20, UserName: "bob"},
		},
	}
	h := query.NewCostDistributionHandler(repo)
	rsp, err := h.Handle(t.Context(), query.CostDistributionQuery{
		GroupBy: enum.CostGroupByUser, Limit: 10,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// alice: USD 350（key1+key2 合并）、CNY 700；bob: USD 50
	if len(rsp.Items) != 3 {
		t.Fatalf("items = %+v", rsp.Items)
	}
	byKey := map[string]int64{}
	for _, it := range rsp.Items {
		byKey[it.Name+"|"+it.Currency] = it.CostMicro
	}
	if byKey["alice|USD"] != 350 || byKey["alice|CNY"] != 700 || byKey["bob|USD"] != 50 {
		t.Fatalf("merged = %+v", byKey)
	}
}

func TestCostDistributionTrimsPerCurrency(t *testing.T) {
	t.Parallel()
	repo := &fakeCostRepo{
		dist: []*modelcall.CostDistributionPoint{
			{ID: "m1", Currency: "USD", CostMicro: 300},
			{ID: "m2", Currency: "USD", CostMicro: 200},
			{ID: "m3", Currency: "USD", CostMicro: 100},
			{ID: "m4", Currency: "CNY", CostMicro: 900},
			{ID: "m5", Currency: "CNY", CostMicro: 800},
		},
	}
	h := query.NewCostDistributionHandler(repo)
	rsp, err := h.Handle(t.Context(), query.CostDistributionQuery{
		GroupBy: enum.CostGroupByModel, Limit: 2,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// 每币种取前 2：USD 2 条 + CNY 2 条
	if len(rsp.Items) != 4 {
		t.Fatalf("items = %+v", rsp.Items)
	}
	if rsp.Items[0].ID != "m1" || rsp.Items[2].ID != "m4" {
		t.Fatalf("order = %+v", rsp.Items)
	}
}

func TestCostDistributionRejectsUserGroupForPlainUser(t *testing.T) {
	t.Parallel()
	h := query.NewCostDistributionByUserHandler(&fakeCostRepo{}, &fakeKeyLookup{ids: []uint{1}})
	_, err := h.Handle(t.Context(), query.CostDistributionByUserQuery{
		UserID: 7, GroupBy: enum.CostGroupByUser,
	})
	if err == nil {
		t.Fatal("group_by=user must be rejected for non-admin")
	}
}
