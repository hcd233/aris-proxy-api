// Package dto Endpoint DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// CreateEndpointReq 创建 Endpoint 请求
type CreateEndpointReq struct {
	Body *CreateEndpointReqBody `json:"body" doc:"Request body"`
}

// CreateEndpointReqBody 创建 Endpoint 请求体
type CreateEndpointReqBody struct {
	Name                        string `json:"name" required:"true" minLength:"1" maxLength:"64" doc:"Endpoint 名称"`
	OpenaiBaseURL               string `json:"openaiBaseURL" doc:"OpenAI Base URL"`
	AnthropicBaseURL            string `json:"anthropicBaseURL" doc:"Anthropic Base URL"`
	APIKey                      string `json:"apiKey" required:"true" doc:"上游 API Key"`
	SupportOpenAIChatCompletion bool   `json:"supportOpenAIChatCompletion" doc:"是否支持 OpenAI Chat Completion"`
	SupportOpenAIResponse       bool   `json:"supportOpenAIResponse" doc:"是否支持 OpenAI Response"`
	SupportAnthropicMessage     bool   `json:"supportAnthropicMessage" doc:"是否支持 Anthropic Message"`
}

// UpdateEndpointReq 更新 Endpoint 请求
type UpdateEndpointReq struct {
	ID   uint                   `path:"id" required:"true" minimum:"1" doc:"Endpoint ID"`
	Body *UpdateEndpointReqBody `json:"body" doc:"Request body"`
}

// UpdateEndpointReqBody 更新 Endpoint 请求体
type UpdateEndpointReqBody struct {
	Name                        *string `json:"name,omitempty" doc:"Endpoint 名称"`
	OpenaiBaseURL               *string `json:"openaiBaseURL,omitempty" doc:"OpenAI Base URL"`
	AnthropicBaseURL            *string `json:"anthropicBaseURL,omitempty" doc:"Anthropic Base URL"`
	APIKey                      *string `json:"apiKey,omitempty" doc:"上游 API Key"`
	SupportOpenAIChatCompletion *bool   `json:"supportOpenAIChatCompletion,omitempty" doc:"是否支持 OpenAI Chat Completion"`
	SupportOpenAIResponse       *bool   `json:"supportOpenAIResponse,omitempty" doc:"是否支持 OpenAI Response"`
	SupportAnthropicMessage     *bool   `json:"supportAnthropicMessage,omitempty" doc:"是否支持 Anthropic Message"`
}

// DeleteEndpointReq 删除 Endpoint 请求
type DeleteEndpointReq struct {
	ID uint `path:"id" required:"true" minimum:"1" doc:"Endpoint ID"`
}

// ListEndpointsReq 列出 Endpoint 请求
//
//	@author centonhuang
//	@update 2026-05-27 10:00:00
type ListEndpointsReq struct {
	model.CommonParam
}

// ListEndpointsRsp 列出 Endpoint 响应
type ListEndpointsRsp struct {
	CommonRsp
	Endpoints []*EndpointItem `json:"endpoints,omitempty" doc:"Endpoint 列表"`
	PageInfo  *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// EndpointItem Endpoint 列表项
type EndpointItem struct {
	ID                          uint      `json:"id" doc:"Endpoint ID"`
	Name                        string    `json:"name" doc:"Endpoint 名称"`
	OpenaiBaseURL               string    `json:"openaiBaseURL" doc:"OpenAI Base URL"`
	AnthropicBaseURL            string    `json:"anthropicBaseURL" doc:"Anthropic Base URL"`
	MaskedAPIKey                string    `json:"maskedAPIKey" doc:"Masked API Key"`
	SupportOpenAIChatCompletion bool      `json:"supportOpenAIChatCompletion" doc:"是否支持 OpenAI Chat Completion"`
	SupportOpenAIResponse       bool      `json:"supportOpenAIResponse" doc:"是否支持 OpenAI Response"`
	SupportAnthropicMessage     bool      `json:"supportAnthropicMessage" doc:"是否支持 Anthropic Message"`
	CreatedAt                   time.Time `json:"createdAt" doc:"创建时间"`
	UpdatedAt                   time.Time `json:"updatedAt" doc:"更新时间"`
}
