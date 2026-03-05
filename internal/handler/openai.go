package handler

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// OpenAIHandler OpenAI兼容接口处理器
//
//	@author centonhuang
//	@update 2025-11-12 10:00:00
type OpenAIHandler interface {
	HandleListModels(ctx context.Context, req *dto.EmptyReq) (*dto.ListModelsResponse, error)
}

type openaiHandler struct {
	models []*dto.OpenAIModel
}

// NewOpenAIHandler 创建OpenAI兼容接口处理器
//
//	@return OpenAIHandler
//	@author centonhuang
//	@update 2025-11-12 10:00:00
func NewOpenAIHandler() OpenAIHandler {
	return &openaiHandler{
		models: loadModelsFromConfig(),
	}
}

// HandleListModels 获取模型列表
func (h *openaiHandler) HandleListModels(_ context.Context, _ *dto.EmptyReq) (*dto.ListModelsResponse, error) {
	rsp := &dto.ListModelsResponse{}
	rsp.Body.Object = "list"
	rsp.Body.Data = h.models
	return rsp, nil
}

// loadModelsFromConfig 从配置文件加载模型列表
func loadModelsFromConfig() []*dto.OpenAIModel {
	cfg := config.GetProxyConfig()
	now := time.Now().Unix()

	models := lo.MapToSlice(cfg.Models, func(key string, _ config.ModelConfig) *dto.OpenAIModel {
		return &dto.OpenAIModel{
			ID:      key,
			Created: now,
			Object:  "model",
			OwnedBy: "aris-proxy",
		}
	})

	return models
}
