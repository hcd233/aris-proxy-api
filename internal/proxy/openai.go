package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"go.uber.org/zap"
)

// OpenAIProxy OpenAI 协议上游代理
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
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
	logger := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("[OpenAIProxy] Read upstream response error", zap.Error(err))
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	completion := &dto.OpenAIChatCompletion{}
	if err := sonic.Unmarshal(respBody, completion); err != nil {
		logger.Warn("[OpenAIProxy] Unmarshal upstream response error", zap.Error(err))
		return nil, fmt.Errorf("unmarshal upstream response: %w", err)
	}

	return completion, nil
}

func (p *openAIProxy) ForwardChatCompletionStream(ctx context.Context, ep UpstreamEndpoint, body []byte, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	logger := logger.WithCtx(ctx)

	resp, err := p.sendRequest(ctx, ep, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var collectedChunks []*dto.OpenAIChatCompletionChunk

	reader := bufio.NewReader(resp.Body)
	for {
		raw, readErr := reader.ReadString('\n')
		line := strings.TrimRight(raw, "\r\n")

		if line != "" {
			const dataPrefix = "data: "
			if strings.HasPrefix(line, dataPrefix) {
				payload := line[len(dataPrefix):]
				if payload != "[DONE]" {
					chunk := &dto.OpenAIChatCompletionChunk{}
					if err := sonic.UnmarshalString(payload, chunk); err != nil {
						logger.Warn("[OpenAIProxy] Unmarshal sse chunk error", zap.String("payload", payload), zap.Error(err))
						continue
					}
					collectedChunks = append(collectedChunks, chunk)

					if err := onChunk(chunk); err != nil {
						logger.Warn("[OpenAIProxy] OnChunk callback error", zap.Error(err))
						return nil, err
					}
				}
			}
		}

		if readErr != nil {
			if readErr != io.EOF {
				logger.Warn("[OpenAIProxy] Read upstream sse error", zap.Error(readErr))
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
	logger := logger.WithCtx(ctx)

	upstreamURL := strings.TrimRight(ep.BaseURL, "/") + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		logger.Error("[OpenAIProxy] New request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ep.APIKey)

	logger.Info("[OpenAIProxy] Send upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", ep.Model),
		zap.Any("upstreamAPIKey", util.MaskSecret(ep.APIKey)))

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		logger.Error("[OpenAIProxy] Send http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		logger.Error("[OpenAIProxy] Upstream returned non-200 status",
			zap.String("upstreamURL", upstreamURL),
			zap.Int("statusCode", resp.StatusCode),
			zap.String("responseBody", string(errorBody)),
		)
		return nil, &model.UpstreamError{StatusCode: resp.StatusCode, Body: string(errorBody)}
	}

	return resp, nil
}
