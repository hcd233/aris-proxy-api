// Package audit_query 审计查询的单元测试 —— demo 视角分发
package audit_query

import (
	"context"
	"testing"
	"time"

	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
)

func TestAuditService_DispatchesDemoToFullAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)

	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, param model.CommonParam, _, _ time.Time, _ *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			return nil, &model.PageInfo{Page: 1, PageSize: 20, Total: 0}, nil
		},
	}
	svc := auditquery.NewAuditService(
		auditquery.NewListAllAuditLogsHandler(repo),
		auditquery.NewListAuditLogsByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewListAuditOptionHandler(repo),
		auditquery.NewModelTrendHandler(repo),
		auditquery.NewModelTrendByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewRequestRateHandler(repo),
		auditquery.NewRequestRateByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewTokenThroughputHandler(repo),
		auditquery.NewTokenThroughputByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewTokenRateHandler(repo),
		auditquery.NewTokenRateByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewModelUsageHandler(repo),
		auditquery.NewModelUsageByUserHandler(repo, &fakeAPIKeyIDLookup{}),
		auditquery.NewFirstTokenLatencyHandler(repo),
		auditquery.NewFirstTokenLatencyByUserHandler(repo, &fakeAPIKeyIDLookup{}),
	)

	if _, _, err := svc.ListLogs(ctx, enum.PermissionDemo, 1, auditport.ListAuditLogsParams{
		Page: 1, PageSize: 20, StartTime: t1, EndTime: t2,
	}); err != nil {
		t.Fatalf("demo ListLogs err: %v", err)
	}
	if repo.listAllCalls != 1 {
		t.Fatalf("expected listAll called once, got %d", repo.listAllCalls)
	}
	if repo.listByAPIKeyIDsCnt != 0 {
		t.Fatalf("expected no byUser dispatch for demo, got %d", repo.listByAPIKeyIDsCnt)
	}
}
