package util

import (
	"bufio"
	"context"
	"fmt"
	"net/http"

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
			fCtx.Set("X-Accel-Buffering", "no")

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
		logger.Error("[WriteErrorResponse] Flush error", zap.Error(err))
	}
}

// SendOpenAIModelNotFoundError 发送OpenAI模型不存在错误
//
//	@return rsp
//	@author centonhuang
//	@update 2026-03-06 15:58:35
func SendOpenAIModelNotFoundError(model string) (rsp *huma.StreamResponse) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			humaCtx.SetStatus(http.StatusNotFound)
			humaCtx.SetHeader("Content-Type", "application/json")
			humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.OpenAIError{
				Message: fmt.Sprintf("The model `%s` does not exist", model),
				Type:    "invalid_request_error",
				Code:    "model_not_found",
			})))
		},
	}
}

// SendOpenAIInternalError 发送OpenAI内部错误
//
//	@return rsp
//	@author centonhuang
//	@update 2026-03-06 16:00:05
func SendOpenAIInternalError() (rsp *huma.StreamResponse) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			humaCtx.SetStatus(http.StatusInternalServerError)
			humaCtx.SetHeader("Content-Type", "application/json")
			humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.OpenAIError{
				Message: "Internal error",
				Type:    "server_error",
				Code:    "internal_error",
			})))
		},
	}
}
