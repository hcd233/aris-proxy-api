// Package command trace 写侧 usecase
package command

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
	"github.com/hcd233/aris-proxy-api/internal/logger"
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
	agent := cmd.Agent
	if agent == "" {
		agent = constant.TraceAgentCodex
	}
	if agent != constant.TraceAgentCodex && agent != constant.TraceAgentClaude {
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
	results := insertRecords(ctx, h.repo, t.ID, cmd.SessionID, records)
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
		parentTraceID := resolveParentTraceID(ctx, h.repo, cmd.ParentSessionID, cmd.SessionID, cmd.APIKeyName)
		metadata := resolveSubagentAttrs(cmd)
		return h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent:         agent,
			SessionID:     cmd.SessionID,
			ParentTraceID: parentTraceID,
			APIKeyName:    cmd.APIKeyName,
			Model:         cmd.Model,
			CWD:           cmd.CWD,
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
	return h.repo.UpsertBySessionID(ctx, &trace.Trace{
		ID:            existing.ID,
		Agent:         agent,
		SessionID:     cmd.SessionID,
		ParentTraceID: existing.ParentTraceID,
		APIKeyName:    existing.APIKeyName,
		Model:         modelName,
		CWD:           cwd,
		Metadata:      existing.Metadata,
	})
}

// resolveParentTraceID 按父 session 解析父 trace id；无父、父不存在或租户不一致时返回 0。
func resolveParentTraceID(ctx context.Context, repo trace.TraceRepository, parentSessionID, sessionID, apiKeyName string) uint {
	if parentSessionID == "" || parentSessionID == sessionID {
		return 0
	}
	parent, err := repo.FindBySessionID(ctx, parentSessionID)
	if err != nil || parent == nil {
		return 0
	}
	if apiKeyName != "" && parent.APIKeyName != apiKeyName {
		return 0 // 跨租户 session 不应建立父/子关联
	}
	return parent.ID
}

// resolveSubagentAttrs 子代理批次返回 metadata；主批次返回空 map。
func resolveSubagentAttrs(cmd port.ReportTraceEventCommand) map[string]string {
	metadata := map[string]string{}
	if cmd.ParentSessionID == "" {
		return metadata
	}
	if cmd.AgentID != "" {
		metadata[constant.TraceMetadataAgentID] = cmd.AgentID
	}
	if cmd.AgentType != "" {
		metadata[constant.TraceMetadataAgentType] = cmd.AgentType
	}
	return metadata
}

func insertRecords(
	ctx context.Context,
	repo trace.TraceRepository,
	traceID uint,
	sessionID string,
	records []port.ReportTraceRecord,
) []port.ReportTraceRecordResult {
	results := make([]port.ReportTraceRecordResult, 0, len(records))
	for _, record := range records {
		result := port.ReportTraceRecordResult{DedupKey: record.DedupKey}
		if !validRecord(record) {
			result.Status = constant.TraceRecordStatusRejected
			result.Message = constant.TraceRecordMessageInvalid
			results = append(results, result)
			continue
		}
		if record.RecordType == constant.TraceRolloutTypeUnknown {
			// 未知记录类型：打 warning 便于发现 codex 新类型，不入库
			logger.WithCtx(ctx).Warn("[Trace] Unknown record dropped",
				zap.String("sessionID", sessionID),
				zap.String("event", record.Event),
				zap.Int("payloadBytes", len(record.Payload)),
			)
			result.Status = constant.TraceRecordStatusRejected
			result.Message = constant.TraceRecordMessageUnknown
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
	}
	return results
}

func validRecord(record port.ReportTraceRecord) bool {
	isSourceValid := record.Source == constant.TraceRecordSourceHook ||
		record.Source == constant.TraceRecordSourceRollout
	if !isSourceValid || record.RecordType == "" || len(record.Payload) == 0 {
		return false
	}
	return record.DedupKey != ""
}
