package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

var upstreamHTTPClient = &http.Client{
	Timeout: 5 * time.Minute,
}

// OpenAIService OpenAI服务
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type OpenAIService interface {
	// CreateChatCompletion 创建聊天补全
	//
	//	@param ctx context.Context
	//	@param req *dto.ChatCompletionRequestBody
	//	@return *ChatCompletionResult
	//	@return error
	CreateChatCompletion(ctx context.Context, req *dto.ChatCompletionRequestBody) (*ChatCompletionResult, error)
}

// ChatCompletionResult 聊天补全结果
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type ChatCompletionResult struct {
	// IsStream 是否为流式响应
	IsStream bool
	// UpstreamResponse 上游响应（非流式）
	UpstreamResponse *http.Response
	// UpstreamRequest 上游请求
	UpstreamRequest *http.Request
	// Error 错误信息
	Error *dto.OpenAIError
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

// CreateChatCompletion 创建聊天补全
//
//	@receiver s *openAIService
//	@param _ context.Context
//	@param req *dto.ChatCompletionRequestBody
//	@return *ChatCompletionResult
//	@return error
//	@author centonhuang
//	@update 2026-03-06 10:00:00
func (s *openAIService) CreateChatCompletion(_ context.Context, req *dto.ChatCompletionRequestBody) (*ChatCompletionResult, error) {
	cfg := config.GetLLMProxyConfig()
	modelCfg, ok := cfg.Models[req.Model]
	if !ok {
		return &ChatCompletionResult{
			Error: &dto.OpenAIError{
				Message: fmt.Sprintf("The model `%s` does not exist", req.Model),
				Type:    "invalid_request_error",
				Code:    "model_not_found",
			},
		}, nil
	}

	// Build upstream request body as map to replace model name
	bodyBytes, err := sonic.Marshal(req)
	if err != nil {
		return &ChatCompletionResult{
			Error: &dto.OpenAIError{
				Message: "Failed to marshal request body",
				Type:    "server_error",
				Code:    "internal_error",
			},
		}, nil
	}

	var bodyMap map[string]any
	if err := sonic.Unmarshal(bodyBytes, &bodyMap); err != nil {
		return &ChatCompletionResult{
			Error: &dto.OpenAIError{
				Message: "Failed to unmarshal request body",
				Type:    "server_error",
				Code:    "internal_error",
			},
		}, nil
	}

	bodyMap["model"] = modelCfg.Model

	upstreamBody, err := sonic.Marshal(bodyMap)
	if err != nil {
		return &ChatCompletionResult{
			Error: &dto.OpenAIError{
				Message: "Failed to marshal upstream body",
				Type:    "server_error",
				Code:    "internal_error",
			},
		}, nil
	}

	upstreamURL := strings.TrimRight(modelCfg.BaseURL, "/") + "/chat/completions"

	upstreamReq, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return &ChatCompletionResult{
			Error: &dto.OpenAIError{
				Message: "Failed to create upstream request",
				Type:    "server_error",
				Code:    "internal_error",
			},
		}, nil
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+modelCfg.APIKey)

	upstreamResp, err := upstreamHTTPClient.Do(upstreamReq)
	if err != nil {
		return &ChatCompletionResult{
			IsStream: req.Stream,
			Error: &dto.OpenAIError{
				Message: "Failed to reach upstream provider",
				Type:    "server_error",
				Code:    "upstream_error",
			},
		}, nil
	}

	return &ChatCompletionResult{
		IsStream:         req.Stream,
		UpstreamResponse: upstreamResp,
		UpstreamRequest:  upstreamReq,
	}, nil
}
