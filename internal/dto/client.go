// Package dto Client DTO
package dto

import (
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// ClientModelsReq 客户端拉取模型列表请求
type ClientModelsReq struct{}

// ClientModelsRsp 客户端拉取模型列表响应
type ClientModelsRsp struct {
	CommonRsp
	Models []*ClientModelItem `json:"models" doc:"启用中的模型列表（含能力与长度限制）"`
}

// ClientModelItem 客户端模型列表项（ModelItem 裁剪版）
type ClientModelItem struct {
	Alias           string               `json:"alias" doc:"模型别名"`
	UpstreamModel   string               `json:"upstreamModel" doc:"上游实际模型名"`
	ContextLength   int                  `json:"contextLength" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens int                  `json:"maxOutputTokens" doc:"最大输出长度（tokens）"`
	Capabilities    []enum.InputModality `json:"capabilities" doc:"模型能力（输入模态集合）"`
}
