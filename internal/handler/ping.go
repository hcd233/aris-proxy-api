package handler

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"github.com/valyala/fasthttp"
)

// PingHandler 健康检查处理器
//
//	author centonhuang
//	update 2025-01-04 15:52:48
type PingHandler interface {
	HandlePing(ctx context.Context, req *dto.EmptyReq) (rsp *dto.HTTPResponse[*dto.PingRsp], err error)
	HandleSSEPing(ctx context.Context, req *dto.EmptyReq) (rsp *huma.StreamResponse, err error)
}

type pingHandler struct{}

// NewPingHandler 创建健康检查处理器
//
//	return PingHandler
//	author centonhuang
//	update 2025-01-04 15:52:48
func NewPingHandler() PingHandler {
	return &pingHandler{}
}

// HandlePing 健康检查处理器
func (h *pingHandler) HandlePing(_ context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.PingRsp], error) {
	rsp := &dto.PingRsp{
		Status: constant.PingStatusOK,
	}

	return util.WrapHTTPResponse(rsp, nil)
}

func (h *pingHandler) HandleSSEPing(_ context.Context, _ *dto.EmptyReq) (rsp *huma.StreamResponse, err error) {
	return &huma.StreamResponse{
		Body: func(ctx huma.Context) {
			fCtx := humafiber.Unwrap(ctx)
			fCtx.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeEventStream)
			fCtx.Set(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fCtx.Set(constant.HTTPHeaderConnection, constant.HTTPConnectionKeepAlive)
			fCtx.Set(constant.HTTPHeaderTransferEncoding, constant.HTTPTransferEncodingChunked)
			fCtx.Set(constant.HTTPHeaderXAccelBuffering, constant.HTTPHeaderDisabled)

			fCtx.Response().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
				for i := range constant.SSEHeartbeatCount {
					data := &dto.SSEResponse{
						DataType: enum.SSEDataTypeHeartBeat,
						Data:     strconv.Itoa(i),
					}
					_, _ = fmt.Fprintf(w, constant.SSEDataFrameTemplate, lo.Must1(sonic.Marshal(data)))
					err := w.Flush()
					if err != nil {
						return
					}
					time.Sleep(constant.HeartbeatInterval)
				}
			}))
		},
	}, nil
}
