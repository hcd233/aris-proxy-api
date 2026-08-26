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
	"github.com/samber/mo"

	usecase "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
)

type openAIProxy struct {
	tracker *inflight.Tracker
	guard   *EndpointGuard
}

var _ usecase.OpenAIProxyPort = (*openAIProxy)(nil)

func NewOpenAIProxy(tracker *inflight.Tracker, guard *EndpointGuard) usecase.OpenAIProxyPort {
	return &openAIProxy{tracker: tracker, guard: guard}
}

func (p *openAIProxy) ForwardChatCompletion(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.doUpstreamRequest(ctx, ep, body, constant.UpstreamPathOpenAIChatCompletions)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // ensure body closed on return

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[OpenAIProxy] Read upstream response error", zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	completion := &dto.OpenAIChatCompletion{}
	if err := sonic.Unmarshal(respBody, completion); err != nil {
		log.Warn("[OpenAIProxy] Unmarshal upstream response error", zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	return completion, nil
}

func (p *openAIProxy) OpenChatCompletionStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	resp, err := p.doUpstreamRequest(ctx, ep, body, constant.UpstreamPathOpenAIChatCompletions)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (p *openAIProxy) ReadChatCompletionStream(ctx context.Context, stream io.ReadCloser, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	log := logger.WithCtx(ctx)
	defer func() { _ = stream.Close() }() //nolint:errcheck // ensure stream closed on return

	agg := proxyutil.NewChatCompletionStreamAggregator()

	reader := bufio.NewReader(stream)
	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimRight(raw, constant.NewlineCRLF)

		if readErr != nil {
			if readErr != io.EOF {
				log.Warn("[OpenAIProxy] Read upstream sse error", zap.Error(readErr))
				return nil, &model.UpstreamConnectionError{Cause: readErr}
			}
			break
		}

		chunk, ok := parseSSEDataLine(line).Get()
		if !ok {
			continue
		}
		agg.Add(chunk)
		if err := onChunk(chunk); err != nil {
			log.Warn("[OpenAIProxy] OnChunk callback error", zap.Error(err))
			return nil, err
		}
	}

	if agg.Count() == 0 {
		return nil, nil
	}

	return agg.Completion(), nil
}

func parseSSEDataLine(line string) mo.Option[*dto.OpenAIChatCompletionChunk] {
	if !strings.HasPrefix(line, constant.SSEDataPrefix) {
		return mo.None[*dto.OpenAIChatCompletionChunk]()
	}
	payload := line[len(constant.SSEDataPrefix):]
	if payload == "" || payload == constant.SSEDoneSignal {
		return mo.None[*dto.OpenAIChatCompletionChunk]()
	}
	chunk := &dto.OpenAIChatCompletionChunk{}
	if err := sonic.UnmarshalString(payload, chunk); err != nil {
		zap.L().Warn("[OpenAIProxy] Unmarshal sse chunk error", zap.String("payload", payload), zap.Error(err))
		return mo.None[*dto.OpenAIChatCompletionChunk]()
	}
	return mo.Some(chunk)
}

// doUpstreamRequest 构建并发送上游 HTTP 请求：先过容错守卫（熔断/信号量），再对可重试错误自动重试。
// ctx 融合 drain 广播：优雅退出 soft deadline 到达时取消上游连接，
// 使阻塞的 SSE 读循环返回 context canceled（礼貌断流的前半段）。
// bulkhead 槽位与熔断上报绑定到返回 body 的 Close（见 BindLease）：
// 流式响应在整个消费阶段占用并发槽；body 读错误（非 EOF）经租约翻转
// 计入熔断失败，上游半死（断流）因此能触发熔断。
func (p *openAIProxy) doUpstreamRequest(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, pathSuffix string) (*http.Response, error) {
	ctx = p.tracker.CancelOnDrain(ctx)
	key := EndpointKey(ep)
	lease, err := p.guard.Allow(ctx, key)
	if err != nil {
		return nil, err
	}

	sendFn := func() (*http.Response, error) {
		return p.sendUpstreamRequestOnce(ctx, ep, body, pathSuffix)
	}
	resp, err := SendUpstreamWithRetry(ctx, constant.ModuleOpenAIProxy, sendFn)
	if err != nil || resp == nil {
		// 未拿到响应体：立即结束租约并上报结果
		lease.Done(!IsCircuitError(err))
		return resp, err
	}
	// 响应头已到达；成功与否延迟到 body 消费完成时判定上报
	resp.Body = p.guard.BindLease(resp.Body, lease, true)
	return resp, nil
}

// sendUpstreamRequestOnce 执行单次上游 HTTP 请求发送（不含重试逻辑）
func (p *openAIProxy) sendUpstreamRequestOnce(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, pathSuffix string) (*http.Response, error) {
	log := logger.WithCtx(ctx)

	upstreamURL := strings.TrimRight(ep.BaseURL, "/") + pathSuffix

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		log.Error("[OpenAIProxy] New request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "create request")
	}

	// 透传客户端请求头
	applyPassthroughRequestHeaders(ctx, req.Header)

	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+ep.APIKey)

	log.Info("[OpenAIProxy] Send upstream request", zap.String("upstreamURL", upstreamURL),
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
		log.Error("[OpenAIProxy] Upstream returned non-200 status",
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

func (p *openAIProxy) ForwardCreateResponse(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.doUpstreamRequest(ctx, ep, body, constant.UpstreamPathOpenAIResponses)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // ensure body closed on return

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[OpenAIProxy] Read response api upstream response error", zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	return respBody, nil
}

func (p *openAIProxy) OpenCreateResponseStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	resp, err := p.doUpstreamRequest(ctx, ep, body, constant.UpstreamPathOpenAIResponses)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (p *openAIProxy) ReadCreateResponseStream(ctx context.Context, stream io.ReadCloser, onEvent func(event string, data []byte) error) error {
	log := logger.WithCtx(ctx)
	defer func() { _ = stream.Close() }() //nolint:errcheck // ensure stream closed on return

	reader := bufio.NewReader(stream)
	var currentEvent string

	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimRight(raw, constant.NewlineCRLF)

		if line != "" {
			if strings.HasPrefix(line, constant.SSEEventPrefix) {
				currentEvent = line[len(constant.SSEEventPrefix):]
			} else if strings.HasPrefix(line, constant.SSEDataPrefix) {
				payload := line[len(constant.SSEDataPrefix):]
				if err := onEvent(currentEvent, []byte(payload)); err != nil {
					log.Warn("[OpenAIProxy] Response API onEvent callback error", zap.Error(err))
					return err
				}
				currentEvent = ""
			}
		}

		if readErr != nil {
			if readErr != io.EOF {
				log.Warn("[OpenAIProxy] Read response api upstream sse error", zap.Error(readErr))
				return &model.UpstreamConnectionError{Cause: readErr}
			}
			break
		}
	}

	return nil
}
