package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/proxy"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

var upstreamHTTPClient = &http.Client{
	Timeout: 5 * time.Minute,
}

// OpenAIService OpenAI服务
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type OpenAIService interface {
	ListModels(ctx context.Context, req *dto.EmptyReq) (*dto.ListModelsResponse, error)
	// CreateChatCompletion 创建聊天补全
	//
	//	@param ctx context.Context
	//	@param req *dto.ChatCompletionRequestBody
	//	@return *ChatCompletionResult
	//	@return error
	CreateChatCompletion(ctx context.Context, req *dto.ChatCompletionRequest) (*huma.StreamResponse, error)
}

type openAIService struct{}

// NewOpenAIService 创建OpenAI服务
//
//	@return OpenAIService
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func NewOpenAIService() OpenAIService {
	return &openAIService{}
}

// ListModels 获取模型列表
//
//	@receiver s *openAIService
//	@param _ context.Context
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func (s *openAIService) ListModels(_ context.Context, _ *dto.EmptyReq) (*dto.ListModelsResponse, error) {
	rsp := &dto.ListModelsResponse{}

	config := proxy.GetLLMProxyConfig()

	rsp.Body = &dto.ListModelsResponseBody{
		Object: "list",
		Data: lo.MapToSlice(config.Models, func(key string, model proxy.ModelConfig) *dto.OpenAIModel {
			return &dto.OpenAIModel{
				ID:      model.Model,
				Created: time.Now().Unix(),
				Object:  "model",
				OwnedBy: key,
			}
		}),
	}

	return rsp, nil
}

// CreateChatCompletion 创建聊天补全
//
//	@receiver s *openAIService
//	@param _ context.Context
//	@param req *dto.ChatCompletionRequestBody
//	@return *ChatCompletionResult
//	@return error
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func (s *openAIService) CreateChatCompletion(ctx context.Context, req *dto.ChatCompletionRequest) (*huma.StreamResponse, error) {
	logger := logger.WithCtx(ctx)

	cfg := proxy.GetLLMProxyConfig()
	modelCfg, ok := cfg.Models[req.Body.Model]
	if !ok {
		logger.Error("[CreateChatCompletion] model not found", zap.String("model", req.Body.Model))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}
	// Build upstream request body as map to replace model name
	bodyBytes := lo.Must1(sonic.Marshal(req.Body))

	var bodyMap map[string]any
	if err := sonic.Unmarshal(bodyBytes, &bodyMap); err != nil {
		logger.Error("[CreateChatCompletion] unmarshal body error", zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}

	bodyMap["model"] = modelCfg.Model

	upstreamBody := lo.Must1(sonic.Marshal(bodyMap))
	upstreamURL := strings.TrimRight(modelCfg.BaseURL, "/") + "/chat/completions"

	upstreamReq, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(upstreamBody))
	if err != nil {
		logger.Error("[CreateChatCompletion] new request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+modelCfg.APIKey)

	logger.Info("[CreateChatCompletion] send upstream request", zap.String("upstreamURL", upstreamURL), 
	zap.String("upstreamModel", modelCfg.Model), 
	zap.Any("upstreamAPIKey", util.MaskSecret(modelCfg.APIKey)),
	 zap.Any("upstreamBody", bodyMap),)

	upstreamResp, err := upstreamHTTPClient.Do(upstreamReq)
	if err != nil {
		logger.Error("[CreateChatCompletion] send http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}

	if req.Body.Stream {
		return &huma.StreamResponse{
			Body: func(humaCtx huma.Context) {
				defer upstreamResp.Body.Close()

				fiberCtx := humafiber.Unwrap(humaCtx)
				humaCtx.SetStatus(upstreamResp.StatusCode)
				fiberCtx.Set("Content-Type", "text/event-stream")
				fiberCtx.Set("Cache-Control", "no-cache")
				fiberCtx.Set("Connection", "keep-alive")
				fiberCtx.Set("Transfer-Encoding", "chunked")

				fiberCtx.Response().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
					scanner := bufio.NewScanner(upstreamResp.Body)
					for scanner.Scan() {
						line := scanner.Text()
						if line == "" {
							continue
						}
						// Replace model name in SSE data lines
						const dataPrefix = "data: "
						if strings.HasPrefix(line, dataPrefix) {
							payload := line[len(dataPrefix):]
							if payload != "[DONE]" {
								chunk := &dto.ChatCompletionChunk{}
								err := sonic.UnmarshalString(payload, chunk)
								if err != nil {
									logger.Warn("[CreateChatCompletion] unmarshal sse chunk error", zap.Error(err))
									continue
								}
								chunk.Model = req.Body.Model
								line = fmt.Sprintf("%s%s", dataPrefix, lo.Must1(sonic.Marshal(chunk)))
							}
						}
						fmt.Fprintf(w, "%s\n\n", line)
						if err := w.Flush(); err != nil {
							logger.Warn("[CreateChatCompletion] flush sse error", zap.Error(err))
							return
						}
					}
				}))
			},
		}, nil
	}

	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			defer upstreamResp.Body.Close()

			respBody, err := io.ReadAll(upstreamResp.Body)
			if err != nil {
				humaCtx.SetStatus(http.StatusBadGateway)
				humaCtx.SetHeader("Content-Type", "application/json")
				humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.OpenAIErrorResponse{
					Error: &dto.OpenAIError{
						Message: "Failed to read upstream response",
						Type:    "server_error",
						Code:    "upstream_error",
					},
				})))
				return
			}

			humaCtx.SetStatus(upstreamResp.StatusCode)
			humaCtx.SetHeader("Content-Type", "application/json")

			// Replace model name in non-stream response
			completion := &dto.ChatCompletion{}
			err = sonic.Unmarshal(respBody, completion)
			if err != nil {
				logger.Warn("[CreateChatCompletion] unmarshal upstream response error", zap.Error(err))
				humaCtx.BodyWriter().Write(respBody)
			}
			completion.Model = req.Body.Model
			humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(completion)))
		},
	}, nil
}
