package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/proxy"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// AnthropicService Anthropic服务
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicService interface {
	ListModels(ctx context.Context, req *dto.EmptyReq) (*dto.AnthropicListModelsRsp, error)
	CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error)
}

type anthropicService struct{}

// NewAnthropicService 创建Anthropic服务
//
//	@return AnthropicService
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func NewAnthropicService() AnthropicService {
	return &anthropicService{}
}

// ListModels 获取Anthropic模型列表
//
//	@receiver s *anthropicService
//	@param _ context.Context
//	@param _ *dto.EmptyReq
//	@return *dto.AnthropicListModelsRsp
//	@return error
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func (s *anthropicService) ListModels(_ context.Context, _ *dto.EmptyReq) (*dto.AnthropicListModelsRsp, error) {
	config := proxy.GetLLMProxyConfig()

	// Filter models that have an anthropic endpoint configured
	anthropicKeys := lo.Filter(lo.Keys(config.Models), func(key string, _ int) bool {
		mc := config.Models[key]
		_, hasAnthropic := mc.Endpoints[enum.ProviderAnthropic]
		return hasAnthropic
	})

	models := lo.Map(anthropicKeys, func(key string, _ int) *dto.AnthropicModelInfo {
		return &dto.AnthropicModelInfo{
			ID:          key,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			DisplayName: key,
			Type:        "model",
		}
	})

	rsp := &dto.AnthropicListModelsRsp{
		Data:    models,
		HasMore: false,
	}
	if len(models) > 0 {
		rsp.FirstID = models[0].ID
		rsp.LastID = models[len(models)-1].ID
	}
	return rsp, nil
}

// CreateMessage 创建Anthropic消息
//
//	@receiver s *anthropicService
//	@param ctx context.Context
//	@param req *dto.AnthropicCreateMessageRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func (s *anthropicService) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
	logger := logger.WithCtx(ctx)

	cfg := proxy.GetLLMProxyConfig()
	modelCfg, ok := cfg.Models[req.Body.Model]
	if !ok {
		logger.Error("[CreateMessage] model not found", zap.String("model", req.Body.Model))
		return util.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}

	endpoint, hasEndpoint := modelCfg.Endpoints[enum.ProviderAnthropic]
	if !hasEndpoint {
		logger.Error("[CreateMessage] model has no anthropic endpoint", zap.String("model", req.Body.Model))
		return util.SendAnthropicModelNotFoundError(req.Body.Model), nil
	}

	// Build upstream request body as map to replace model name
	bodyBytes := lo.Must1(sonic.Marshal(req.Body))

	var bodyMap map[string]any
	if err := sonic.Unmarshal(bodyBytes, &bodyMap); err != nil {
		logger.Error("[CreateMessage] unmarshal body error", zap.Error(err))
		return util.SendAnthropicInternalError(), nil
	}

	bodyMap["model"] = modelCfg.Model

	upstreamBody := lo.Must1(sonic.Marshal(bodyMap))
	upstreamURL := strings.TrimRight(endpoint.BaseURL, "/") + "/v1/messages"

	upstreamReq, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(upstreamBody))
	if err != nil {
		logger.Error("[CreateMessage] new request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return util.SendAnthropicInternalError(), nil
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("x-api-key", endpoint.APIKey)
	upstreamReq.Header.Set("anthropic-version", "2023-06-01")

	logger.Info("[CreateMessage] send upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", modelCfg.Model),
		zap.Any("upstreamAPIKey", util.MaskSecret(endpoint.APIKey)),
		zap.Any("upstreamBody", bodyMap))

	upstreamResp, err := upstreamHTTPClient.Do(upstreamReq)
	if err != nil {
		logger.Error("[CreateMessage] send http request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return util.SendAnthropicInternalError(), nil
	}

	exposedModel := req.Body.Model

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

					var collectedEvents []util.AnthropicSSEEvent
					var currentEvent string

					reader := bufio.NewReader(upstreamResp.Body)
					for {
						raw, readErr := reader.ReadString('\n')
						line := strings.TrimRight(raw, "\r\n")

						if line != "" {
							if strings.HasPrefix(line, "event: ") {
								currentEvent = strings.TrimPrefix(line, "event: ")
								// Forward event line as-is
								fmt.Fprintf(w, "%s\n", line)
							} else if strings.HasPrefix(line, "data: ") {
								payload := line[len("data: "):]

								// Try to replace model name in data
								var dataMap map[string]any
								if err := sonic.UnmarshalString(payload, &dataMap); err == nil {
									// Replace model in message_start event's nested message object
									if msgRaw, ok := dataMap["message"]; ok {
										if msgMap, ok := msgRaw.(map[string]any); ok {
											if _, hasModel := msgMap["model"]; hasModel {
												msgMap["model"] = exposedModel
											}
										}
									}
									// Also check top-level model field
									if _, hasModel := dataMap["model"]; hasModel {
										dataMap["model"] = exposedModel
									}
									modifiedPayload := lo.Must1(sonic.Marshal(dataMap))
									line = fmt.Sprintf("data: %s", string(modifiedPayload))

									// Collect event for message assembly
									collectedEvents = append(collectedEvents, util.AnthropicSSEEvent{
										Event: currentEvent,
										Data:  json.RawMessage(modifiedPayload),
									})
								}

								fmt.Fprintf(w, "%s\n\n", line)
								if err := w.Flush(); err != nil {
									logger.Warn("[CreateMessage] flush sse error", zap.Error(err))
									return
								}
							}
						}

						if readErr != nil {
							if readErr != io.EOF {
								logger.Warn("[CreateMessage] read upstream sse error", zap.Error(readErr))
							}
							break
						}
					}

					// Assemble complete message for storage
					if len(collectedEvents) == 0 {
						return
					}
					assembledMsg, err := util.ConcatAnthropicSSEEvents(collectedEvents)
					if err != nil {
						logger.Error("[CreateMessage] failed to assemble SSE events", zap.Error(err))
						return
					}
					if assembledMsg == nil || len(assembledMsg.Content) == 0 {
						logger.Warn("[CreateMessage] assembled message is empty")
						return
					}

					s.storeAnthropicMessages(ctx, logger, req, assembledMsg, modelCfg.Model)
				}))
			},
		}, nil
	}

	// Non-streaming response
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			defer upstreamResp.Body.Close()

			respBody, err := io.ReadAll(upstreamResp.Body)
			if err != nil {
				humaCtx.SetStatus(http.StatusBadGateway)
				humaCtx.SetHeader("Content-Type", "application/json")
				humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
					Type: "error",
					Error: &dto.AnthropicError{
						Type:    "api_error",
						Message: "Failed to read upstream response",
					},
				})))
				return
			}

			humaCtx.SetStatus(upstreamResp.StatusCode)
			humaCtx.SetHeader("Content-Type", "application/json")

			// Replace model name in non-stream response
			message := &dto.AnthropicMessage{}
			err = sonic.Unmarshal(respBody, message)
			if err != nil {
				logger.Warn("[CreateMessage] unmarshal upstream response error", zap.Error(err))
				humaCtx.BodyWriter().Write(respBody)
				return
			}
			message.Model = exposedModel
			humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(message)))

			s.storeAnthropicMessages(ctx, logger, req, message, modelCfg.Model)
		},
	}, nil
}

// storeAnthropicMessages 存储 Anthropic 消息到统一消息格式
func (s *anthropicService) storeAnthropicMessages(
	ctx context.Context,
	logger *zap.Logger,
	req *dto.AnthropicCreateMessageRequest,
	assistantMsg *dto.AnthropicMessage,
	upstreamModel string,
) {
	var unifiedMessages []*dto.UnifiedMessage

	// Convert request messages to UnifiedMessage
	for _, msg := range req.Body.Messages {
		um, err := dto.FromAnthropicMessage(msg)
		if err != nil {
			logger.Error("[storeAnthropicMessages] failed to convert anthropic message", zap.Error(err))
			return
		}
		unifiedMessages = append(unifiedMessages, um)
	}

	// Convert assistant response to UnifiedMessage
	aiMsg, err := dto.FromAnthropicResponse(assistantMsg)
	if err != nil {
		logger.Error("[storeAnthropicMessages] failed to convert anthropic response", zap.Error(err))
		return
	}
	unifiedMessages = append(unifiedMessages, aiMsg)

	// Convert request tools to UnifiedTool
	unifiedTools := make([]*dto.UnifiedTool, 0, len(req.Body.Tools))
	for _, tool := range req.Body.Tools {
		unifiedTools = append(unifiedTools, dto.FromAnthropicTool(tool))
	}

	if err := pool.GetPoolManager().SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:        util.CopyContextValues(ctx),
		APIKeyName: ctx.Value(constant.CtxKeyUserName).(string),
		Model:      upstreamModel,
		Messages:   unifiedMessages,
		Tools:      unifiedTools,
	}); err != nil {
		logger.Error("[CreateMessage] failed to submit message store task", zap.Error(err))
	}
}
