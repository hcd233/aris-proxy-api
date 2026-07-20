// Package command trace 写侧 usecase
package command

import (
	"context"

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

func (h *reportTraceEventHandler) Handle(
	ctx context.Context,
	cmd port.ReportTraceEventCommand,
) ([]port.ReportTraceRecordResult, error) {
	if cmd.SessionID == "" {
		return nil, ierr.New(ierr.ErrValidation, "hook payload missing session_id")
	}

	isLegacy := len(cmd.Records) == 0
	records := normalizeRecords(cmd)
	t, err := h.ensureTrace(ctx, cmd)
	if err != nil {
		return nil, err
	}
	results, isComplete := insertRecords(
		ctx,
		h.repo,
		t.ID,
		cmd.SessionID,
		records,
		!isLegacy,
	)
	if isComplete {
		if err := h.repo.MarkDone(ctx, cmd.SessionID); err != nil {
			return results, err
		}
	}
	return results, nil
}

func normalizeRecords(cmd port.ReportTraceEventCommand) []port.ReportTraceRecord {
	records := cmd.Records
	if len(records) == 0 {
		return []port.ReportTraceRecord{{
			Source:        constant.TraceRecordSourceHook,
			RecordType:    constant.TraceRecordTypeHookEvent,
			HookEventName: cmd.HookEventName,
			Event:         cmd.HookEventName,
			TurnID:        cmd.TurnID,
			Payload:       cmd.RawPayload,
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
	}
	return records
}

func (h *reportTraceEventHandler) ensureTrace(
	ctx context.Context,
	cmd port.ReportTraceEventCommand,
) (*trace.Trace, error) {
	t, err := h.repo.FindBySessionID(ctx, cmd.SessionID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent:      constant.TraceAgentCodex,
			SessionID:  cmd.SessionID,
			APIKeyName: cmd.APIKeyName,
			UserID:     cmd.UserID,
			Model:      cmd.Model,
			CWD:        cmd.CWD,
			Source:     cmd.Source,
			Status:     constant.TraceStatusActive,
		})
	}

	modelName := t.Model
	if cmd.Model != "" {
		modelName = cmd.Model
	}
	cwd := t.CWD
	if cmd.CWD != "" {
		cwd = cmd.CWD
	}
	source := t.Source
	if cmd.Source != "" {
		source = cmd.Source
	}
	return h.repo.UpsertBySessionID(ctx, &trace.Trace{
		ID:         t.ID,
		Agent:      constant.TraceAgentCodex,
		SessionID:  cmd.SessionID,
		APIKeyName: t.APIKeyName,
		UserID:     t.UserID,
		Model:      modelName,
		CWD:        cwd,
		Source:     source,
		Status:     t.Status,
		Metadata:   t.Metadata,
	})
}

func insertRecords(
	ctx context.Context,
	repo trace.TraceRepository,
	traceID uint,
	sessionID string,
	records []port.ReportTraceRecord,
	requireDedupKey bool,
) ([]port.ReportTraceRecordResult, bool) {
	results := make([]port.ReportTraceRecordResult, 0, len(records))
	isComplete := false
	for _, record := range records {
		result := port.ReportTraceRecordResult{DedupKey: record.DedupKey}
		if !validRecord(record, requireDedupKey) {
			result.Status = constant.TraceRecordStatusRejected
			result.Message = constant.TraceRecordMessageInvalid
			results = append(results, result)
			continue
		}

		inserted, err := repo.InsertEvent(ctx, &trace.TraceEvent{
			TraceID:        traceID,
			SessionID:      sessionID,
			Source:         record.Source,
			RecordType:     record.RecordType,
			Event:          record.Event,
			TurnID:         record.TurnID,
			CallID:         record.CallID,
			TranscriptLine: record.TranscriptLine,
			ClientSequence: record.ClientSequence,
			DedupKey:       record.DedupKey,
			Payload:        record.Payload,
		})
		switch {
		case err != nil:
			result.Status = constant.TraceRecordStatusRejected
			result.Message = constant.TraceRecordMessageStorageFailed
		case !inserted:
			result.Status = constant.TraceRecordStatusDuplicate
		default:
			result.Status = constant.TraceRecordStatusAccepted
		}
		results = append(results, result)
		if result.Status != constant.TraceRecordStatusRejected && completeEvent(record.Event) {
			isComplete = true
		}
	}
	return results, isComplete
}

func validRecord(record port.ReportTraceRecord, requireDedupKey bool) bool {
	isSourceValid := record.Source == constant.TraceRecordSourceHook ||
		record.Source == constant.TraceRecordSourceRollout
	if !isSourceValid || record.RecordType == "" || len(record.Payload) == 0 {
		return false
	}
	return !requireDedupKey || record.DedupKey != ""
}

func completeEvent(event string) bool {
	return event == constant.TraceEventStop || event == constant.TraceEventTaskComplete
}
