package proxy

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// AnthropicProxy Anthropic 协议上游代理
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type AnthropicProxy interface {
	// ForwardCreateMessage 非流式转发
	//
	//	@param ctx context.Context
	//	@param ep UpstreamEndpoint
	//	@param body []byte
	//	@return *dto.AnthropicMessage
	//	@return error
	ForwardCreateMessage(ctx context.Context, ep UpstreamEndpoint, body []byte) (*dto.AnthropicMessage, error)

	// ForwardCreateMessageStream 流式转发，每个事件调用 onEvent 回调，返回合并后的完整响应
	//
	//	@param ctx context.Context
	//	@param ep UpstreamEndpoint
	//	@param body []byte
	//	@param onEvent func(dto.AnthropicSSEEvent) error
	//	@return *dto.AnthropicMessage
	//	@return error
	ForwardCreateMessageStream(ctx context.Context, ep UpstreamEndpoint, body []byte, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error)

	// ForwardCountTokens 转发 Count Tokens 请求
	//
	//	@param ctx context.Context
	//	@param ep UpstreamEndpoint
	//	@param body []byte
	//	@return *dto.AnthropicTokensCount
	//	@return error
	ForwardCountTokens(ctx context.Context, ep UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error)
}

type anthropicProxy struct{}

// NewAnthropicProxy 创建 Anthropic 代理
//
//	@return AnthropicProxy
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func NewAnthropicProxy() AnthropicProxy {
	return &anthropicProxy{}
}

func (p *anthropicProxy) ForwardCreateMessage(ctx context.Context, ep UpstreamEndpoint, body []byte) (*dto.AnthropicMessage, error) {
	logger := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, "/v1/messages", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("[AnthropicProxy] Read upstream response error", zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	message := &dto.AnthropicMessage{}
	if err := sonic.Unmarshal(respBody, message); err != nil {
		logger.Warn("[AnthropicProxy] Unmarshal upstream response error", zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	return message, nil
}

func (p *anthropicProxy) ForwardCreateMessageStream(ctx context.Context, ep UpstreamEndpoint, body []byte, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	logger := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, "/v1/messages", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var collectedEvents []dto.AnthropicSSEEvent
	var currentEvent string

	reader := bufio.NewReader(resp.Body)
	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimRight(raw, "\r\n")

		if line != "" {
			if eventType, ok := strings.CutPrefix(line, "event: "); ok {
				currentEvent = eventType
			} else if payload, ok := strings.CutPrefix(line, "data: "); ok {
				event := dto.AnthropicSSEEvent{
					Event: currentEvent,
					Data:  []byte(payload),
				}
				collectedEvents = append(collectedEvents, event)

				if err := onEvent(event); err != nil {
					logger.Warn("[AnthropicProxy] OnEvent callback error", zap.Error(err))
					return nil, err
				}
			}
		}

		if readErr != nil {
			if readErr != io.EOF {
				logger.Warn("[AnthropicProxy] Read upstream sse error", zap.Error(readErr))
				return nil, &model.UpstreamConnectionError{Cause: readErr}
			}
			break
		}
	}

	if len(collectedEvents) == 0 {
		return nil, nil
	}

	return util.ConcatAnthropicSSEEvents(collectedEvents)
}

func (p *anthropicProxy) ForwardCountTokens(ctx context.Context, ep UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error) {
	logger := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, "/v1/messages/count_tokens", body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("[AnthropicProxy] Read upstream response error", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyResponse, err, "read upstream response")
	}

	rsp := &dto.AnthropicTokensCount{}
	if err := sonic.Unmarshal(respBody, rsp); err != nil {
		logger.Warn("[AnthropicProxy] Unmarshal upstream response error", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyResponse, err, "unmarshal upstream response")
	}

	return rsp, nil
}

// sendRequest 构建并发送 Anthropic 协议的上游请求
func (p *anthropicProxy) sendRequest(ctx context.Context, ep UpstreamEndpoint, path string, body []byte) (*http.Response, error) {
	logger := logger.WithCtx(ctx)

	upstreamURL := strings.TrimRight(ep.BaseURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		logger.Error("[AnthropicProxy] New request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", ep.APIKey)
	req.Header.Set("anthropic-version", constant.AnthropicAPIVersion)

	logger.Info("[AnthropicProxy] Send upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", ep.Model),
		zap.Any("upstreamAPIKey", util.MaskSecret(ep.APIKey)))

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		logger.Error("[AnthropicProxy] Send http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		logger.Error("[AnthropicProxy] Upstream returned non-200 status",
			zap.String("upstreamURL", upstreamURL),
			zap.Int("statusCode", resp.StatusCode),
			zap.String("responseBody", string(errorBody)),
		)
		return nil, &model.UpstreamError{StatusCode: resp.StatusCode, Body: string(errorBody)}
	}

	return resp, nil
}

// ReplaceModelInBody 替换 JSON body 中的 model 字段
//
//	@param body []byte
//	@param model string
//	@return []byte
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func ReplaceModelInBody(body []byte, modelName string) []byte {
	var bodyMap map[string]any
	if err := sonic.Unmarshal(body, &bodyMap); err != nil {
		return body
	}
	bodyMap["model"] = modelName
	return lo.Must1(sonic.Marshal(bodyMap))
}

// ReplaceModelInSSEData 替换 Anthropic SSE data 中的 model 字段（包括嵌套的 message.model）
//
//	@param data []byte
//	@param model string
//	@return []byte
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func ReplaceModelInSSEData(data []byte, modelName string) []byte {
	var dataMap map[string]any
	if err := sonic.Unmarshal(data, &dataMap); err != nil {
		return data
	}
	if msgRaw, ok := dataMap["message"]; ok {
		if msgMap, ok := msgRaw.(map[string]any); ok {
			if _, hasModel := msgMap["model"]; hasModel {
				msgMap["model"] = modelName
			}
		}
	}
	if _, hasModel := dataMap["model"]; hasModel {
		dataMap["model"] = modelName
	}
	return lo.Must1(sonic.Marshal(dataMap))
}
