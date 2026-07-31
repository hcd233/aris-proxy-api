package trace_e2e

import (
	"context"
	"strconv"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	traceschema "github.com/hcd233/aris-proxy-api/internal/dto/schema"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	tracefake "github.com/hcd233/aris-proxy-api/test/unit/trace"
)

// TestE2E_TraceDeleteFlow 上报建 trace → 删除 → 列表不含 → 同 session 再上报被拒
func TestE2E_TraceDeleteFlow(t *testing.T) {
	t.Parallel()

	repo := tracefake.NewFakeRepo()
	apiKeyRepo := newE2EAPIKeyRepo(map[uint][]string{7: {"e2e-key"}})
	h := handler.NewTraceHandler(handler.TraceDependencies{
		Report: command.NewReportTraceEventHandler(repo),
		Delete: command.NewDeleteTraceHandler(repo, apiKeyRepo),
	})

	ctx := context.WithValue(context.Background(), constant.CtxKeyUserID, uint(7))
	ctx = context.WithValue(ctx, constant.CtxKeyAPIKeyName, "e2e-key")

	body := &dto.ReportTraceEventReqBody{
		SessionID: "e2e-del",
		Agent:     constant.TraceAgentCodex,
		Records: []*dto.ReportTraceRecordReq{{
			Source:        constant.TraceRecordSourceHook,
			RecordType:    constant.TraceRecordTypeHookEvent,
			HookEventName: "UserPromptSubmit",
			DedupKey:      "hook:e2e-del:1",
			Payload:       traceschema.RawJSON(`{"hook_event_name":"UserPromptSubmit","session_id":"e2e-del"}`),
		}},
	}
	if _, err := h.HandleReportTraceEvent(ctx, &dto.ReportTraceEventReq{Body: body}); err != nil {
		t.Fatalf("report: %v", err)
	}
	tr, _ := repo.FindBySessionID(context.Background(), "e2e-del")
	if tr == nil {
		t.Fatal("trace not persisted before delete")
	}

	// 删除
	delRsp, err := h.HandleDeleteTraces(ctx, &dto.DeleteTraceReq{IDs: strconv.FormatUint(uint64(tr.ID), constant.DecimalBase)})
	if err != nil || delRsp == nil || delRsp.Body == nil {
		t.Fatalf("delete: rsp=%+v err=%v", delRsp, err)
	}
	if delRsp.Body.DeletedCount != 1 || len(delRsp.Body.Failures) != 0 {
		t.Fatalf("expected 1 deleted, got %+v", delRsp.Body)
	}

	// 列表不再包含（fake 的 FindBySessionID 过滤软删）
	if again, _ := repo.FindBySessionID(context.Background(), "e2e-del"); again != nil {
		t.Fatal("trace should be gone from normal lookup after delete")
	}

	// 同 session 再上报 → 全部 rejected
	reRsp, err := h.HandleReportTraceEvent(ctx, &dto.ReportTraceEventReq{Body: body})
	if err != nil {
		t.Fatalf("re-report: %v", err)
	}
	if reRsp == nil || reRsp.Body == nil || len(reRsp.Body.Results) != 1 ||
		reRsp.Body.Results[0].Status != constant.TraceRecordStatusRejected {
		t.Fatalf("expected rejected on re-report, got %+v", reRsp.Body)
	}
	if n, _ := repo.CountEvents(context.Background(), tr.ID); n != 1 {
		t.Fatalf("expected events unchanged after rejected re-report, got %d", n)
	}
}

// e2eAPIKeyRepo 最小 API Key 仓储，仅供删除流程 owner 查询
type e2eAPIKeyRepo struct {
	owners map[uint][]string
}

func newE2EAPIKeyRepo(owners map[uint][]string) *e2eAPIKeyRepo {
	return &e2eAPIKeyRepo{owners: owners}
}

func (r *e2eAPIKeyRepo) Save(_ context.Context, _ *aggregate.ProxyAPIKey) error { return nil }
func (r *e2eAPIKeyRepo) FindByID(_ context.Context, _ uint) (*aggregate.ProxyAPIKey, error) {
	return nil, nil
}
func (r *e2eAPIKeyRepo) ListByUser(_ context.Context, _ uint) ([]*aggregate.ProxyAPIKey, error) {
	return nil, nil
}
func (r *e2eAPIKeyRepo) ListAll(_ context.Context) ([]*aggregate.ProxyAPIKey, error) {
	return nil, nil
}
func (r *e2eAPIKeyRepo) PaginateByUser(_ context.Context, _ uint, _ model.CommonParam) ([]*aggregate.ProxyAPIKey, *model.PageInfo, error) {
	return nil, nil, nil
}
func (r *e2eAPIKeyRepo) PaginateAll(_ context.Context, _ model.CommonParam) ([]*aggregate.ProxyAPIKey, *model.PageInfo, error) {
	return nil, nil, nil
}
func (r *e2eAPIKeyRepo) CountByUser(_ context.Context, _ uint) (int64, error) { return 0, nil }
func (r *e2eAPIKeyRepo) Delete(_ context.Context, _ uint) error               { return nil }
func (r *e2eAPIKeyRepo) LookupOwnerNamesByUserID(_ context.Context, userID uint) ([]string, error) {
	return r.owners[userID], nil
}
func (r *e2eAPIKeyRepo) LookupIDsByUserID(_ context.Context, _ uint) ([]uint, error) {
	return nil, nil
}
