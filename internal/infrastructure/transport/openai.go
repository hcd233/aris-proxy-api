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

type openAIProxy struct{}

var _ usecase.OpenAIProxyPort = (*openAIProxy)(nil)

func NewOpenAIProxy() usecase.OpenAIProxyPort {
	return &openAIProxy{}
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

func (p *openAIProxy) ForwardChatCompletionStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.doUpstreamRequest(ctx, ep, body, constant.UpstreamPathOpenAIChatCompletions)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // ensure body closed on return

	var collectedChunks []*dto.OpenAIChatCompletionChunk

	reader := bufio.NewReader(resp.Body)
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

		chunk, skip := parseSSEDataLine(line)
		if skip || chunk == nil {
			continue
		}

		collectedChunks = append(collectedChunks, chunk)
		if err := onChunk(chunk); err != nil {
			log.Warn("[OpenAIProxy] OnChunk callback error", zap.Error(err))
			return nil, err
		}
	}

	if len(collectedChunks) == 0 {
		return nil, nil
	}

	return proxyutil.ConcatChatCompletionChunks(collectedChunks)
}

func parseSSEDataLine(line string) (chunk *dto.OpenAIChatCompletionChunk, skip bool) {
	if line == "" {
		skip = true
		return
	}
	if !strings.HasPrefix(line, constant.SSEDataPrefix) {
		skip = true
		return
	}
	payload := line[len(constant.SSEDataPrefix):]
	if payload == constant.SSEDoneSignal {
		return
	}
	chunk = &dto.OpenAIChatCompletionChunk{}
	if err := sonic.UnmarshalString(payload, chunk); err != nil {
		zap.L().Warn("[OpenAIProxy] Unmarshal sse chunk error", zap.String("payload", payload), zap.Error(err))
		skip = true
		return
	}
	return
}

// doUpstreamRequest 构建并发送上游 HTTP 请求的公共逻辑
func (p *openAIProxy) doUpstreamRequest(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, pathSuffix string) (*http.Response, error) {
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
		zap.ByteString("upstreamBody", body),
	)

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		log.Error("[OpenAIProxy] Send http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // read best effort on error path
		_ = resp.Body.Close()                 //nolint:errcheck // close best effort on error path
		log.Error("[OpenAIProxy] Upstream returned non-200 status",
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

func (p *openAIProxy) ForwardCreateResponseStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, onEvent func(event string, data []byte) error) error {
	log := logger.WithCtx(ctx)

	resp, err := p.doUpstreamRequest(ctx, ep, body, constant.UpstreamPathOpenAIResponses)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // ensure body closed on return

	reader := bufio.NewReader(resp.Body)
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
