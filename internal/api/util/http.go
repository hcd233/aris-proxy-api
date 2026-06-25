package apiutil

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

func WrapHTTPResponse[rspT any](rsp rspT, err error) (*dto.HTTPResponse[rspT], error) {
	return &dto.HTTPResponse[rspT]{
		Body: rsp,
	}, err
}

func WriteErrorResponse(bodyWriter io.Writer, err *model.Error) error {
	_, writeErr := bodyWriter.Write(lo.Must1(sonic.Marshal(&dto.CommonRsp{Error: err})))
	return writeErr
}

func WriteErrorHTTPResponse(ctx huma.Context, statusCode int, err *model.Error) error {
	ctx.SetStatus(statusCode)
	ctx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	return WriteErrorResponse(ctx.BodyWriter(), err)
}

// streamLifecycle 承载包裹"真实流式写入"的起止回调。
//
// 必须在 SendStreamWriter 的回调内部触发，而非在注册阶段：
// fiber/fasthttp 的 SendStreamWriter 只是把写函数登记到响应上，注册后立即返回，
// 真正的流式写入要等 handler 链返回后由 fasthttp 在连接写阶段调用。
// 若在注册前后 bracket，回调会在微秒内 start→end，导致 SSE 并发 gauge 恒为 0。
type streamLifecycle struct {
	onStart func()
	onEnd   func()
}

type streamLifecycleKeyType struct{}

var streamLifecycleKey streamLifecycleKeyType

// WithStreamLifecycle 在 ctx 上挂载流式写入的起止回调。
//
// onStart 在真正开始向客户端写流时触发，onEnd 在流结束（含异常/中断）时触发；
// 二者由 WrapStreamResponse 在 SendStreamWriter 回调内部 bracket 真实写入过程。
//
//	@param ctx context.Context
//	@param onStart func()
//	@param onEnd func()
//	@return context.Context
//	@author centonhuang
//	@update 2026-06-25 20:00:00
func WithStreamLifecycle(ctx context.Context, onStart, onEnd func()) context.Context {
	return context.WithValue(ctx, streamLifecycleKey, streamLifecycle{onStart: onStart, onEnd: onEnd})
}

func streamLifecycleFromContext(ctx context.Context) streamLifecycle {
	lc, _ := ctx.Value(streamLifecycleKey).(streamLifecycle)
	return lc
}

func WrapStreamResponse(ctx context.Context, handler func(w *bufio.Writer)) *huma.StreamResponse {
	lc := streamLifecycleFromContext(ctx)
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			fiberCtx := humafiber.Unwrap(humaCtx)
			if headers := util.GetPassthroughResponseHeaders(humaCtx.Context()); headers != nil {
				for k, hv := range headers {
					fiberCtx.Set(k, hv)
				}
			}
			fiberCtx.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeEventStream)
			fiberCtx.Set(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fiberCtx.Set(constant.HTTPHeaderConnection, constant.HTTPConnectionKeepAlive)
			fiberCtx.Set(constant.HTTPHeaderTransferEncoding, constant.HTTPTransferEncodingChunked)
			fiberCtx.Set(constant.HTTPHeaderXAccelBuffering, constant.HTTPHeaderDisabled)
			fiberCtx.Status(fiber.StatusOK)
			// 在 SendStreamWriter 回调内部 bracket，确保 onStart/onEnd 覆盖真实流式写入全程。
			_ = fiberCtx.SendStreamWriter(func(w *bufio.Writer) { //nolint:errcheck // stream write errors propagate via the Fiber error handler
				if lc.onStart != nil {
					lc.onStart()
				}
				if lc.onEnd != nil {
					defer lc.onEnd()
				}
				handler(w)
			})
		},
	}
}

type JSONResponseWriter struct {
	HumaCtx huma.Context
	Ctx     context.Context
}

func (rw JSONResponseWriter) WriteJSON(v any) {
	if headers := util.GetPassthroughResponseHeaders(rw.Ctx); headers != nil {
		for k, hv := range headers {
			rw.HumaCtx.SetHeader(k, hv)
		}
	}
	rw.HumaCtx.SetStatus(fiber.StatusOK)
	rw.HumaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	_, _ = rw.HumaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(v))) //nolint:errcheck // best-effort write in response handler
}

func (rw JSONResponseWriter) WriteError(statusCode int, body []byte) {
	rw.HumaCtx.SetStatus(statusCode)
	rw.HumaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	_, _ = rw.HumaCtx.BodyWriter().Write(body) //nolint:errcheck // best-effort write in error response
}

func WrapJSONResponse(ctx context.Context, handler func(writer JSONResponseWriter)) *huma.StreamResponse {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			handler(JSONResponseWriter{HumaCtx: humaCtx, Ctx: ctx})
		},
	}
}

func WriteUpstreamError(writer JSONResponseWriter, err error, fallbackBody []byte) {
	log := logger.WithCtx(writer.Ctx)
	var upstreamErr *model.UpstreamError
	if errors.As(err, &upstreamErr) {
		for k, v := range upstreamErr.Headers {
			writer.HumaCtx.SetHeader(k, v)
		}
		writer.HumaCtx.SetStatus(upstreamErr.StatusCode)
		writer.HumaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = writer.HumaCtx.BodyWriter().Write([]byte(upstreamErr.Body)) //nolint:errcheck // best-effort write in stream error handler
		return
	}
	log.Error("[ProxyService] Proxy error", zap.Error(err))
	writer.WriteError(fiber.StatusBadGateway, fallbackBody)
}

func ExtractUpstreamStatusAndError(err error) (statusCode int, errorMessage string) {
	if err == nil {
		return fiber.StatusOK, ""
	}
	var ue *model.UpstreamError
	if errors.As(err, &ue) {
		msg := ue.Error()
		if ue.Body != "" {
			msg += fmt.Sprintf(constant.ColonMessageTemplate, ue.Body)
		}
		return ue.StatusCode, msg
	}
	var connErr *model.UpstreamConnectionError
	if errors.As(err, &connErr) {
		return enum.CallStatusConnectionError, connErr.Error()
	}
	return enum.CallStatusUnknownError, err.Error()
}
