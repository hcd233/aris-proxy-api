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

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// PingHandler 健康检查处理器
//
//	author centonhuang
//	update 2025-01-04 15:52:48
type PingHandler interface {
	HandlePing(ctx context.Context, req *dto.EmptyReq) (rsp *dto.HTTPResponse[*dto.PingRsp], err error)
	HandleReady(ctx context.Context, req *dto.EmptyReq) (rsp *dto.HTTPResponse[*dto.PingRsp], err error)
	HandleSSEPing(ctx context.Context, req *dto.EmptyReq) (rsp *huma.StreamResponse, err error)
}

type pingHandler struct {
	tracker *inflight.Tracker
}

// NewPingHandler 创建健康检查处理器
//
//	return PingHandler
//	author centonhuang
//	update 2025-01-04 15:52:48
func NewPingHandler(tracker *inflight.Tracker) PingHandler {
	return &pingHandler{tracker: tracker}
}

// HandlePing 健康检查处理器
func (h *pingHandler) HandlePing(_ context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.PingRsp], error) {
	rsp := &dto.PingRsp{
		Status: constant.PingStatusOK,
	}

	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *pingHandler) HandleReady(_ context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.PingRsp], error) {
	if h.tracker.IsDraining() {
		return nil, huma.Error503ServiceUnavailable(constant.ServerShuttingDownMsg)
	}
	rsp := &dto.PingRsp{
		Status: constant.PingStatusOK,
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
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

			_ = fCtx.SendStreamWriter(func(w *bufio.Writer) { //nolint:errcheck // SSE healthcheck
				for i := range constant.SSEHeartbeatCount {
					data := &dto.SSEResponse{
						DataType: enum.SSEDataTypeHeartBeat,
						Data:     strconv.Itoa(i),
					}
					_, _ = fmt.Fprintf(w, constant.SSEDataFrameTemplate, lo.Must1(sonic.Marshal(data))) //nolint:errcheck // best-effort write
					err := w.Flush()
					if err != nil {
						return
					}
					time.Sleep(constant.HeartbeatInterval)
				}
			})
		},
	}, nil
}
