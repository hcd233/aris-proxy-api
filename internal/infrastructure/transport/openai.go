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

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// OpenAIProxy OpenAI 协议上游代理
//
//	@author centonhuang
//	@update 2026-04-17 10:00:00
type OpenAIProxy interface {
	// ForwardChatCompletion 非流式转发
	//
	//	@param ctx context.Context
	//	@param ep UpstreamEndpoint
	//	@param body []byte
	//	@return *dto.OpenAIChatCompletion
	//	@return error
	ForwardChatCompletion(ctx context.Context, ep UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error)

	// ForwardChatCompletionStream 流式转发，每个 chunk 调用 onChunk 回调，返回合并后的完整响应
	//
	//	@param ctx context.Context
	//	@param ep UpstreamEndpoint
	//	@param body []byte
	//	@param onChunk func(*dto.OpenAIChatCompletionChunk) error
	//	@return *dto.OpenAIChatCompletion
	//	@return error
	ForwardChatCompletionStream(ctx context.Context, ep UpstreamEndpoint, body []byte, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error)

	// ForwardCreateResponse Response API 非流式转发
	//
	//	@param ctx context.Context
	//	@param ep UpstreamEndpoint
	//	@param body []byte
	//	@return []byte 原始响应体
	//	@return error
	ForwardCreateResponse(ctx context.Context, ep UpstreamEndpoint, body []byte) ([]byte, error)

	// ForwardCreateResponseStream Response API 流式转发，每个 SSE 事件调用 onEvent 回调
	//
	//	@param ctx context.Context
	//	@param ep UpstreamEndpoint
	//	@param body []byte
	//	@param onEvent func(event string, data []byte) error
	//	@return error
	ForwardCreateResponseStream(ctx context.Context, ep UpstreamEndpoint, body []byte, onEvent func(event string, data []byte) error) error
}

type openAIProxy struct{}

// NewOpenAIProxy 创建 OpenAI 代理
//
//	@return OpenAIProxy
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func NewOpenAIProxy() OpenAIProxy {
	return &openAIProxy{}
}

func (p *openAIProxy) ForwardChatCompletion(ctx context.Context, ep UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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

func (p *openAIProxy) ForwardChatCompletionStream(ctx context.Context, ep UpstreamEndpoint, body []byte, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var collectedChunks []*dto.OpenAIChatCompletionChunk

	reader := bufio.NewReader(resp.Body)
	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimRight(raw, "\r\n")

		if line != "" {
			if strings.HasPrefix(line, constant.SSEDataPrefix) {
				payload := line[len(constant.SSEDataPrefix):]
				if payload != constant.SSEDoneSignal {
					chunk := &dto.OpenAIChatCompletionChunk{}
					if err := sonic.UnmarshalString(payload, chunk); err != nil {
						log.Warn("[OpenAIProxy] Unmarshal sse chunk error", zap.String("payload", payload), zap.Error(err))
						continue
					}
					collectedChunks = append(collectedChunks, chunk)

					if err := onChunk(chunk); err != nil {
						log.Warn("[OpenAIProxy] OnChunk callback error", zap.Error(err))
						return nil, err
					}
				}
			}
		}

		if readErr != nil {
			if readErr != io.EOF {
				log.Warn("[OpenAIProxy] Read upstream sse error", zap.Error(readErr))
				return nil, &model.UpstreamConnectionError{Cause: readErr}
			}
			break
		}
	}

	if len(collectedChunks) == 0 {
		return nil, nil
	}

	return util.ConcatChatCompletionChunks(collectedChunks)
}

// sendRequest 构建并发送 OpenAI 协议的上游请求
func (p *openAIProxy) sendRequest(ctx context.Context, ep UpstreamEndpoint, body []byte) (*http.Response, error) {
	log := logger.WithCtx(ctx)

	body = util.EnsureAssistantMessageReasoningContent(body)

	upstreamURL := strings.TrimRight(ep.BaseURL, "/") + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		log.Error("[OpenAIProxy] New request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ep.APIKey)

	log.Info("[OpenAIProxy] Send upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", ep.Model),
		zap.String("upstreamAPIKey", util.MaskSecret(ep.APIKey)))

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		log.Error("[OpenAIProxy] Send http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		log.Error("[OpenAIProxy] Upstream returned non-200 status",
			zap.String("upstreamURL", upstreamURL),
			zap.Int("statusCode", resp.StatusCode),
			zap.String("responseBody", string(errorBody)),
		)
		return nil, &model.UpstreamError{StatusCode: resp.StatusCode, Body: string(errorBody)}
	}

	return resp, nil
}

func (p *openAIProxy) ForwardCreateResponse(ctx context.Context, ep UpstreamEndpoint, body []byte) ([]byte, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.sendResponseRequest(ctx, ep, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[OpenAIProxy] Read response api upstream response error", zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	return respBody, nil
}

func (p *openAIProxy) ForwardCreateResponseStream(ctx context.Context, ep UpstreamEndpoint, body []byte, onEvent func(event string, data []byte) error) error {
	log := logger.WithCtx(ctx)

	resp, err := p.sendResponseRequest(ctx, ep, body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	reader := bufio.NewReader(resp.Body)
	var currentEvent string

	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimRight(raw, "\r\n")

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

// sendResponseRequest 构建并发送 Response API 的上游请求
func (p *openAIProxy) sendResponseRequest(ctx context.Context, ep UpstreamEndpoint, body []byte) (*http.Response, error) {
	log := logger.WithCtx(ctx)

	upstreamURL := strings.TrimRight(ep.BaseURL, "/") + "/responses"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		log.Error("[OpenAIProxy] New response api request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "create response api request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ep.APIKey)

	log.Info("[OpenAIProxy] Send response api upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", ep.Model),
		zap.String("upstreamAPIKey", util.MaskSecret(ep.APIKey)))

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		log.Error("[OpenAIProxy] Send response api http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		log.Error("[OpenAIProxy] Response API upstream returned non-200 status",
			zap.String("upstreamURL", upstreamURL),
			zap.Int("statusCode", resp.StatusCode),
			zap.String("responseBody", string(errorBody)),
		)
		return nil, &model.UpstreamError{StatusCode: resp.StatusCode, Body: string(errorBody)}
	}

	return resp, nil
}
