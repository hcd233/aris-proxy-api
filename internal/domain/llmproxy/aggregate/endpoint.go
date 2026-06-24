package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonagg "github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
)

// Endpoint 上游端点聚合根
//
// 表示一个上游 LLM 服务的连接配置，包含 OpenAI 和 Anthropic 两个协议的 base URL、
// 共享 API Key、以及各接口的支持标记。
type Endpoint struct {
	commonagg.Base

	name                        string
	openaiBaseURL               string
	anthropicBaseURL            string
	apiKey                      string
	supportOpenAIChatCompletion bool
	supportOpenAIResponse       bool
	supportAnthropicMessage     bool
	createdAt                   time.Time
	updatedAt                   time.Time
}

// CreateEndpoint 构造 Endpoint 聚合根
func CreateEndpoint(
	id uint,
	name, openaiBaseURL, anthropicBaseURL, apiKey string,
	supportChatCompletion, supportResponse, supportMessage bool,
) (*Endpoint, error) {
	if name == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint name cannot be empty")
	}
	if apiKey == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint apiKey cannot be empty")
	}
	if openaiBaseURL == "" && anthropicBaseURL == "" {
		return nil, ierr.New(ierr.ErrValidation, "at least one base URL must be provided")
	}
	if !supportChatCompletion && !supportResponse && !supportMessage {
		return nil, ierr.New(ierr.ErrValidation, "at least one capability must be enabled")
	}
	if (supportChatCompletion || supportResponse) && openaiBaseURL == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint openai baseURL cannot be empty when OpenAI APIs are supported")
	}
	if supportMessage && anthropicBaseURL == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint anthropic baseURL cannot be empty when Anthropic messages API is supported")
	}
	ep := &Endpoint{
		name:                        name,
		openaiBaseURL:               openaiBaseURL,
		anthropicBaseURL:            anthropicBaseURL,
		apiKey:                      apiKey,
		supportOpenAIChatCompletion: supportChatCompletion,
		supportOpenAIResponse:       supportResponse,
		supportAnthropicMessage:     supportMessage,
	}
	ep.SetID(id)
	return ep, nil
}

func (e *Endpoint) Name() string                      { return e.name }
func (e *Endpoint) OpenaiBaseURL() string             { return e.openaiBaseURL }
func (e *Endpoint) AnthropicBaseURL() string          { return e.anthropicBaseURL }
func (e *Endpoint) APIKey() string                    { return e.apiKey }
func (e *Endpoint) SupportOpenAIChatCompletion() bool { return e.supportOpenAIChatCompletion }
func (e *Endpoint) SupportOpenAIResponse() bool       { return e.supportOpenAIResponse }
func (e *Endpoint) SupportAnthropicMessage() bool     { return e.supportAnthropicMessage }
func (e *Endpoint) CreatedAt() time.Time              { return e.createdAt }
func (e *Endpoint) UpdatedAt() time.Time              { return e.updatedAt }

func (e *Endpoint) SetTimestamps(createdAt, updatedAt time.Time) {
	e.createdAt = createdAt
	e.updatedAt = updatedAt
}

// Update 更新 Endpoint 字段（仅非 nil 字段更新）
func (e *Endpoint) Update(name, openaiBaseURL, anthropicBaseURL, apiKey *string, supportChatCompletion, supportResponse, supportMessage *bool) {
	if name != nil {
		e.name = *name
	}
	if openaiBaseURL != nil {
		e.openaiBaseURL = *openaiBaseURL
	}
	if anthropicBaseURL != nil {
		e.anthropicBaseURL = *anthropicBaseURL
	}
	if apiKey != nil {
		e.apiKey = *apiKey
	}
	if supportChatCompletion != nil {
		e.supportOpenAIChatCompletion = *supportChatCompletion
	}
	if supportResponse != nil {
		e.supportOpenAIResponse = *supportResponse
	}
	if supportMessage != nil {
		e.supportAnthropicMessage = *supportMessage
	}
}
