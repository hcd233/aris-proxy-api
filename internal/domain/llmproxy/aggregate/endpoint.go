package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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
	authType                    enum.EndpointAuthType
	openaiBaseURL               string
	anthropicBaseURL            string
	apiKey                      string
	supportOpenAIChatCompletion bool
	supportOpenAIResponse       bool
	supportAnthropicMessage     bool
	supportOpenAIDecision       bool
	// oauthStatus 凭据状态只读投影：repository 读路径填充（零值 = 非 OAuth 或未加载）。
	// OAuth 端点凭据失效时解析层据此 fail-closed 剔除候选。
	oauthStatus enum.OAuthCredentialStatus
	createdAt   time.Time
	updatedAt   time.Time
}

// CreateEndpoint 构造 Endpoint 聚合根
func CreateEndpoint(
	id uint,
	name, openaiBaseURL, anthropicBaseURL, apiKey string,
	supportChatCompletion, supportResponse, supportMessage, supportDecision bool,
	authType enum.EndpointAuthType,
) (*Endpoint, error) {
	ep := newEndpoint(id, name, openaiBaseURL, anthropicBaseURL, apiKey, supportChatCompletion, supportResponse, supportMessage, supportDecision, authType)
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
	authType enum.EndpointAuthType,
) *Endpoint {
	return newEndpoint(id, name, openaiBaseURL, anthropicBaseURL, apiKey, supportChatCompletion, supportResponse, supportMessage, supportDecision, authType)
}

func newEndpoint(
	id uint,
	name, openaiBaseURL, anthropicBaseURL, apiKey string,
	supportChatCompletion, supportResponse, supportMessage, supportDecision bool,
	authType enum.EndpointAuthType,
) *Endpoint {
	ep := &Endpoint{
		name:                        name,
		authType:                    authType.Normalize(),
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
//
// OAuth 类型（订阅账号）先走 provider 专属规则：必须开启本 provider 的协议能力，
// 且沿用「能力已开、对应 baseURL 非空」的一致性检查（2026-10-12 tokenhub 事故
// 同类脏行必须挡在写路径外）；同时豁免 API Key 非空要求（凭据在 credentials 表）。
func (e *Endpoint) validate() error {
	if e.name == "" {
		return ierr.New(ierr.ErrValidation, "endpoint name cannot be empty")
	}
	switch e.authType {
	case enum.EndpointAuthOAuthAnthropic:
		if !e.supportAnthropicMessage {
			return ierr.New(ierr.ErrValidation, "oauth_anthropic endpoint must enable anthropic message capability")
		}
	case enum.EndpointAuthOAuthOpenAI:
		if !e.supportOpenAIResponse {
			return ierr.New(ierr.ErrValidation, "oauth_openai endpoint must enable openai response capability")
		}
	default:
	}
	if e.apiKey == "" && !e.authType.IsOAuth() {
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
func (e *Endpoint) AuthType() enum.EndpointAuthType   { return e.authType }
func (e *Endpoint) OpenaiBaseURL() string             { return e.openaiBaseURL }
func (e *Endpoint) AnthropicBaseURL() string          { return e.anthropicBaseURL }
func (e *Endpoint) APIKey() string                    { return e.apiKey }
func (e *Endpoint) SupportOpenAIChatCompletion() bool { return e.supportOpenAIChatCompletion }
func (e *Endpoint) SupportOpenAIResponse() bool       { return e.supportOpenAIResponse }
func (e *Endpoint) SupportAnthropicMessage() bool     { return e.supportAnthropicMessage }
func (e *Endpoint) SupportOpenAIDecision() bool       { return e.supportOpenAIDecision }
func (e *Endpoint) CreatedAt() time.Time              { return e.createdAt }
func (e *Endpoint) UpdatedAt() time.Time              { return e.updatedAt }

// SetOAuthStatus 设置凭据状态投影（repository 读路径填充）
func (e *Endpoint) SetOAuthStatus(status enum.OAuthCredentialStatus) { e.oauthStatus = status }

// OAuthStatus 返回凭据状态投影；零值表示非 OAuth 端点或状态未加载
func (e *Endpoint) OAuthStatus() enum.OAuthCredentialStatus { return e.oauthStatus }

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
	// OAuth 端点（订阅账号）的连接字段是服务端固定值：凭据在 credentials 表，
	// baseURL 指向官方端点，能力标记决定转发路由。放开修改会让端点静默失联。
	if e.authType.IsOAuth() {
		touchesConnection := openaiBaseURL != nil || anthropicBaseURL != nil || apiKey != nil
		touchesCapability := supportChatCompletion != nil || supportResponse != nil || supportMessage != nil || supportDecision != nil
		if touchesConnection || touchesCapability {
			return ierr.New(ierr.ErrValidation, "oauth endpoint connection fields are immutable")
		}
	}
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
