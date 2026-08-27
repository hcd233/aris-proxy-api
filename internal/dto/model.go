// Package dto Model DTO
package dto

import (
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// CreateModelReq 创建 Model 请求
type CreateModelReq struct {
	Body *CreateModelReqBody `json:"body" doc:"Request body"`
}

// CreateModelReqBody 创建 Model 请求体
type CreateModelReqBody struct {
	Alias           string               `json:"alias" required:"true" minLength:"1" doc:"模型别名（对外暴露）"`
	ModelID         *string              `json:"modelId,omitempty" doc:"业务模型ID；缺省时默认等于 alias"`
	UpstreamModel   string               `json:"upstreamModel" required:"true" minLength:"1" doc:"上游实际模型名"`
	EndpointID      uint                 `json:"endpointID" required:"true" minimum:"1" doc:"关联 Endpoint ID"`
	ContextLength   int                  `json:"contextLength,omitempty" minimum:"0" default:"128000" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens int                  `json:"maxOutputTokens,omitempty" minimum:"0" default:"64000" doc:"最大输出长度（tokens）"`
	Capabilities    []enum.InputModality `json:"capabilities,omitempty" doc:"模型能力（输入模态集合；合法值 text/image；必须包含 text；缺省为 [text]）"`
}

// UpdateModelReq 更新 Model 请求
type UpdateModelReq struct {
	ID   uint                `query:"id" required:"true" minimum:"1" doc:"Model ID"`
	Body *UpdateModelReqBody `json:"body" doc:"Request body"`
}

// UpdateModelReqBody 更新 Model 请求体
type UpdateModelReqBody struct {
	Alias           *string               `json:"alias,omitempty" doc:"模型别名"`
	ModelID         *string               `json:"modelId,omitempty" doc:"业务模型ID(非空)"`
	UpstreamModel   *string               `json:"upstreamModel,omitempty" doc:"上游实际模型名"`
	EndpointID      *uint                 `json:"endpointID,omitempty" minimum:"1" doc:"关联 Endpoint ID"`
	Enabled         *bool                 `json:"enabled,omitempty" doc:"是否启用"`
	ContextLength   *int                  `json:"contextLength,omitempty" minimum:"0" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens *int                  `json:"maxOutputTokens,omitempty" minimum:"0" doc:"最大输出长度（tokens）"`
	Capabilities    *[]enum.InputModality `json:"capabilities,omitempty" doc:"模型能力（输入模态集合；合法值 text/image；必须包含 text）"`
}

// DeleteModelReq 删除 Model 请求
type DeleteModelReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"Model ID"`
}
