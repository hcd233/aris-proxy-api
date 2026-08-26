package transport

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"go.uber.org/zap"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"

	usecase "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
)

type anthropicProxy struct {
	tracker *inflight.Tracker
	guard   *EndpointGuard
}

var _ usecase.AnthropicProxyPort = (*anthropicProxy)(nil)

func NewAnthropicProxy(tracker *inflight.Tracker, guard *EndpointGuard) usecase.AnthropicProxyPort {
	return &anthropicProxy{tracker: tracker, guard: guard}
}

func (p *anthropicProxy) ForwardCreateMessage(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicMessage, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, constant.UpstreamPathAnthropicMessages, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // ensure body closed on return

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[AnthropicProxy] Read upstream response error", zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	message := &dto.AnthropicMessage{}
	if err := sonic.Unmarshal(respBody, message); err != nil {
		log.Warn("[AnthropicProxy] Unmarshal upstream response error", zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	return message, nil
}

func (p *anthropicProxy) OpenCreateMessageStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	resp, err := p.sendRequest(ctx, ep, constant.UpstreamPathAnthropicMessages, body)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (p *anthropicProxy) ReadCreateMessageStream(ctx context.Context, stream io.ReadCloser, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	log := logger.WithCtx(ctx)
	defer func() { _ = stream.Close() }() //nolint:errcheck // ensure stream closed on return

	agg := proxyutil.NewAnthropicSSEStreamAggregator()
	var currentEvent string

	reader := bufio.NewReader(stream)
	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimRight(raw, constant.NewlineCRLF)

		if eventType, ok := strings.CutPrefix(line, constant.SSEEventPrefix); ok {
			currentEvent = eventType
		} else if payload, ok := strings.CutPrefix(line, constant.SSEDataPrefix); ok {
			event := dto.AnthropicSSEEvent{
				Event: currentEvent,
				Data:  []byte(payload),
			}
			if err := forwardAndAggregateSSEEvent(ctx, event, onEvent, agg); err != nil {
				return nil, err
			}
		}

		if readErr != nil {
			if readErr != io.EOF {
				log.Warn("[AnthropicProxy] Read upstream sse error", zap.Error(readErr))
				return nil, &model.UpstreamConnectionError{Cause: readErr}
			}
			break
		}
	}

	if agg.Count() == 0 {
		return nil, nil
	}

	return agg.Message()
}

// forwardAndAggregateSSEEvent 先把 SSE 事件转发给回调，再做增量聚合。
func forwardAndAggregateSSEEvent(ctx context.Context, event dto.AnthropicSSEEvent, onEvent func(dto.AnthropicSSEEvent) error, agg *proxyutil.AnthropicSSEStreamAggregator) error {
	log := logger.WithCtx(ctx)
	if err := onEvent(event); err != nil {
		log.Warn("[AnthropicProxy] OnEvent callback error", zap.Error(err))
		return err
	}
	if err := agg.Add(event); err != nil {
		log.Warn("[AnthropicProxy] Aggregate sse event error", zap.Error(err))
		return err
	}
	return nil
}

func (p *anthropicProxy) ForwardCountTokens(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, constant.UpstreamPathAnthropicCountTokens, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // ensure body closed on return

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[AnthropicProxy] Read upstream response error", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyResponse, err, "read upstream response")
	}

	rsp := &dto.AnthropicTokensCount{}
	if err := sonic.Unmarshal(respBody, rsp); err != nil {
		log.Warn("[AnthropicProxy] Unmarshal upstream response error", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyResponse, err, "unmarshal upstream response")
	}

	return rsp, nil
}

// sendRequest 构建并发送 Anthropic 协议的上游请求：先过容错守卫（熔断/信号量），再对可重试错误自动重试。
// ctx 融合 drain 广播：优雅退出 soft deadline 到达时取消上游连接，
// 使阻塞的 SSE 读循环返回 context canceled（礼貌断流的前半段）。
// bulkhead 槽位与熔断上报绑定到返回 body 的 Close（见 BindLease）：
// 流式响应在整个消费阶段占用并发槽；body 读错误（非 EOF）经租约翻转
// 计入熔断失败，上游半死（断流）因此能触发熔断。
func (p *anthropicProxy) sendRequest(ctx context.Context, ep vo.UpstreamEndpoint, path string, body []byte) (*http.Response, error) {
	ctx = p.tracker.CancelOnDrain(ctx)
	key := EndpointKey(ep)
	lease, err := p.guard.Allow(ctx, key)
	if err != nil {
		return nil, err
	}

	sendFn := func() (*http.Response, error) {
		return p.sendRequestOnce(ctx, ep, path, body)
	}
	resp, err := SendUpstreamWithRetry(ctx, constant.ModuleAnthropicProxy, sendFn)
	if err != nil || resp == nil {
		// 未拿到响应体：立即结束租约并上报结果
		lease.Done(!IsCircuitError(err))
		return resp, err
	}
	// 响应头已到达；成功与否延迟到 body 消费完成时判定上报
	resp.Body = p.guard.BindLease(resp.Body, lease, true)
	return resp, nil
}

// sendRequestOnce 执行单次 Anthropic 协议上游请求发送（不含重试逻辑）
func (p *anthropicProxy) sendRequestOnce(ctx context.Context, ep vo.UpstreamEndpoint, path string, body []byte) (*http.Response, error) {
	log := logger.WithCtx(ctx)

	upstreamURL := strings.TrimRight(ep.BaseURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		log.Error("[AnthropicProxy] New request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "create request")
	}

	// 透传客户端请求头
	applyPassthroughRequestHeaders(ctx, req.Header)

	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+ep.APIKey)
	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	req.Header.Set(constant.HTTPHeaderAPIKey, ep.APIKey)
	req.Header.Set(constant.HTTPHeaderAnthropicVersion, constant.AnthropicAPIVersion)

	log.Info("[AnthropicProxy] Send upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", ep.Model),
		zap.String("upstreamAPIKey", commonutil.MaskSecret(ep.APIKey)),
		zap.Any("upstreamHeaders", util.MaskHTTPHeadersForLog(req.Header)),
		zap.Any("upstreamRequestSummary", parseUpstreamRequestSummary(body)),
	)

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // read best effort on error path
		_ = resp.Body.Close()                 //nolint:errcheck // close best effort on error path
		log.Error("[AnthropicProxy] Upstream returned non-200 status",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("upstreamURL", upstreamURL),
			zap.String("upstreamModel", ep.Model),
			zap.String("responseBody", string(errorBody)),
		)
		return nil, &model.UpstreamError{
			StatusCode: resp.StatusCode,
			Headers:    capturePassthroughResponseHeaders(resp.Header),
			Body:       string(errorBody),
		}
	}

	storePassthroughResponseHeaders(ctx, resp.Header)

	return resp, nil
}
