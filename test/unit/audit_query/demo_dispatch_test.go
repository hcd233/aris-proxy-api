// Package audit_query 审计查询的单元测试 —— demo 视角分发
package audit_query

import (
	"context"
	"testing"
	"time"

	auditport "github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/vo"
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

func TestAuditService_DemoListLogsMasksIdentityAndConnection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	audit := aggregate.ReconstructAudit(aggregate.ReconstructAuditInput{
		APIKeyID:         1,
		ModelID:          "gpt-4",
		UpstreamProtocol: enum.ProtocolOpenAIChatCompletion,
		APIProtocol:      enum.ProtocolOpenAIChatCompletion,
		Endpoint:         "openai-chat-completions",
		Tokens:           vo.NewTokenBreakdown(10, 20, 0, 0),
		Latency:          vo.NewCallLatency(time.Second, time.Second),
		Status:           vo.NewCallStatus(200, ""),
		UserAgent:        "Mozilla/5.0",
		TraceID:          "trace-id-12345678",
		CreatedAt:        t1,
	})

	repo := &fakeAuditRepo{
		listAllFunc: func(ctx context.Context, _ model.CommonParam, _, _ time.Time, _ *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
			return []*aggregate.ModelCallAudit{audit}, &model.PageInfo{Page: 1, PageSize: 20, Total: 1}, nil
		},
		batchGetRelationsFn: func(ctx context.Context, apiKeyIDs []uint) (map[uint]*modelcall.AuditRelation, error) {
			return map[uint]*modelcall.AuditRelation{
				1: {APIKeyID: 1, APIKeyName: "sk-my-super-secret-key", UserName: "Alice", UserEmail: "alice@example.com"},
			}, nil
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

	views, _, err := svc.ListLogs(ctx, enum.PermissionDemo, 1, auditport.ListAuditLogsParams{
		Page: 1, PageSize: 20, StartTime: t1, EndTime: t1.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("demo ListLogs err: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("len(views) = %d, want 1", len(views))
	}
	v := views[0]
	if v.APIKeyName != "sk-m***-key" {
		t.Errorf("APIKeyName = %q, want %q", v.APIKeyName, "sk-m***-key")
	}
	if v.UserName != "***" {
		t.Errorf("UserName = %q, want %q", v.UserName, "***")
	}
	if v.UserEmail != "***" {
		t.Errorf("UserEmail = %q, want %q", v.UserEmail, "***")
	}
	if v.Endpoint != "open***ions" {
		t.Errorf("Endpoint = %q, want %q", v.Endpoint, "open***ions")
	}
	if v.TraceID != "trac***5678" {
		t.Errorf("TraceID = %q, want %q", v.TraceID, "trac***5678")
	}
	if v.ModelID != "gpt-4" {
		t.Errorf("ModelID = %q, want unmasked %q", v.ModelID, "gpt-4")
	}
}

func TestAuditService_DemoOptionsMaskUserField(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := &fakeAuditRepo{
		listDistinctUserNamesFn: func(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
			return []string{"alice", "bob", "alice"}, nil
		},
	}
	svc := auditquery.NewAuditService(
		nil,
		nil,
		auditquery.NewListAuditOptionHandler(repo),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	items, err := svc.ListAuditOption(ctx, enum.PermissionDemo, constant.AuditFilterFieldUser, "", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("list demo audit options: %v", err)
	}
	if len(items) != 1 || items[0] != "***" {
		t.Fatalf("items = %v, want [***]", items)
	}
}
