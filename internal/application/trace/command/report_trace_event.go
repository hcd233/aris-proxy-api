// Package command trace 写侧 usecase
package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

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

func (h *reportTraceEventHandler) Handle(ctx context.Context, cmd port.ReportTraceEventCommand) error {
	if cmd.SessionID == "" {
		return ierr.New(ierr.ErrValidation, "hook payload missing session_id")
	}

	cmd.Records = normalizeRecords(cmd)
	t, err := h.ensureTrace(ctx, cmd)
	if err != nil {
		return err
	}
	if err := insertRecords(ctx, h.repo, t.ID, cmd.SessionID, cmd.Records); err != nil {
		return err
	}
	if recordsComplete(cmd.Records) {
		return h.repo.MarkDone(ctx, cmd.SessionID)
	}
	return nil
}

func normalizeRecords(cmd port.ReportTraceEventCommand) []port.ReportTraceRecord {
	records := cmd.Records
	if len(records) == 0 {
		records = []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: cmd.HookEventName, Event: cmd.HookEventName, TurnID: cmd.TurnID, Payload: cmd.RawPayload,
		}}
	}
	for i := range records {
		record := &records[i]
		if record.Source == "" {
			record.Source = constant.TraceRecordSourceHook
		}
		if record.RecordType == "" {
			record.RecordType = constant.TraceRecordTypeHookEvent
		}
		if record.Event == "" {
			record.Event = record.HookEventName
		}
		if record.HookEventName == "" {
			record.HookEventName = record.Event
		}
		if record.DedupKey == "" {
			hash := sha256.Sum256(record.Payload)
			record.DedupKey = record.Source + ":" + cmd.SessionID + ":" + record.HookEventName + ":" + record.TurnID + ":" + strconv.FormatInt(record.ClientSequence, 10) + ":" + hex.EncodeToString(hash[:])
		}
	}
	return records
}

func (h *reportTraceEventHandler) ensureTrace(ctx context.Context, cmd port.ReportTraceEventCommand) (*trace.Trace, error) {
	t, err := h.repo.FindBySessionID(ctx, cmd.SessionID)
	if err != nil || t == nil {
		if err != nil {
			return nil, err
		}
		return h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent: constant.TraceAgentCodex, SessionID: cmd.SessionID, APIKeyName: cmd.APIKeyName,
			UserID: cmd.UserID, Model: cmd.Model, CWD: cmd.CWD, Source: cmd.Source, Status: constant.TraceStatusActive,
		})
	}
	modelName, cwd, source := t.Model, t.CWD, t.Source
	if cmd.Model != "" {
		modelName = cmd.Model
	}
	if cmd.CWD != "" {
		cwd = cmd.CWD
	}
	if cmd.Source != "" {
		source = cmd.Source
	}
	return h.repo.UpsertBySessionID(ctx, &trace.Trace{
		ID: t.ID, Agent: constant.TraceAgentCodex, SessionID: cmd.SessionID, APIKeyName: t.APIKeyName,
		UserID: t.UserID, Model: modelName, CWD: cwd, Source: source, Status: t.Status, Metadata: t.Metadata,
	})
}

func insertRecords(ctx context.Context, repo trace.TraceRepository, traceID uint, sessionID string, records []port.ReportTraceRecord) error {
	for _, record := range records {
		if err := repo.InsertEvent(ctx, &trace.TraceEvent{
			TraceID: traceID, SessionID: sessionID, Source: record.Source, RecordType: record.RecordType,
			Event: record.Event, TurnID: record.TurnID, CallID: record.CallID, TranscriptLine: record.TranscriptLine,
			ClientSequence: record.ClientSequence, DedupKey: record.DedupKey, Payload: record.Payload,
		}); err != nil {
			return err
		}
	}
	return nil
}

func recordsComplete(records []port.ReportTraceRecord) bool {
	for _, record := range records {
		if record.Event == constant.TraceEventStop || record.Event == constant.TraceEventTaskComplete {
			return true
		}
	}
	return false
}
