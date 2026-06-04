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
	ctx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
	return WriteErrorResponse(ctx.BodyWriter(), err)
}

func WrapStreamResponse(handler func(w *bufio.Writer)) *huma.StreamResponse {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			fiberCtx := humafiber.Unwrap(humaCtx)
			if headers := util.GetPassthroughResponseHeaders(humaCtx.Context()); headers != nil {
				for k, hv := range headers {
					fiberCtx.Set(k, hv)
				}
			}
			fiberCtx.Set(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeEventStream)
			fiberCtx.Set(constant.HTTPTitleHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fiberCtx.Set(constant.HTTPLowerHeaderConnection, constant.HTTPConnectionKeepAlive)
			fiberCtx.Set(constant.HTTPLowerHeaderTransferEncoding, constant.HTTPTransferEncodingChunked)
			fiberCtx.Set(constant.HTTPTitleHeaderXAccelBuffering, constant.HTTPHeaderDisabled)
			fiberCtx.Status(fiber.StatusOK)
			_ = fiberCtx.SendStreamWriter(handler) //nolint:errcheck // stream write errors propagate via the Fiber error handler
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
	rw.HumaCtx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
	_, _ = rw.HumaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(v))) //nolint:errcheck // best-effort response write
}

func (rw JSONResponseWriter) WriteError(statusCode int, body []byte) {
	rw.HumaCtx.SetStatus(statusCode)
	rw.HumaCtx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
	_, _ = rw.HumaCtx.BodyWriter().Write(body) //nolint:errcheck // best-effort response write
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
		writer.HumaCtx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = writer.HumaCtx.BodyWriter().Write([]byte(upstreamErr.Body)) //nolint:errcheck // best-effort write on error response
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
