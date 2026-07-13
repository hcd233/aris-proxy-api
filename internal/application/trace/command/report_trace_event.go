// Package command trace 写侧 usecase
package command

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type reportTraceEventHandler struct {
	repo trace.TraceRepository
}

// NewReportTraceEventHandler 构造上报 handler
func NewReportTraceEventHandler(repo trace.TraceRepository) port.ReportTraceEventHandler {
	return &reportTraceEventHandler{repo: repo}
}

type hookInput struct {
	HookEventName string `json:"hook_event_name"`
	SessionID     string `json:"session_id"`
	Model         string `json:"model"`
	CWD           string `json:"cwd"`
	Source        string `json:"source"`
	TurnID        string `json:"turn_id"`
}

func (h *reportTraceEventHandler) Handle(ctx context.Context, cmd port.ReportTraceEventCommand) error {
	var in hookInput
	if err := sonic.Unmarshal(cmd.RawPayload, &in); err != nil {
		return ierr.Wrap(ierr.ErrValidation, err, "parse hook payload")
	}
	if in.SessionID == "" {
		return ierr.New(ierr.ErrValidation, "hook payload missing session_id")
	}

	// 保证 trace 存在（SessionStart 可能丢失时兜底创建）
	t, err := h.repo.FindBySessionID(ctx, in.SessionID)
	if err != nil {
		return err
	}
	if t == nil {
		t, err = h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent: constant.TraceAgentCodex, SessionID: in.SessionID, APIKeyName: cmd.APIKeyName,
			UserID: cmd.UserID, Model: in.Model, CWD: in.CWD, Source: in.Source, Status: constant.TraceStatusActive,
		})
		if err != nil {
			return err
		}
	}

	switch in.HookEventName {
	case constant.TraceEventSessionStart:
		if _, err = h.repo.UpsertBySessionID(ctx, &trace.Trace{
			ID: t.ID, Agent: constant.TraceAgentCodex, SessionID: in.SessionID, APIKeyName: cmd.APIKeyName,
			UserID: cmd.UserID, Model: in.Model, CWD: in.CWD, Source: in.Source, Status: constant.TraceStatusActive,
		}); err != nil {
			return err
		}
	case constant.TraceEventStop:
		if err := h.repo.InsertEvent(ctx, &trace.TraceEvent{
			TraceID: t.ID, SessionID: in.SessionID, Event: in.HookEventName, TurnID: in.TurnID, Payload: cmd.RawPayload,
		}); err != nil {
			return err
		}
		return h.repo.MarkDone(ctx, in.SessionID)
	default:
		if err := h.repo.InsertEvent(ctx, &trace.TraceEvent{
			TraceID: t.ID, SessionID: in.SessionID, Event: in.HookEventName, TurnID: in.TurnID, Payload: cmd.RawPayload,
		}); err != nil {
			return err
		}
	}
	return nil
}
