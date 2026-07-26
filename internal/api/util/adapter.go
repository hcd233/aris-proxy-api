package apiutil

import (
	"bufio"
	"context"
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// sseWriter 实现 port.EventSink，将协议事件编码为 SSE 帧写入 bufio.Writer。
//
// event 非空时输出 "event: <event>\n" + "data: <data>\n\n"（Anthropic Responses 格式）；
// event 为空时输出 "data: <data>\n\n"（OpenAI Chat data-only 格式）。
// 每次写入后 Flush，使下游 fasthttp 立即发送。
type sseWriter struct {
	w *bufio.Writer
}

// NewSSEWriter 创建一个写 bufio.Writer 的 port.EventSink。
func NewSSEWriter(w *bufio.Writer) port.EventSink {
	return &sseWriter{w: w}
}

func (s *sseWriter) WriteEvent(event string, data []byte) error {
	if event != "" {
		if _, err := fmt.Fprintf(s.w, constant.SSEEventLineTemplate, event); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(s.w, constant.SSEDataLineTemplate, data); err != nil {
		return err
	}
	return s.w.Flush()
}

// AdaptProxyResult 把 application 层的 transport-neutral 结果转换为 *huma.StreamResponse。
//
// err 非 nil 时按 ProxyError 或 fallback 处理；result 非 nil 时按 JSONResult/StreamResult 处理。
// 这是 application 与 Huma/Fiber 之间唯一的响应构造边界。
func AdaptProxyResult(ctx context.Context, result port.Result, err error, fallbackBody []byte) (*huma.StreamResponse, error) {
	if err != nil {
		var proxyErr *port.ProxyError
		if errors.As(err, &proxyErr) {
			return wrapProxyError(ctx, proxyErr), nil
		}
		return WrapJSONResponse(ctx, func(writer JSONResponseWriter) {
			WriteUpstreamError(writer, err, fallbackBody)
		}), nil
	}
	if result == nil {
		return nil, nil
	}
	switch r := result.(type) {
	case *port.JSONResult:
		return wrapJSONResult(ctx, r), nil
	case *port.StreamResult:
		return wrapStreamResult(ctx, r, fallbackBody), nil
	default:
		return nil, ierr.Newf(ierr.ErrInternal, constant.UnknownProxyResultTypeTemplate, result)
	}
}

func wrapProxyError(ctx context.Context, e *port.ProxyError) *huma.StreamResponse {
	return WrapJSONResponse(ctx, func(writer JSONResponseWriter) {
		writeProxyErrorBody(writer, e)
	})
}

func wrapJSONResult(ctx context.Context, r *port.JSONResult) *huma.StreamResponse {
	return WrapJSONResponse(ctx, func(writer JSONResponseWriter) {
		for k, v := range r.Headers {
			writer.HumaCtx.SetHeader(k, v)
		}
		writer.HumaCtx.SetStatus(r.StatusCode)
		writer.HumaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = writer.HumaCtx.BodyWriter().Write(r.Body) //nolint:errcheck // best-effort write in response handler
	})
}

// wrapStreamResult 把 StreamResult 转成 Huma SSE response。
//
// 关键约束：Open 在 body callback 内调用，且必须在写出 200 SSE headers 之前完成。
// Open 失败时改写为 JSON 错误响应（status/headers/body 来自 ProxyError），
// 避免上游打开阶段错误被包进 200 SSE 流。
func wrapStreamResult(ctx context.Context, r *port.StreamResult, fallbackBody []byte) *huma.StreamResponse {
	lc := streamLifecycleFromContext(ctx)
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			fiberCtx := humafiber.Unwrap(humaCtx)
			if headers := util.GetPassthroughResponseHeaders(humaCtx.Context()); headers != nil {
				for k, hv := range headers {
					fiberCtx.Set(k, hv)
				}
			}
			stream, err := r.Open(ctx)
			if err != nil {
				var proxyErr *port.ProxyError
				if errors.As(err, &proxyErr) {
					writeProxyErrorBody(JSONResponseWriter{HumaCtx: humaCtx}, proxyErr)
				} else {
					humaCtx.SetStatus(fiber.StatusBadGateway)
					humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
					_, _ = humaCtx.BodyWriter().Write(fallbackBody) //nolint:errcheck // best-effort write in error response
				}
				return
			}
			defer func() { _ = stream.Close() }() //nolint:errcheck // stream close errors are best-effort during shutdown

			fiberCtx.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeEventStream)
			fiberCtx.Set(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fiberCtx.Set(constant.HTTPHeaderConnection, constant.HTTPConnectionKeepAlive)
			fiberCtx.Set(constant.HTTPHeaderTransferEncoding, constant.HTTPTransferEncodingChunked)
			fiberCtx.Set(constant.HTTPHeaderXAccelBuffering, constant.HTTPHeaderDisabled)
			fiberCtx.Status(fiber.StatusOK)
			_ = fiberCtx.SendStreamWriter(func(w *bufio.Writer) { //nolint:errcheck // stream write errors propagate via the Fiber error handler
				if lc.onStart != nil {
					lc.onStart()
				}
				if lc.onEnd != nil {
					defer lc.onEnd()
				}
				sink := NewSSEWriter(w)
				_ = stream.Read(ctx, sink) //nolint:errcheck // stream read errors propagate via SSE error frames written by application
			})
		},
	}
}

func writeProxyErrorBody(writer JSONResponseWriter, e *port.ProxyError) {
	for k, v := range e.Headers {
		writer.HumaCtx.SetHeader(k, v)
	}
	writer.HumaCtx.SetStatus(e.StatusCode)
	writer.HumaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	_, _ = writer.HumaCtx.BodyWriter().Write(e.Body) //nolint:errcheck // best-effort write in error response
}
