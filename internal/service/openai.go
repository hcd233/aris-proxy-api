package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// upstreamHTTPClient 上游 LLM 服务 HTTP 客户端
//
// Transport 细粒度超时配置：
//   - DialContext: 连接建立超时 10s
//   - TLSHandshakeTimeout: TLS 握手超时 10s
//   - ResponseHeaderTimeout: 等待响应头超时 30s（仅约束首字节，不影响流式读取）
//   - MaxIdleConns: 全局空闲连接上限 100
//   - MaxIdleConnsPerHost: 每个 host 空闲连接上限 20
//   - IdleConnTimeout: 空闲连接回收时间 90s
//
// Client.Timeout 保持 5min，因为 LLM 流式响应的总传输时长可能很长
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
var upstreamHTTPClient = &http.Client{
	Timeout: constant.HTTPClientTimeout,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   constant.HTTPDialTimeout,
			KeepAlive: constant.HTTPKeepAlive,
		}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   constant.HTTPTLSHandshakeTimeout,
		ResponseHeaderTimeout: constant.HTTPResponseHeaderTimeout,
		MaxIdleConns:          constant.HTTPMaxIdleConns,
		MaxIdleConnsPerHost:   constant.HTTPMaxIdleConnsPerHost,
		IdleConnTimeout:       constant.HTTPIdleConnTimeout,
		ForceAttemptHTTP2:     true,
	},
}

// OpenAIService OpenAI服务
//
//	@author centonhuang
//	@update 2026-04-04 10:00:00
type OpenAIService interface {
	ListModels(ctx context.Context, req *dto.EmptyReq) (*dto.ListModelsRsp, error)
	CreateChatCompletion(ctx context.Context, req *dto.ChatCompletionRequest) (*huma.StreamResponse, error)
}

type openAIService struct {
	modelEndpointDAO *dao.ModelEndpointDAO
}

// NewOpenAIService 创建OpenAI服务
//
//	@return OpenAIService
//	@author centonhuang
//	@update 2026-04-04 10:00:00
func NewOpenAIService() OpenAIService {
	return &openAIService{
		modelEndpointDAO: dao.GetModelEndpointDAO(),
	}
}

// ListModels 获取模型列表
//
//	@receiver s *openAIService
//	@param ctx context.Context
//	@param _ *dto.EmptyReq
//	@return *dto.ListModelsRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-04 10:00:00
func (s *openAIService) ListModels(ctx context.Context, _ *dto.EmptyReq) (*dto.ListModelsRsp, error) {
	db := database.GetDBInstance(ctx)

	endpoints, err := s.modelEndpointDAO.BatchGet(db, &dbmodel.ModelEndpoint{Provider: enum.ProviderOpenAI}, []string{"alias"})
	if err != nil {
		logger.WithCtx(ctx).Error("[OpenAIService] Failed to query model endpoints", zap.Error(err))
		return &dto.ListModelsRsp{Object: "list", Data: []*dto.OpenAIModel{}}, nil
	}

	return &dto.ListModelsRsp{
		Object: "list",
		Data: lo.Map(endpoints, func(ep *dbmodel.ModelEndpoint, _ int) *dto.OpenAIModel {
			return &dto.OpenAIModel{
				ID:      ep.Alias,
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
//	@param ctx context.Context
//	@param req *dto.ChatCompletionRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-04-04 10:00:00
func (s *openAIService) CreateChatCompletion(ctx context.Context, req *dto.ChatCompletionRequest) (*huma.StreamResponse, error) {
	logger := logger.WithCtx(ctx)

	db := database.GetDBInstance(ctx)
	endpoint, err := s.modelEndpointDAO.Get(db, &dbmodel.ModelEndpoint{
		Alias:    req.Body.Model,
		Provider: enum.ProviderOpenAI,
	}, []string{"model", "api_key", "base_url"})
	if err != nil {
		logger.Error("[OpenAIService] Model not found", zap.String("model", req.Body.Model), zap.Error(err))
		return util.SendOpenAIModelNotFoundError(req.Body.Model), nil
	}

	if req.Body.MaxTokens != nil {
		logger.Info("[OpenAIService] Adapt max_tokens to max_completion_tokens", zap.Intp("max_tokens", req.Body.MaxTokens))
		req.Body.MaxCompletionTokens, req.Body.MaxTokens = lo.ToPtr(*req.Body.MaxTokens), nil
	}
	// Build upstream request body as map to replace model name
	bodyBytes := lo.Must1(sonic.Marshal(req.Body))

	var bodyMap map[string]any
	if err := sonic.Unmarshal(bodyBytes, &bodyMap); err != nil {
		logger.Error("[OpenAIService] Unmarshal body error", zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}

	bodyMap["model"] = endpoint.Model

	upstreamBody := lo.Must1(sonic.Marshal(bodyMap))
	upstreamURL := strings.TrimRight(endpoint.BaseURL, "/") + "/chat/completions"

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(upstreamBody))
	if err != nil {
		logger.Error("[OpenAIService] New request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+endpoint.APIKey)

	logger.Info("[OpenAIService] Send upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", endpoint.Model),
		zap.Any("upstreamAPIKey", util.MaskSecret(endpoint.APIKey)),
		zap.Any("upstreamBody", bodyMap))

	upstreamResp, err := upstreamHTTPClient.Do(upstreamReq)
	if err != nil {
		logger.Error("[OpenAIService] Send http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return util.SendOpenAIInternalError(), nil
	}

	// 检查上游响应状态码，非200时记录详细错误信息
	if upstreamResp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(upstreamResp.Body)
		upstreamResp.Body.Close()
		logger.Error("[OpenAIService] Upstream returned non-200 status",
			zap.String("upstreamURL", upstreamURL),
			zap.Int("statusCode", upstreamResp.StatusCode),
			zap.String("responseBody", string(errorBody)),
			zap.Any("headers", upstreamResp.Header),
		)
		return util.SendOpenAIUpstreamError(upstreamResp.StatusCode, string(errorBody)), nil
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

					reader := bufio.NewReader(upstreamResp.Body)
					for {
						raw, readErr := reader.ReadString('\n')
						line := strings.TrimRight(raw, "\r\n")

						if line != "" {
							const dataPrefix = "data: "
							if strings.HasPrefix(line, dataPrefix) {
								payload := line[len(dataPrefix):]
								if payload != "[DONE]" {
									chunk := &dto.ChatCompletionChunk{}
									if err := sonic.UnmarshalString(payload, chunk); err != nil {
										logger.Warn("[OpenAIService] Unmarshal sse chunk error", zap.String("payload", payload), zap.Error(err))
										continue
									}

									chunk.Model = req.Body.Model
									collectedChunks = append(collectedChunks, chunk)
									line = fmt.Sprintf("%s%s", dataPrefix, lo.Must1(sonic.Marshal(chunk)))
								}
							}
							fmt.Fprintf(w, "%s\n\n", line)
							if err := w.Flush(); err != nil {
								logger.Warn("[OpenAIService] Flush sse error", zap.Error(err))
								return
							}
						}

						if readErr != nil {
							if readErr != io.EOF {
								logger.Warn("[OpenAIService] Read upstream sse error", zap.Error(readErr))
							}
							break
						}
					}

					if len(collectedChunks) == 0 {
						return
					}
					completion, err := util.ConcatChatCompletionChunks(collectedChunks)
					if err != nil {
						logger.Warn("[OpenAIService] Concat sse chunks error", zap.Error(err))
						return
					}
					if len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
						logger.Warn("[OpenAIService] AI response is empty", zap.Any("response", completion))
						return
					}

					s.storeOpenAIMessages(ctx, logger, req, completion.Choices[0].Message, endpoint.Model, completion.Usage)
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

			completion := &dto.ChatCompletion{}
			err = sonic.Unmarshal(respBody, completion)
			if err != nil {
				logger.Warn("[OpenAIService] Unmarshal upstream response error", zap.Error(err))
				humaCtx.BodyWriter().Write(respBody)
				return
			}
			completion.Model = req.Body.Model
			humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(completion)))

			if len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
				logger.Warn("[OpenAIService] AI response is empty", zap.Any("response", completion))
				return
			}

			s.storeOpenAIMessages(ctx, logger, req, completion.Choices[0].Message, endpoint.Model, completion.Usage)
		},
	}, nil
}

// storeOpenAIMessages 存储 OpenAI 消息到统一消息格式
func (s *openAIService) storeOpenAIMessages(
	ctx context.Context,
	logger *zap.Logger,
	req *dto.ChatCompletionRequest,
	assistantMsg *dto.ChatCompletionMessageParam,
	upstreamModel string,
	usage *dto.CompletionUsage,
) {
	var unifiedMessages []*dto.UnifiedMessage

	for _, msg := range req.Body.Messages {
		um, err := dto.FromOpenAIMessage(msg)
		if err != nil {
			logger.Error("[OpenAIService] Failed to convert openai message", zap.Error(err))
			return
		}
		unifiedMessages = append(unifiedMessages, um)
	}

	aiMsg, err := dto.FromOpenAIMessage(assistantMsg)
	if err != nil {
		logger.Error("[OpenAIService] Failed to convert ai response message", zap.Error(err))
		return
	}
	unifiedMessages = append(unifiedMessages, aiMsg)

	unifiedTools := lo.Map(req.Body.Tools, func(tool dto.ChatCompletionTool, _ int) *dto.UnifiedTool {
		return dto.FromOpenAITool(&tool)
	})

	var inputTokens, outputTokens int
	if usage != nil {
		inputTokens = usage.PromptTokens
		outputTokens = usage.CompletionTokens
	}

	if err := pool.GetPoolManager().SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:          util.CopyContextValues(ctx),
		APIKeyName:   util.CtxValueString(ctx, constant.CtxKeyUserName),
		Model:        upstreamModel,
		Messages:     unifiedMessages,
		Tools:        unifiedTools,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Client:       util.CtxValueString(ctx, constant.CtxKeyClient),
		Metadata:     req.Body.Metadata,
	}); err != nil {
		logger.Error("[OpenAIService] Failed to submit message store task", zap.Error(err))
	}
}
