// Package util 工具包
package util

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// WrapHTTPResponse 包装HTTP响应错误
//
//	@param rsp rspT
//	@param err error
//	@return *dto.HTTPResponse[rspT]
//	@return error
//	@author centonhuang
//	@update 2025-11-11 04:58:31
func WrapHTTPResponse[rspT any](rsp rspT, err error) (*dto.HTTPResponse[rspT], error) {
	return &dto.HTTPResponse[rspT]{
		Body: rsp,
	}, err
}

// WriteErrorResponse 写入错误响应
//
//	@param ctx
//	@param err
//	@return error
//	@author centonhuang
//	@update 2025-11-10 20:55:14
func WriteErrorResponse(bodyWriter io.Writer, err *model.Error) error {
	_, writeErr := bodyWriter.Write(lo.Must1(sonic.Marshal(&dto.CommonRsp{Error: err})))
	return writeErr
}

// WriteErrorHTTPResponse 写入带 HTTP 状态码和 JSON Content-Type 的错误响应
//
//	@param ctx huma.Context
//	@param statusCode int
//	@param err *model.Error
//	@return error
//	@author centonhuang
//	@update 2026-05-12 14:20:00
func WriteErrorHTTPResponse(ctx huma.Context, statusCode int, err *model.Error) error {
	ctx.SetStatus(statusCode)
	ctx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
	return WriteErrorResponse(ctx.BodyWriter(), err)
}

// WrapStreamResponse 创建 SSE 流式响应包装
//
//	@param handler func(w *bufio.Writer)
//	@return *huma.StreamResponse
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func WrapStreamResponse(handler func(w *bufio.Writer)) *huma.StreamResponse {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			fiberCtx := humafiber.Unwrap(humaCtx)
			fiberCtx.Set(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeEventStream)
			fiberCtx.Set(constant.HTTPTitleHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fiberCtx.Set(constant.HTTPLowerHeaderConnection, constant.HTTPConnectionKeepAlive)
			fiberCtx.Set(constant.HTTPLowerHeaderTransferEncoding, constant.HTTPTransferEncodingChunked)
			fiberCtx.Set(constant.HTTPTitleHeaderXAccelBuffering, constant.HTTPHeaderDisabled)
			fiberCtx.Status(fiber.StatusOK).Response().SetBodyStreamWriter(fasthttp.StreamWriter(handler))
		},
	}
}

// JSONResponseWriter JSON 响应写入器
//
//	@author centonhuang
//	@update 2026-04-29 10:00:00
type JSONResponseWriter struct {
	HumaCtx huma.Context
	Ctx     context.Context
}

// WriteJSON 写入 JSON 响应
//
//	@receiver rw JSONResponseWriter
//	@param v any
//	@author centonhuang
//	@update 2026-04-29 10:00:00
func (rw JSONResponseWriter) WriteJSON(v any) {
	if headers := GetPassthroughResponseHeaders(rw.Ctx); headers != nil {
		for k, hv := range headers {
			rw.HumaCtx.SetHeader(k, hv)
		}
	}
	rw.HumaCtx.SetStatus(fiber.StatusOK)
	rw.HumaCtx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
	_, _ = rw.HumaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(v)))
}

// WriteError 写入自定义状态码和 JSON body 的错误响应
//
//	@receiver rw JSONResponseWriter
//	@param statusCode int
//	@param body []byte
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func (rw JSONResponseWriter) WriteError(statusCode int, body []byte) {
	rw.HumaCtx.SetStatus(statusCode)
	rw.HumaCtx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
	_, _ = rw.HumaCtx.BodyWriter().Write(body)
}

// WrapJSONResponse 创建 JSON 响应包装
//
//	@param ctx context.Context
//	@param handler func(writer JSONResponseWriter)
//	@return *huma.StreamResponse
//	@author centonhuang
//	@update 2026-04-29 10:00:00
func WrapJSONResponse(ctx context.Context, handler func(writer JSONResponseWriter)) *huma.StreamResponse {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			handler(JSONResponseWriter{HumaCtx: humaCtx, Ctx: ctx})
		},
	}
}

// WriteUpstreamError 将上游错误写入响应，支持上游错误透传和兜底错误
//
//	@param writer JSONResponseWriter
//	@param err error
//	@param fallbackBody []byte
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func WriteUpstreamError(writer JSONResponseWriter, err error, fallbackBody []byte) {
	log := logger.WithCtx(writer.Ctx)
	var upstreamErr *model.UpstreamError
	if errors.As(err, &upstreamErr) {
		for k, v := range upstreamErr.Headers {
			writer.HumaCtx.SetHeader(k, v)
		}
		writer.HumaCtx.SetStatus(upstreamErr.StatusCode)
		writer.HumaCtx.SetHeader(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = writer.HumaCtx.BodyWriter().Write([]byte(upstreamErr.Body))
		return
	}
	log.Error("[ProxyService] Proxy error", zap.Error(err))
	writer.WriteError(fiber.StatusBadGateway, fallbackBody)
}

// ExtractUpstreamStatusAndError 从 error 中提取上游 HTTP 状态码和错误消息字符串
//
//	状态码语义：
//	- 200 ：成功（err == nil）
//	- >0  ：上游返回的 HTTP 状态码（UpstreamError）
//	- -1  ：上游连接错误（网络层失败，无法获取 HTTP 状态码，UpstreamConnectionError）
//	- 0   ：其它未知错误（如 DTO 转换失败、上下文取消等，非上游传输问题）
//
//	@param err error
//	@return int statusCode
//	@return string errorMessage
//	@author centonhuang
//	@update 2026-04-20 11:00:00
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
		return constant.CallStatusConnectionError, connErr.Error()
	}
	return constant.CallStatusUnknownError, err.Error()
}
