// Package query implements dataset export query handlers.
package query

import (
	"bufio"
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/dataset/converter"
	datasetport "github.com/hcd233/aris-proxy-api/internal/application/dataset/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type ownerNameLookup interface {
	LookupOwnerNamesByUserID(ctx context.Context, userID uint) ([]string, error)
}

type previewDatasetHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerNameLookup
}

// NewPreviewDatasetHandler 构造统计预览处理器
//
//	@return datasetport.PreviewDatasetHandler
//	@author centonhuang
//	@update 2026-07-03 10:00:00
func NewPreviewDatasetHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) datasetport.PreviewDatasetHandler {
	return &previewDatasetHandler{readRepo: readRepo, apiKeyRepo: apiKeyRepo}
}

func (h *previewDatasetHandler) Handle(ctx context.Context, p datasetport.ExportParams) (*datasetport.PreviewResult, error) {
	f, err := h.buildFilter(ctx, p)
	if err != nil {
		return nil, err
	}

	preview, err := h.readRepo.PreviewExport(ctx, *f)
	if err != nil {
		logger.WithCtx(ctx).Error("[DatasetPreview] Failed to preview export", zap.Error(err))
		return nil, err
	}

	return &datasetport.PreviewResult{
		TotalSessions:     preview.TotalSessions,
		ScoreDistribution: preview.ScoreDistribution,
		ModelDistribution: preview.ModelDistribution,
	}, nil
}

type exportDatasetHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerNameLookup
}

// NewExportDatasetHandler 构造流式导出处理器
//
//	@return datasetport.ExportDatasetHandler
//	@author centonhuang
//	@update 2026-07-03 10:00:00
func NewExportDatasetHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) datasetport.ExportDatasetHandler {
	return &exportDatasetHandler{readRepo: readRepo, apiKeyRepo: apiKeyRepo}
}

func (h *exportDatasetHandler) Handle(ctx context.Context, p datasetport.ExportParams, w *bufio.Writer) error {
	log := logger.WithCtx(ctx)

	f, err := h.buildFilter(ctx, p)
	if err != nil {
		h.writeSSEError(w, err.Error())
		return err
	}

	rows, err := h.readRepo.ListSessionsForExport(ctx, *f)
	if err != nil {
		log.Error("[DatasetExport] Failed to list sessions for export", zap.Error(err))
		h.writeSSEError(w, err.Error())
		return err
	}

	total := len(rows)
	if total == 0 {
		h.writeSSEError(w, constant.DatasetExportNoMatchError)
		return ierr.New(ierr.ErrDataNotExists, constant.DatasetExportNoMatchError)
	}

	allMsgIDs := lo.Uniq(lo.FlatMap(rows, func(r *session.ExportSessionRow, _ int) []uint { return r.MessageIDs }))
	allToolIDs := lo.Uniq(lo.FlatMap(rows, func(r *session.ExportSessionRow, _ int) []uint { return r.ToolIDs }))

	messages, err := h.readRepo.FindMessagesByIDs(ctx, allMsgIDs)
	if err != nil {
		log.Error("[DatasetExport] Failed to batch get messages", zap.Error(err))
		h.writeSSEError(w, err.Error())
		return err
	}
	msgMap := lo.SliceToMap(messages, func(m *session.MessageDetailProjection) (uint, *session.MessageDetailProjection) {
		return m.ID, m
	})

	var tools []*session.ToolDetailProjection
	if len(allToolIDs) > 0 {
		toolRecords, toolErr := h.readRepo.FindToolsByIDs(ctx, allToolIDs)
		if toolErr != nil {
			log.Error("[DatasetExport] Failed to batch get tools", zap.Error(err))
			h.writeSSEError(w, toolErr.Error())
			return toolErr
		}
		tools = toolRecords
	}
	toolMap := lo.SliceToMap(tools, func(t *session.ToolDetailProjection) (uint, *session.ToolDetailProjection) {
		return t.ID, t
	})

	h.writeSSEStart(w, total)

	for i, row := range rows {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conv := buildConversation(row, msgMap, toolMap)
		line, marshalErr := converter.MarshalJSONLine(conv)
		if marshalErr != nil {
			log.Error("[DatasetExport] Failed to marshal conversation", zap.Error(marshalErr), zap.Uint("sessionID", row.ID))
			continue
		}

		current := i + 1
		progress := current * constant.DatasetExportProgressFull / total
		h.writeSSEData(w, current, total, progress, string(line))
	}

	h.writeSSEDone(w, total)
	return nil
}

func (h *exportDatasetHandler) buildFilter(ctx context.Context, p datasetport.ExportParams) (*session.ExportFilter, error) {
	f := &session.ExportFilter{
		MinScore:  p.MinScore,
		Models:    p.Models,
		StartTime: p.StartTime,
		EndTime:   p.EndTime,
	}

	if p.Permission != enum.PermissionAdmin {
		ownerNames, err := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, p.UserID)
		if err != nil {
			logger.WithCtx(ctx).Error("[DatasetExport] Failed to lookup owner names", zap.Error(err), zap.Uint("userID", p.UserID))
			return nil, err
		}
		f.OwnerNames = ownerNames
	}

	return f, nil
}

func (h *exportDatasetHandler) writeSSEStart(w *bufio.Writer, total int) {
	payload := lo.Must1(sonic.Marshal(&dto.DatasetExportSSEStart{TotalSessions: total}))
	h.writeSSEEvent(w, constant.DatasetExportEventStart, payload)
}

func (h *exportDatasetHandler) writeSSEData(w *bufio.Writer, current, total, progress int, jsonLine string) {
	payload := lo.Must1(sonic.Marshal(&dto.DatasetExportSSEData{
		Current:  current,
		Total:    total,
		Progress: progress,
		JSON:     jsonLine,
	}))
	h.writeSSEEvent(w, constant.DatasetExportEventData, payload)
}

func (h *exportDatasetHandler) writeSSEDone(w *bufio.Writer, total int) {
	payload := lo.Must1(sonic.Marshal(&dto.DatasetExportSSEStart{TotalSessions: total}))
	h.writeSSEEvent(w, constant.DatasetExportEventDone, payload)
}

func (h *exportDatasetHandler) writeSSEError(w *bufio.Writer, message string) {
	payload := lo.Must1(sonic.Marshal(&dto.DatasetExportSSEError{Message: message}))
	h.writeSSEEvent(w, constant.DatasetExportEventError, payload)
}

func (h *exportDatasetHandler) writeSSEEvent(w *bufio.Writer, event string, payload []byte) {
	_, _ = fmt.Fprintf(w, constant.SSEEventFrameTemplate, event, payload) //nolint:errcheck // best-effort write
	_ = w.Flush()                                                         //nolint:errcheck // best-effort flush
}

func (h *previewDatasetHandler) buildFilter(ctx context.Context, p datasetport.ExportParams) (*session.ExportFilter, error) {
	f := &session.ExportFilter{
		MinScore:  p.MinScore,
		Models:    p.Models,
		StartTime: p.StartTime,
		EndTime:   p.EndTime,
	}

	if p.Permission != enum.PermissionAdmin {
		ownerNames, err := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, p.UserID)
		if err != nil {
			logger.WithCtx(ctx).Error("[DatasetPreview] Failed to lookup owner names", zap.Error(err), zap.Uint("userID", p.UserID))
			return nil, err
		}
		f.OwnerNames = ownerNames
	}

	return f, nil
}

func buildConversation(
	row *session.ExportSessionRow,
	msgMap map[uint]*session.MessageDetailProjection,
	toolMap map[uint]*session.ToolDetailProjection,
) *converter.ShareGPTConversation {
	msgs := lo.FilterMap(row.MessageIDs, func(id uint, _ int) (*session.MessageDetailProjection, bool) {
		m, ok := msgMap[id]
		return m, ok
	})

	tools := lo.FilterMap(row.ToolIDs, func(id uint, _ int) (*session.ToolDetailProjection, bool) {
		t, ok := toolMap[id]
		return t, ok
	})

	return converter.ConvertSession(msgs, tools)
}

type previewFormatDatasetHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerNameLookup
}

// NewPreviewFormatDatasetHandler 构造单条会话格式预览处理器
//
//	@return datasetport.PreviewFormatDatasetHandler
//	@author centonhuang
//	@update 2026-07-06 10:00:00
func NewPreviewFormatDatasetHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) datasetport.PreviewFormatDatasetHandler {
	return &previewFormatDatasetHandler{readRepo: readRepo, apiKeyRepo: apiKeyRepo}
}

func (h *previewFormatDatasetHandler) Handle(ctx context.Context, p datasetport.ExportParams, offset int) (*datasetport.FormatPreviewResult, error) {
	log := logger.WithCtx(ctx)

	f, err := h.buildFilter(ctx, p)
	if err != nil {
		return nil, err
	}

	if offset < 0 {
		offset = 0
	}

	rows, err := h.readRepo.ListSessionsForExport(ctx, *f)
	if err != nil {
		log.Error("[DatasetFormatPreview] Failed to list sessions", zap.Error(err))
		return nil, err
	}

	total := len(rows)
	if total == 0 {
		return nil, ierr.New(ierr.ErrDataNotExists, constant.DatasetExportNoMatchError)
	}

	if offset >= total {
		offset = total - 1
	}

	row := rows[offset]

	msgs, err := h.readRepo.FindMessagesByIDs(ctx, row.MessageIDs)
	if err != nil {
		log.Error("[DatasetFormatPreview] Failed to get messages", zap.Error(err))
		return nil, err
	}
	msgMap := lo.SliceToMap(msgs, func(m *session.MessageDetailProjection) (uint, *session.MessageDetailProjection) {
		return m.ID, m
	})

	var toolMap map[uint]*session.ToolDetailProjection
	if len(row.ToolIDs) > 0 {
		tools, toolErr := h.readRepo.FindToolsByIDs(ctx, row.ToolIDs)
		if toolErr != nil {
			log.Error("[DatasetFormatPreview] Failed to get tools", zap.Error(toolErr))
			return nil, toolErr
		}
		toolMap = lo.SliceToMap(tools, func(t *session.ToolDetailProjection) (uint, *session.ToolDetailProjection) {
			return t.ID, t
		})
	}

	conv := buildConversation(row, msgMap, toolMap)
	line, marshalErr := converter.MarshalJSONLine(conv)
	if marshalErr != nil {
		log.Error("[DatasetFormatPreview] Failed to marshal conversation", zap.Error(marshalErr))
		return nil, marshalErr
	}

	return &datasetport.FormatPreviewResult{
		SessionID:    row.ID,
		Offset:       offset,
		TotalCount:   total,
		ShareGPTJSON: string(line),
	}, nil
}

func (h *previewFormatDatasetHandler) buildFilter(ctx context.Context, p datasetport.ExportParams) (*session.ExportFilter, error) {
	f := &session.ExportFilter{
		MinScore:  p.MinScore,
		Models:    p.Models,
		StartTime: p.StartTime,
		EndTime:   p.EndTime,
	}

	if p.Permission != enum.PermissionAdmin {
		ownerNames, err := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, p.UserID)
		if err != nil {
			logger.WithCtx(ctx).Error("[DatasetFormatPreview] Failed to lookup owner names", zap.Error(err), zap.Uint("userID", p.UserID))
			return nil, err
		}
		f.OwnerNames = ownerNames
	}

	return f, nil
}
