package handler

import (
	"bufio"
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	datasetport "github.com/hcd233/aris-proxy-api/internal/application/dataset/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// DatasetHandler 数据集导出处理器接口
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type DatasetHandler interface {
	HandlePreview(ctx context.Context, req *dto.DatasetPreviewReq) (*dto.HTTPResponse[*dto.DatasetPreviewRsp], error)
	HandleExport(ctx context.Context, req *dto.DatasetExportReq) (*huma.StreamResponse, error)
}

// DatasetDependencies 数据集处理器依赖
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type DatasetDependencies struct {
	Preview datasetport.PreviewDatasetHandler
	Export  datasetport.ExportDatasetHandler
}

type datasetHandler struct {
	preview datasetport.PreviewDatasetHandler
	export  datasetport.ExportDatasetHandler
}

// NewDatasetHandler 构造数据集处理器
//
//	@param deps DatasetDependencies
//	@return DatasetHandler
//	@author centonhuang
//	@update 2026-07-03 10:00:00
func NewDatasetHandler(deps DatasetDependencies) DatasetHandler {
	return &datasetHandler{preview: deps.Preview, export: deps.Export}
}

func (h *datasetHandler) HandlePreview(ctx context.Context, req *dto.DatasetPreviewReq) (*dto.HTTPResponse[*dto.DatasetPreviewRsp], error) {
	rsp := &dto.DatasetPreviewRsp{}

	permission := util.CtxValuePermission(ctx)
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	result, err := h.preview.Handle(ctx, datasetport.ExportParams{
		Permission: permission,
		UserID:     userID,
		MinScore:   req.MinScore,
		Models:     req.Models,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[DatasetHandler] Preview failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.TotalSessions = result.TotalSessions
	rsp.ScoreDistribution = result.ScoreDistribution
	rsp.ModelDistribution = result.ModelDistribution
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *datasetHandler) HandleExport(ctx context.Context, req *dto.DatasetExportReq) (*huma.StreamResponse, error) {
	permission := util.CtxValuePermission(ctx)
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	params := datasetport.ExportParams{
		Permission: permission,
		UserID:     userID,
		MinScore:   req.MinScore,
		Models:     req.Models,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
	}

	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			fiberCtx := humafiber.Unwrap(humaCtx)
			fiberCtx.Set(constant.HTTPHeaderContentType, constant.DatasetExportContentType)
			fiberCtx.Set(constant.HTTPHeaderContentDisposition, constant.DatasetExportContentDisposition)
			fiberCtx.Set(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fiberCtx.Status(http.StatusOK)

			_ = fiberCtx.SendStreamWriter(func(w *bufio.Writer) { //nolint:errcheck // stream write errors propagate via Fiber
				if err := h.export.Handle(ctx, params, w); err != nil {
					logger.WithCtx(ctx).Error("[DatasetHandler] Export stream error", zap.Error(err))
				}
			})
		},
	}, nil
}
