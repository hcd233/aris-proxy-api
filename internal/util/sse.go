package util

import (
	"bufio"
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// WrapErrorSSE 包装错误响应
//
//	@param ctx
//	@param err
//	@return rsp
//	@author centonhuang
//	@update 2025-11-11 17:46:36
func WrapErrorSSE(ctx context.Context, err *model.Error) (rsp *huma.StreamResponse) {
	return &huma.StreamResponse{
		Body: func(hCtx huma.Context) {
			fCtx := humafiber.Unwrap(hCtx)
			fCtx.Set("Content-Type", "text/event-stream")
			fCtx.Set("Cache-Control", "no-cache")
			fCtx.Set("Connection", "keep-alive")
			fCtx.Set("Transfer-Encoding", "chunked")

			fCtx.Response().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
				writeSSEErrorResponse(ctx, w, err)
			}))
		},
	}
}

func writeSSEErrorResponse(ctx context.Context, w *bufio.Writer, err *model.Error) {
	logger := logger.WithCtx(ctx)
	rsp := &dto.SSEResponse{
		DataType: enum.SSEDataTypeError,
		Status:   enum.SSEStatusError,
		Data:     &dto.CommonRsp{Error: err},
	}
	fmt.Fprintf(w, "data: %s\n\n", lo.Must1(sonic.Marshal(rsp)))
	if err := w.Flush(); err != nil {
		logger.Error("[WriteErrorResponse] flush error", zap.Error(err))
	}
}
