// Package trace_e2e 端到端验证：hook 上报 → handler → usecase → 落库（fake repo 驱动，免 DB）
package trace_e2e

import (
	"context"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	tracefake "github.com/hcd233/aris-proxy-api/test/unit/trace"
)

func TestE2E_TraceReportFlow(t *testing.T) {
	t.Parallel()

	repo := tracefake.NewFakeRepo()
	h := handler.NewTraceHandler(handler.TraceDependencies{
		Report: command.NewReportTraceEventHandler(repo),
	})

	ctx := context.WithValue(context.Background(), constant.CtxKeyUserID, uint(7))
	ctx = context.WithValue(ctx, constant.CtxKeyAPIKeyName, "e2e-key")

	payload, err := sonic.Marshal(map[string]any{
		"hook_event_name": "UserPromptSubmit",
		"session_id":      "e2e-s1",
		"prompt":          "hello",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	body := &dto.ReportTraceEventReqBody{}
	if err := sonic.Unmarshal(payload, body); err != nil {
		t.Fatalf("unmarshal report body: %v", err)
	}

	rsp, err := h.HandleReportTraceEvent(ctx, &dto.ReportTraceEventReq{Body: body})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rsp == nil || rsp.Body == nil {
		t.Fatal("expected non-nil response")
	}

	tr, ferr := repo.FindBySessionID(context.Background(), "e2e-s1")
	if ferr != nil {
		t.Fatalf("FindBySessionID: %v", ferr)
	}
	if tr == nil {
		t.Fatal("trace not persisted")
	}
	if tr.APIKeyName != "e2e-key" || tr.UserID != 7 {
		t.Fatalf("unexpected trace ownership: %+v", tr)
	}
	if n, _ := repo.CountEvents(context.Background(), tr.ID); n != 1 {
		t.Fatalf("expected 1 event persisted, got %d", n)
	}
}
