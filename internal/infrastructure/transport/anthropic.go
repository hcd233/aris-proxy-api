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
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"

	usecase "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
)

type anthropicProxy struct{}

var _ usecase.AnthropicProxyPort = (*anthropicProxy)(nil)

func NewAnthropicProxy() usecase.AnthropicProxyPort {
	return &anthropicProxy{}
}

func (p *anthropicProxy) ForwardCreateMessage(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicMessage, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, constant.UpstreamPathAnthropicMessages, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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

func (p *anthropicProxy) ForwardCreateMessageStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, constant.UpstreamPathAnthropicMessages, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var collectedEvents []dto.AnthropicSSEEvent
	var currentEvent string

	reader := bufio.NewReader(resp.Body)
	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimRight(raw, constant.NewlineCRLF)

		if line != "" {
			if eventType, ok := strings.CutPrefix(line, constant.SSEEventPrefix); ok {
				currentEvent = eventType
			} else if payload, ok := strings.CutPrefix(line, constant.SSEDataPrefix); ok {
				event := dto.AnthropicSSEEvent{
					Event: currentEvent,
					Data:  []byte(payload),
				}
				collectedEvents = append(collectedEvents, event)

				if err := onEvent(event); err != nil {
					log.Warn("[AnthropicProxy] OnEvent callback error", zap.Error(err))
					return nil, err
				}
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

	if len(collectedEvents) == 0 {
		return nil, nil
	}

	return proxyutil.ConcatAnthropicSSEEvents(collectedEvents)
}

func (p *anthropicProxy) ForwardCountTokens(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, constant.UpstreamPathAnthropicCountTokens, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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

// sendRequest 构建并发送 Anthropic 协议的上游请求
func (p *anthropicProxy) sendRequest(ctx context.Context, ep vo.UpstreamEndpoint, path string, body []byte) (*http.Response, error) {
	log := logger.WithCtx(ctx)

	upstreamURL := strings.TrimRight(ep.BaseURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		log.Error("[AnthropicProxy] New request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "create request")
	}

	// 透传客户端请求头
	applyPassthroughRequestHeaders(ctx, req.Header)

	setRequestHeader(req.Header, constant.HTTPTitleHeaderAuthorization, constant.HTTPAuthBearerPrefix+ep.APIKey)
	setRequestHeader(req.Header, constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
	setRequestHeader(req.Header, constant.HTTPLowerHeaderAPIKey, ep.APIKey)
	setRequestHeader(req.Header, constant.HTTPLowerHeaderAnthropicVersion, constant.AnthropicAPIVersion)

	log.Info("[AnthropicProxy] Send upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", ep.Model),
		zap.String("upstreamAPIKey", commonutil.MaskSecret(ep.APIKey)),
		zap.Any("upstreamHeaders", util.MaskHTTPHeadersForLog(req.Header)),
		zap.ByteString("upstreamBody", body),
	)

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		log.Error("[AnthropicProxy] Send http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		log.Error("[AnthropicProxy] Upstream returned non-200 status",
			zap.String("upstreamURL", upstreamURL),
			zap.Int("statusCode", resp.StatusCode),
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
