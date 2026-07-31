// Package command trace 写侧 usecase
package command

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

// traceDoneEvents 按 agent 注册的完成事件集：命中即把 trace 置为 done。
// 新 agent 接入在此登记。
var traceDoneEvents = map[string][]string{
	constant.TraceAgentCodex:  {constant.TraceEventStop, constant.TraceEventTaskComplete},
	constant.TraceAgentClaude: {constant.TraceEventSessionEnd},
}

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
	agent := cmd.Agent
	if agent == "" {
		agent = constant.TraceAgentCodex
	}
	doneEvents, ok := traceDoneEvents[agent]
	if !ok {
		return nil, ierr.New(ierr.ErrValidation, "unknown trace agent")
	}
	if len(cmd.Records) == 0 {
		return nil, ierr.New(ierr.ErrValidation, "empty trace records")
	}

	records := normalizeRecords(cmd)
	existing, err := h.repo.FindBySessionIDIncludingDeleted(ctx, cmd.SessionID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.DeletedAt != 0 {
		return lo.Map(records, func(r port.ReportTraceRecord, _ int) port.ReportTraceRecordResult {
			return port.ReportTraceRecordResult{
				DedupKey: r.DedupKey,
				Status:   constant.TraceRecordStatusRejected,
				Message:  constant.TraceRecordMessageTraceDeleted,
			}
		}), nil
	}
	t, err := h.ensureTrace(ctx, cmd, agent, existing)
	if err != nil {
		return nil, err
	}
	results, isComplete := insertRecords(
		ctx,
		h.repo,
		t.ID,
		cmd.SessionID,
		records,
		doneEvents,
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
	agent string,
	existing *trace.Trace,
) (*trace.Trace, error) {
	if existing == nil {
		parentTraceID := resolveParentTraceID(ctx, h.repo, cmd.ParentSessionID, cmd.SessionID)
		source, metadata := resolveSubagentAttrs(cmd)
		return h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent:         agent,
			SessionID:     cmd.SessionID,
			ParentTraceID: parentTraceID,
			APIKeyName:    cmd.APIKeyName,
			UserID:        cmd.UserID,
			Model:         cmd.Model,
			CWD:           cmd.CWD,
			Source:        source,
			Status:        constant.TraceStatusActive,
			Metadata:      metadata,
		})
	}
	if existing.Agent != "" && existing.Agent != agent {
		return nil, ierr.New(ierr.ErrValidation, "trace agent mismatch for session")
	}

	modelName := existing.Model
	if cmd.Model != "" {
		modelName = cmd.Model
	}
	cwd := existing.CWD
	if cmd.CWD != "" {
		cwd = cmd.CWD
	}
	source := existing.Source
	if cmd.Source != "" {
		source = cmd.Source
	}
	return h.repo.UpsertBySessionID(ctx, &trace.Trace{
		ID:            existing.ID,
		Agent:         agent,
		SessionID:     cmd.SessionID,
		ParentTraceID: existing.ParentTraceID,
		APIKeyName:    existing.APIKeyName,
		UserID:        existing.UserID,
		Model:         modelName,
		CWD:           cwd,
		Source:        source,
		Status:        existing.Status,
		Metadata:      existing.Metadata,
	})
}

// resolveParentTraceID 按父 session 解析父 trace id；无父或父不存在返回 0。
func resolveParentTraceID(ctx context.Context, repo trace.TraceRepository, parentSessionID, sessionID string) uint {
	if parentSessionID == "" || parentSessionID == sessionID {
		return 0
	}
	parent, err := repo.FindBySessionID(ctx, parentSessionID)
	if err != nil || parent == nil {
		return 0
	}
	return parent.ID
}

// resolveSubagentAttrs 子代理批次返回 (source, metadata)；主批次保持 cmd 原值。
func resolveSubagentAttrs(cmd port.ReportTraceEventCommand) (source string, metadata map[string]string) {
	metadata = map[string]string{}
	if cmd.ParentSessionID == "" {
		return cmd.Source, metadata
	}
	source = constant.TraceSourceSubagent
	if cmd.AgentID != "" {
		metadata[constant.TraceMetadataAgentID] = cmd.AgentID
	}
	if cmd.AgentType != "" {
		metadata[constant.TraceMetadataAgentType] = cmd.AgentType
	}
	return source, metadata
}

func insertRecords(
	ctx context.Context,
	repo trace.TraceRepository,
	traceID uint,
	sessionID string,
	records []port.ReportTraceRecord,
	doneEvents []string,
) ([]port.ReportTraceRecordResult, bool) {
	results := make([]port.ReportTraceRecordResult, 0, len(records))
	isComplete := false
	for _, record := range records {
		result := port.ReportTraceRecordResult{DedupKey: record.DedupKey}
		if !validRecord(record) {
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
		if result.Status != constant.TraceRecordStatusRejected && lo.Contains(doneEvents, record.Event) {
			isComplete = true
		}
	}
	return results, isComplete
}

func validRecord(record port.ReportTraceRecord) bool {
	isSourceValid := record.Source == constant.TraceRecordSourceHook ||
		record.Source == constant.TraceRecordSourceRollout
	if !isSourceValid || record.RecordType == "" || len(record.Payload) == 0 {
		return false
	}
	return record.DedupKey != ""
}
