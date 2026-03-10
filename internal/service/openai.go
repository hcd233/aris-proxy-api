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
	ListModels(ctx context.Context, req *dto.EmptyReq) (*dto.ListModelsRsp, error)
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
func (s *openAIService) ListModels(_ context.Context, _ *dto.EmptyReq) (*dto.ListModelsRsp, error) {

	config := proxy.GetLLMProxyConfig()

	return &dto.ListModelsRsp{
		Object: "list",
		Data: lo.Map(lo.Keys(config.Models), func(key string, _ int) *dto.OpenAIModel {
			return &dto.OpenAIModel{
				ID:      key,
				Created: time.Now().Unix(),
				Object:  "model",
				OwnedBy: "openai",
			}
		}),
	}, nil
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
		zap.Any("upstreamBody", bodyMap))

	upstreamResp, err := upstreamHTTPClient.Do(upstreamReq)
	if err != nil {
		logger.Error("[CreateChatCompletion] send http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}

	if req.Body.Stream != nil && *req.Body.Stream {
		return &huma.StreamResponse{
			Body: func(humaCtx huma.Context) {
				fiberCtx := humafiber.Unwrap(humaCtx)
				fiberCtx.Set("Content-Type", "text/event-stream")
				fiberCtx.Set("Cache-Control", "no-cache")
				fiberCtx.Set("Connection", "keep-alive")
				fiberCtx.Set("Transfer-Encoding", "chunked")
				fiberCtx.Set("X-Accel-Buffering", "no")

				fiberCtx.Status(upstreamResp.StatusCode).Response().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
					defer upstreamResp.Body.Close()

					var collectedChunks []*dto.ChatCompletionChunk

					// Use bufio.Reader.ReadString instead of bufio.Scanner to avoid
					// batch pre-reading: ReadString blocks on I/O when upstream has no
					// data yet, naturally pacing writes to match the upstream token rate
					// and preventing multiple events from being coalesced (粘包).
					reader := bufio.NewReader(upstreamResp.Body)
					for {
						raw, readErr := reader.ReadString('\n')
						line := strings.TrimRight(raw, "\r\n")

						if line != "" {
							// Replace model name in SSE data lines
							const dataPrefix = "data: "
							if strings.HasPrefix(line, dataPrefix) {
								payload := line[len(dataPrefix):]
								if payload != "[DONE]" {
									chunk := &dto.ChatCompletionChunk{}
									if err := sonic.UnmarshalString(payload, chunk); err != nil {
										logger.Warn("[CreateChatCompletion] unmarshal sse chunk error", zap.Error(err))
									} else {
										chunk.Model = req.Body.Model
										collectedChunks = append(collectedChunks, chunk)
										line = fmt.Sprintf("%s%s", dataPrefix, lo.Must1(sonic.Marshal(chunk)))
									}
								}
							}
							fmt.Fprintf(w, "%s\n\n", line)
							if err := w.Flush(); err != nil {
								logger.Warn("[CreateChatCompletion] flush sse error", zap.Error(err))
								return
							}
						}

						if readErr != nil {
							if readErr != io.EOF {
								logger.Warn("[CreateChatCompletion] read upstream sse error", zap.Error(readErr))
							}

							// Merge all collected chunks and log the full response.
							if len(collectedChunks) > 0 {
								merged, mergeErr := util.ConcatChatCompletionChunks(collectedChunks)
								if mergeErr != nil {
									logger.Warn("[CreateChatCompletion] concat sse chunks error", zap.Error(mergeErr))
								} else {
									logger.Info("[CreateChatCompletion] merged sse response", zap.Any("merged", merged))
								}
							}
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
