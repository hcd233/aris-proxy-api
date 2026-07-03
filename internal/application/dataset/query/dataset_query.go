// Package query implements dataset export query handlers.
package query

import (
	"bufio"
	"context"
	"io"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/dataset/converter"
	datasetport "github.com/hcd233/aris-proxy-api/internal/application/dataset/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
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

func (h *exportDatasetHandler) Handle(ctx context.Context, p datasetport.ExportParams, w io.Writer) error {
	log := logger.WithCtx(ctx)

	f, err := h.buildFilter(ctx, p)
	if err != nil {
		return err
	}

	rows, err := h.readRepo.ListSessionsForExport(ctx, *f)
	if err != nil {
		log.Error("[DatasetExport] Failed to list sessions for export", zap.Error(err))
		return err
	}

	if len(rows) == 0 {
		return ierr.New(ierr.ErrDataNotExists, "no sessions match the filter")
	}

	allMsgIDs := lo.Uniq(lo.FlatMap(rows, func(r *session.ExportSessionRow, _ int) []uint { return r.MessageIDs }))
	allToolIDs := lo.Uniq(lo.FlatMap(rows, func(r *session.ExportSessionRow, _ int) []uint { return r.ToolIDs }))

	messages, err := h.readRepo.FindMessagesByIDs(ctx, allMsgIDs)
	if err != nil {
		log.Error("[DatasetExport] Failed to batch get messages", zap.Error(err))
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
			return toolErr
		}
		tools = toolRecords
	}
	toolMap := lo.SliceToMap(tools, func(t *session.ToolDetailProjection) (uint, *session.ToolDetailProjection) {
		return t.ID, t
	})

	bw := bufio.NewWriter(w)
	defer func() { _ = bw.Flush() }() //nolint:errcheck // best-effort flush on stream end

	for _, row := range rows {
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

		if _, writeErr := bw.Write(line); writeErr != nil {
			return writeErr
		}
		if _, writeErr := bw.WriteString("\n"); writeErr != nil {
			return writeErr
		}
	}

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
