package util

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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
			fCtx.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeEventStream)
			fCtx.Set(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fCtx.Set(constant.HTTPHeaderConnection, constant.HTTPConnectionKeepAlive)
			fCtx.Set(constant.HTTPHeaderTransferEncoding, constant.HTTPTransferEncodingChunked)
			fCtx.Set(constant.HTTPHeaderXAccelBuffering, constant.HTTPHeaderDisabled)

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
	if _, writeErr := fmt.Fprintf(w, constant.SSEDataFrameTemplate, lo.Must1(sonic.Marshal(rsp))); writeErr != nil {
		logger.Debug("[WriteErrorResponse] Failed to write sse data frame", zap.Error(writeErr))
	}
	if err := w.Flush(); err != nil {
		logger.Error("[WriteErrorResponse] Flush error", zap.Error(err))
	}
}

// WriteAnthropicMessageStop 向客户端写入 Anthropic 协议的 message_stop 结束帧。
//
// 两条转发路径（forwardNative / forwardViaOpenAI）都通过此函数发送结束帧，
// 保证 event 类型和 data payload 一致（参见提交 184dcf9 的回归修复）。
// 返回 flush 错误而不 panic，调用方可按需处理（通常忽略即可）。
//
//	@param w *bufio.Writer
//	@return error
//	@author centonhuang
//	@update 2026-04-20 11:00:00
func WriteAnthropicMessageStop(w *bufio.Writer) error {
	if _, err := w.WriteString(constant.AnthropicMessageStopSSEFrame); err != nil {
		return err
	}
	return w.Flush()
}

// WriteUpstreamSSEError 在 SSE 流中写入上游错误。
// 当上游在流式请求开始后（HTTP 200 已发送）返回错误时，本函数将上游错误体
// 以 SSE data 帧的形式写入客户端，避免客户端收到空的截断流。
//
//	@param log *zap.Logger
//	@param w *bufio.Writer
//	@param err error
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func WriteUpstreamSSEError(log *zap.Logger, w *bufio.Writer, err error) {
	var upstreamErr *model.UpstreamError
	if errors.As(err, &upstreamErr) {
		if upstreamErr.Body != "" {
			if _, writeErr := fmt.Fprintf(w, constant.SSEDataFrameTemplate, upstreamErr.Body); writeErr != nil {
				log.Debug("[WriteUpstreamSSEError] Failed to write upstream error body", zap.Error(writeErr))
			}
		} else {
			if _, writeErr := fmt.Fprintf(w, constant.SSEOpenAIUpstreamErrorFrame, upstreamErr.StatusCode); writeErr != nil {
				log.Debug("[WriteUpstreamSSEError] Failed to write upstream status frame", zap.Error(writeErr))
			}
		}
	} else {
		log.Error("[WriteUpstreamSSEError] Non-upstream error in SSE stream", zap.Error(err))
		if _, writeErr := fmt.Fprint(w, constant.SSEOpenAIInternalErrorFrame); writeErr != nil {
			log.Debug("[WriteUpstreamSSEError] Failed to write internal error frame", zap.Error(writeErr))
		}
	}
	if flushErr := w.Flush(); flushErr != nil {
		log.Debug("[WriteUpstreamSSEError] Failed to flush SSE writer", zap.Error(flushErr))
	}
}

// SendOpenAIModelNotFoundError 发送OpenAI模型不存在错误
//
//	@return rsp
//	@author centonhuang
//	@update 2026-03-06 15:58:35
func SendOpenAIModelNotFoundError(modelName string) (rsp *huma.StreamResponse) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			humaCtx.SetStatus(http.StatusNotFound)
			humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
			_, _ = humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.OpenAIError{
				Message: fmt.Sprintf(constant.OpenAIModelNotFoundMessageTemplate, modelName),
				Type:    constant.OpenAIInvalidRequestErrorType,
				Code:    constant.OpenAIModelNotFoundCode,
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
			humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
			_, _ = humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.OpenAIError{
				Message: constant.OpenAIInternalErrorShortMessage,
				Type:    constant.OpenAIInternalErrorType,
				Code:    constant.OpenAIInternalErrorCode,
			})))
		},
	}
}
