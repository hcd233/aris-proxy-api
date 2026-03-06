package handler

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/proxy"
	"github.com/hcd233/aris-proxy-api/internal/service"
	"github.com/samber/lo"
)

// OpenAIHandler OpenAI兼容接口处理器
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type OpenAIHandler interface {
	HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.ListModelsResponse, error)
	HandleChatCompletion(ctx context.Context, req *dto.ChatCompletionRequest) (*huma.StreamResponse, error)
}

type openAIHandler struct {
	models []*dto.OpenAIModel
	svc    service.OpenAIService
}

// NewOpenAIHandler 创建OpenAI兼容接口处理器
//
//	@return OpenAIHandler
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func NewOpenAIHandler() OpenAIHandler {
	return &openAIHandler{
		models: loadModelsFromConfig(),
		svc:    service.NewOpenAIService(),
	}
}

// HandleListModels 获取模型列表
//
//	@receiver h *openAIHandler
//	@param _ context.Context
//	@param _ *dto.EmptyReq
//	@return *dto.ListModelsResponse
//	@return error
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func (h *openAIHandler) HandleListModels(_ context.Context, _ *dto.EmptyReq) (*dto.ListModelsResponse, error) {
	rsp := &dto.ListModelsResponse{}
	rsp.Body = &dto.ListModelsResponseBody{
		Object: "list",
		Data:   h.models,
	}
	return rsp, nil
}

// loadModelsFromConfig 从配置文件加载模型列表
//
//	@return []*dto.OpenAIModel
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func loadModelsFromConfig() []*dto.OpenAIModel {
	cfg := proxy.GetLLMProxyConfig()
	now := time.Now().Unix()

	models := lo.MapToSlice(cfg.Models, func(key string, _ proxy.ModelConfig) *dto.OpenAIModel {
		return &dto.OpenAIModel{
			ID:      key,
			Created: now,
			Object:  "model",
			OwnedBy: "aris-proxy",
		}
	})

	return models
}

// writeOpenAIError 写入OpenAI格式的错误响应
//
//	@param w io.Writer
//	@param message string
//	@param errType string
//	@param code string
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func writeOpenAIError(w io.Writer, message, errType, code string) {
	errResp := &dto.OpenAIErrorResponse{
		Error: &dto.OpenAIError{
			Message: message,
			Type:    errType,
			Code:    code,
		},
	}
	data, _ := sonic.Marshal(errResp)
	w.Write(data)
}

// HandleChatCompletion 处理聊天补全请求
//
//	@receiver h *openAIHandler
//	@param ctx context.Context
//	@param req *dto.ChatCompletionRequest
//	@return *huma.StreamResponse
//	@return error
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func (h *openAIHandler) HandleChatCompletion(ctx context.Context, req *dto.ChatCompletionRequest) (*huma.StreamResponse, error) {
	result, err := h.svc.CreateChatCompletion(ctx, &req.Body)
	if err != nil {
		return &huma.StreamResponse{
			Body: func(humaCtx huma.Context) {
				humaCtx.SetStatus(http.StatusInternalServerError)
				humaCtx.SetHeader("Content-Type", "application/json")
				writeOpenAIError(humaCtx.BodyWriter(), "Internal server error", "server_error", "internal_error")
			},
		}, nil
	}

	// 处理服务层返回的错误
	if result.Error != nil {
		statusCode := http.StatusInternalServerError
		if result.Error.Code == "model_not_found" {
			statusCode = http.StatusNotFound
		}
		return &huma.StreamResponse{
			Body: func(humaCtx huma.Context) {
				humaCtx.SetStatus(statusCode)
				humaCtx.SetHeader("Content-Type", "application/json")
				writeOpenAIError(humaCtx.BodyWriter(), result.Error.Message, result.Error.Type, result.Error.Code)
			},
		}, nil
	}

	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			fiberCtx := humafiber.Unwrap(humaCtx)

			if result.IsStream {
				handleStreamResponse(humaCtx, fiberCtx, result.UpstreamResponse)
			} else {
				handleNonStreamResponse(humaCtx, result.UpstreamResponse)
			}
		},
	}, nil
}

// handleStreamResponse 处理流式响应
//
//	@param humaCtx huma.Context
//	@param fiberCtx *fiber.Ctx
//	@param upstreamResp *http.Response
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func handleStreamResponse(humaCtx huma.Context, fiberCtx *fiber.Ctx, upstreamResp *http.Response) {
	defer upstreamResp.Body.Close()

	humaCtx.SetStatus(upstreamResp.StatusCode)
	fiberCtx.Set("Content-Type", "text/event-stream")
	fiberCtx.Set("Cache-Control", "no-cache")
	fiberCtx.Set("Connection", "keep-alive")
	fiberCtx.Set("Transfer-Encoding", "chunked")

	fiberCtx.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
		scanner := bufio.NewScanner(upstreamResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			fmt.Fprintf(w, "%s\n\n", line)
			if err := w.Flush(); err != nil {
				return
			}
		}
	})
}

// handleNonStreamResponse 处理非流式响应
//
//	@param humaCtx huma.Context
//	@param upstreamResp *http.Response
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func handleNonStreamResponse(humaCtx huma.Context, upstreamResp *http.Response) {
	defer upstreamResp.Body.Close()

	respBody, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		humaCtx.SetStatus(http.StatusBadGateway)
		humaCtx.SetHeader("Content-Type", "application/json")
		writeOpenAIError(humaCtx.BodyWriter(), "Failed to read upstream response", "server_error", "upstream_error")
		return
	}

	humaCtx.SetStatus(upstreamResp.StatusCode)
	humaCtx.SetHeader("Content-Type", "application/json")
	humaCtx.BodyWriter().Write(respBody)
}
