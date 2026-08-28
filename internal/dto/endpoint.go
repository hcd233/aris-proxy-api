// Package dto Endpoint DTO
package dto

// CreateEndpointReq 创建 Endpoint 请求
type CreateEndpointReq struct {
	Body *CreateEndpointReqBody `json:"body" doc:"Request body"`
}

// CreateEndpointReqBody 创建 Endpoint 请求体
type CreateEndpointReqBody struct {
	OwnerUserID                 *uint   `json:"ownerUserID,omitempty" minimum:"1" doc:"归属用户ID(仅管理员生效，缺省为当前用户)"`
	Name                        string  `json:"name" required:"true" minLength:"1" maxLength:"64" doc:"Endpoint 名称"`
	OpenaiBaseURL               *string `json:"openaiBaseURL,omitempty" doc:"OpenAI Base URL"`
	AnthropicBaseURL            *string `json:"anthropicBaseURL,omitempty" doc:"Anthropic Base URL"`
	APIKey                      string  `json:"apiKey" required:"true" doc:"上游 API Key"`
	SupportOpenAIChatCompletion *bool   `json:"supportOpenAIChatCompletion,omitempty" doc:"是否支持 OpenAI Chat Completion"`
	SupportOpenAIResponse       *bool   `json:"supportOpenAIResponse,omitempty" doc:"是否支持 OpenAI Response"`
	SupportAnthropicMessage     *bool   `json:"supportAnthropicMessage,omitempty" doc:"是否支持 Anthropic Message"`
}

// UpdateEndpointReq 更新 Endpoint 请求
type UpdateEndpointReq struct {
	ID   uint                   `query:"id" required:"true" minimum:"1" doc:"Endpoint ID"`
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
	ID uint `query:"id" required:"true" minimum:"1" doc:"Endpoint ID"`
}
