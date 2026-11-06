// Package query implements dataset export query handlers.
package query

import (
	"bufio"
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-mass/internal/application/dataset/converter"
	datasetport "github.com/hcd233/aris-mass/internal/application/dataset/port"
	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/domain/apikey"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/domain/session"
	"github.com/hcd233/aris-mass/internal/logger"
)

// ownerIDLookup 归属 API Key ID 查询（由 apikey 仓储实现）。
//
// 用 ID 而非名称：名称可跨用户重复，按名称过滤会让同名 Key 的持有者
// 越权导出他人会话的完整内容（本域是越权影响最高的路径）。
type ownerIDLookup interface {
	LookupIDsByUserID(ctx context.Context, userID uint) ([]uint, error)
}

type previewDatasetHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerIDLookup
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
	apiKeyRepo ownerIDLookup
	images     blob.ImageReader
}

// NewExportDatasetHandler 构造流式导出处理器
//
//	@return datasetport.ExportDatasetHandler
//	@author centonhuang
//	@update 2026-07-03 10:00:00
func NewExportDatasetHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository, images blob.ImageReader) datasetport.ExportDatasetHandler {
	return &exportDatasetHandler{readRepo: readRepo, apiKeyRepo: apiKeyRepo, images: images}
}

func (h *exportDatasetHandler) Handle(ctx context.Context, p datasetport.ExportParams) (*datasetport.ExportStream, error) {
	f, err := h.buildFilter(ctx, p)
	if err != nil {
		return nil, err
	}
	rows, err := h.readRepo.ListSessionsForExport(ctx, *f)
	if err != nil {
		logger.WithCtx(ctx).Error("[DatasetExport] Failed to list sessions for export", zap.Error(err))
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ierr.New(ierr.ErrDataNotExists, constant.DatasetExportNoMatchError)
	}
	return &datasetport.ExportStream{
		Total: len(rows),
		Write: func(w *bufio.Writer) error { return h.write(ctx, rows, w) },
	}, nil
}

// write 按批读取会话的 message/tool，逐行写出 ShareGPT JSON
//
//	每批只把该批会话的 message 读进内存，写完即 Flush，内存占用与总导出量无关。
func (h *exportDatasetHandler) write(ctx context.Context, rows []*session.ExportSessionRow, w *bufio.Writer) error {
	images := NewExportImageResolver(ctx, h.images)
	for _, chunk := range lo.Chunk(rows, constant.DatasetExportBatchSessions) {
		if err := ctx.Err(); err != nil {
			return err
		}
		msgMap, toolMap, err := h.loadChunk(ctx, chunk)
		if err != nil {
			return err
		}
		for _, row := range chunk {
			images.BeginSession()
			line, err := converter.MarshalJSONLine(buildConversation(row, msgMap, toolMap, images.Resolve))
			if err != nil {
				return ierr.Wrapf(ierr.ErrInternal, err, "marshal session %d", row.ID)
			}
			if _, err := w.Write(append(line, '\n')); err != nil {
				return ierr.Wrap(ierr.ErrInternal, err, "write export line")
			}
		}
		if err := w.Flush(); err != nil {
			return ierr.Wrap(ierr.ErrInternal, err, "flush export")
		}
	}
	if failed, skipped := images.Failed(), images.Skipped(); failed > 0 || skipped > 0 {
		logger.WithCtx(ctx).Warn("[DatasetExport] Some images were not exported", zap.Int("failed", failed), zap.Int("skippedOverBudget", skipped))
	}
	return nil
}

func (h *exportDatasetHandler) buildFilter(ctx context.Context, p datasetport.ExportParams) (*session.ExportFilter, error) {
	f := &session.ExportFilter{
		MinScore:  p.MinScore,
		ModelIDs:  p.ModelIDs,
		StartTime: p.StartTime,
		EndTime:   p.EndTime,
	}

	if p.Permission != enum.PermissionAdmin {
		ownerIDs, err := h.apiKeyRepo.LookupIDsByUserID(ctx, p.UserID)
		if err != nil {
			logger.WithCtx(ctx).Error("[DatasetExport] Failed to lookup owner ids", zap.Error(err), zap.Uint("userID", p.UserID))
			return nil, err
		}
		f.OwnerIDs = ownerIDs
	}

	return f, nil
}

// loadChunk 批量读取一批会话引用的 message 与 tool
func (h *exportDatasetHandler) loadChunk(ctx context.Context, rows []*session.ExportSessionRow) (msgMap map[uint]*session.MessageDetailProjection, toolMap map[uint]*session.ToolDetailProjection, err error) {
	log := logger.WithCtx(ctx)
	msgIDs := lo.Uniq(lo.FlatMap(rows, func(r *session.ExportSessionRow, _ int) []uint { return r.MessageIDs }))
	messages, err := h.readRepo.FindMessagesByIDs(ctx, msgIDs)
	if err != nil {
		log.Error("[DatasetExport] Failed to batch get messages", zap.Error(err))
		return nil, nil, err
	}
	msgMap = lo.SliceToMap(messages, func(m *session.MessageDetailProjection) (uint, *session.MessageDetailProjection) {
		return m.ID, m
	})

	toolIDs := lo.Uniq(lo.FlatMap(rows, func(r *session.ExportSessionRow, _ int) []uint { return r.ToolIDs }))
	var tools []*session.ToolDetailProjection
	if len(toolIDs) > 0 {
		if tools, err = h.readRepo.FindToolsByIDs(ctx, toolIDs); err != nil {
			log.Error("[DatasetExport] Failed to batch get tools", zap.Error(err))
			return nil, nil, err
		}
	}
	toolMap = lo.SliceToMap(tools, func(t *session.ToolDetailProjection) (uint, *session.ToolDetailProjection) {
		return t.ID, t
	})
	return msgMap, toolMap, nil
}

func (h *previewDatasetHandler) buildFilter(ctx context.Context, p datasetport.ExportParams) (*session.ExportFilter, error) {
	f := &session.ExportFilter{
		MinScore:  p.MinScore,
		ModelIDs:  p.ModelIDs,
		StartTime: p.StartTime,
		EndTime:   p.EndTime,
	}

	if p.Permission != enum.PermissionAdmin {
		ownerIDs, err := h.apiKeyRepo.LookupIDsByUserID(ctx, p.UserID)
		if err != nil {
			logger.WithCtx(ctx).Error("[DatasetPreview] Failed to lookup owner ids", zap.Error(err), zap.Uint("userID", p.UserID))
			return nil, err
		}
		f.OwnerIDs = ownerIDs
	}

	return f, nil
}

func buildConversation(
	row *session.ExportSessionRow,
	msgMap map[uint]*session.MessageDetailProjection,
	toolMap map[uint]*session.ToolDetailProjection,
	resolve func(part *vo.UnifiedContentPart) (string, bool),
) *converter.ShareGPTConversation {
	msgs := lo.FilterMap(row.MessageIDs, func(id uint, _ int) (*session.MessageDetailProjection, bool) {
		m, ok := msgMap[id]
		return m, ok
	})

	tools := lo.FilterMap(row.ToolIDs, func(id uint, _ int) (*session.ToolDetailProjection, bool) {
		t, ok := toolMap[id]
		return t, ok
	})

	return converter.ConvertSessionWithImages(msgs, tools, resolve)
}

type previewFormatDatasetHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerIDLookup
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

	conv := buildConversation(row, msgMap, toolMap, PreviewImageResolver)
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
		ModelIDs:  p.ModelIDs,
		StartTime: p.StartTime,
		EndTime:   p.EndTime,
	}

	if p.Permission != enum.PermissionAdmin {
		ownerIDs, err := h.apiKeyRepo.LookupIDsByUserID(ctx, p.UserID)
		if err != nil {
			logger.WithCtx(ctx).Error("[DatasetFormatPreview] Failed to lookup owner ids", zap.Error(err), zap.Uint("userID", p.UserID))
			return nil, err
		}
		f.OwnerIDs = ownerIDs
	}

	return f, nil
}
