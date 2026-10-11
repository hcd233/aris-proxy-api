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
	userID                      uint // 归属用户 ID（多租户隔离；由 repository 从 DB 恢复或 Create 时写入）
	openaiBaseURL               string
	anthropicBaseURL            string
	apiKey                      string
	supportOpenAIChatCompletion bool
	supportOpenAIResponse       bool
	supportAnthropicMessage     bool
	supportOpenAIDecision       bool
	createdAt                   time.Time
	updatedAt                   time.Time
}

// CreateEndpoint 构造 Endpoint 聚合根
func CreateEndpoint(
	id uint,
	name, openaiBaseURL, anthropicBaseURL, apiKey string,
	supportChatCompletion, supportResponse, supportMessage, supportDecision bool,
) (*Endpoint, error) {
	ep := newEndpoint(id, name, openaiBaseURL, anthropicBaseURL, apiKey, supportChatCompletion, supportResponse, supportMessage, supportDecision)
	if err := ep.validate(); err != nil {
		return nil, err
	}
	return ep, nil
}

// RehydrateEndpoint 从持久化状态恢复 Endpoint 聚合根，不做不变量校验。
//
// 专供仓储读路径：把校验型工厂用在恢复上，会让单行历史脏数据（例如
// supportAnthropicMessage=true 而 anthropicBaseURL 为空）变成读错误，
// 一次写坏就连锁打挂 upstream 列表、代理路由，以及自己的修复入口
// （PATCH 必须先 FindByID）。恢复是忠实拷贝，不变量由写路径
// CreateEndpoint / Update 保证。
func RehydrateEndpoint(
	id uint,
	name, openaiBaseURL, anthropicBaseURL, apiKey string,
	supportChatCompletion, supportResponse, supportMessage, supportDecision bool,
) *Endpoint {
	return newEndpoint(id, name, openaiBaseURL, anthropicBaseURL, apiKey, supportChatCompletion, supportResponse, supportMessage, supportDecision)
}

func newEndpoint(
	id uint,
	name, openaiBaseURL, anthropicBaseURL, apiKey string,
	supportChatCompletion, supportResponse, supportMessage, supportDecision bool,
) *Endpoint {
	ep := &Endpoint{
		name:                        name,
		openaiBaseURL:               openaiBaseURL,
		anthropicBaseURL:            anthropicBaseURL,
		apiKey:                      apiKey,
		supportOpenAIChatCompletion: supportChatCompletion,
		supportOpenAIResponse:       supportResponse,
		supportAnthropicMessage:     supportMessage,
		supportOpenAIDecision:       supportDecision,
	}
	ep.SetID(id)
	return ep
}

// validate 校验能力标记与对应 baseURL 的一致性；创建与更新共用同一套规则。
func (e *Endpoint) validate() error {
	if e.name == "" {
		return ierr.New(ierr.ErrValidation, "endpoint name cannot be empty")
	}
	if e.apiKey == "" {
		return ierr.New(ierr.ErrValidation, "endpoint apiKey cannot be empty")
	}
	if e.openaiBaseURL == "" && e.anthropicBaseURL == "" {
		return ierr.New(ierr.ErrValidation, "at least one base URL must be provided")
	}
	if !e.supportOpenAIChatCompletion && !e.supportOpenAIResponse && !e.supportAnthropicMessage && !e.supportOpenAIDecision {
		return ierr.New(ierr.ErrValidation, "at least one capability must be enabled")
	}
	if (e.supportOpenAIChatCompletion || e.supportOpenAIResponse || e.supportOpenAIDecision) && e.openaiBaseURL == "" {
		return ierr.New(ierr.ErrValidation, "endpoint openai baseURL cannot be empty when OpenAI APIs are supported")
	}
	if e.supportAnthropicMessage && e.anthropicBaseURL == "" {
		return ierr.New(ierr.ErrValidation, "endpoint anthropic baseURL cannot be empty when Anthropic messages API is supported")
	}
	return nil
}

func (e *Endpoint) Name() string                      { return e.name }
func (e *Endpoint) UserID() uint                      { return e.userID }
func (e *Endpoint) OpenaiBaseURL() string             { return e.openaiBaseURL }
func (e *Endpoint) AnthropicBaseURL() string          { return e.anthropicBaseURL }
func (e *Endpoint) APIKey() string                    { return e.apiKey }
func (e *Endpoint) SupportOpenAIChatCompletion() bool { return e.supportOpenAIChatCompletion }
func (e *Endpoint) SupportOpenAIResponse() bool       { return e.supportOpenAIResponse }
func (e *Endpoint) SupportAnthropicMessage() bool     { return e.supportAnthropicMessage }
func (e *Endpoint) SupportOpenAIDecision() bool       { return e.supportOpenAIDecision }
func (e *Endpoint) CreatedAt() time.Time              { return e.createdAt }
func (e *Endpoint) UpdatedAt() time.Time              { return e.updatedAt }

// SetUserID 设置归属用户 ID（repository 从 DB 恢复时使用）
func (e *Endpoint) SetUserID(userID uint) { e.userID = userID }

func (e *Endpoint) SetTimestamps(createdAt, updatedAt time.Time) {
	e.createdAt = createdAt
	e.updatedAt = updatedAt
}

// Update 更新 Endpoint 字段（仅非 nil 字段更新），并在落库前复核不变量。
//
// 先算出结果再校验、通过后才写回接收者：校验失败时不留下半更新实体。
// 仓储的 Update 会一次性写入全部字段（含能力标记），所以这里是唯一可能把
// 「能力已开、对应 baseURL 为空」写进库的地方——2026-10-12 tokenhub 事故
// 正是从这条缺校验的路径进入，之后该 endpoint 的每次读取都失败。
func (e *Endpoint) Update(name, openaiBaseURL, anthropicBaseURL, apiKey *string, supportChatCompletion, supportResponse, supportMessage, supportDecision *bool) error {
	next := *e
	if name != nil {
		next.name = *name
	}
	if openaiBaseURL != nil {
		next.openaiBaseURL = *openaiBaseURL
	}
	if anthropicBaseURL != nil {
		next.anthropicBaseURL = *anthropicBaseURL
	}
	if apiKey != nil {
		next.apiKey = *apiKey
	}
	if supportChatCompletion != nil {
		next.supportOpenAIChatCompletion = *supportChatCompletion
	}
	if supportResponse != nil {
		next.supportOpenAIResponse = *supportResponse
	}
	if supportMessage != nil {
		next.supportAnthropicMessage = *supportMessage
	}
	if supportDecision != nil {
		next.supportOpenAIDecision = *supportDecision
	}
	if err := next.validate(); err != nil {
		return err
	}
	*e = next
	return nil
}
