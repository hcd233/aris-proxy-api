// Package dto 订阅账号 OAuth DTO
package dto

import (
	"time"
)

// OAuthQuotaWindow DTO 层额度窗口（transport 契约；不直接引用 domain vo）
type OAuthQuotaWindow struct {
	Key           string    `json:"key" doc:"窗口标识（five_hour/seven_day/primary/secondary）"`
	UsedPercent   float64   `json:"usedPercent" doc:"已用百分比"`
	ResetsAt      time.Time `json:"resetsAt" doc:"窗口重置时间"`
	WindowMinutes int       `json:"windowMinutes" doc:"窗口时长（分钟）"`
}

// OAuthQuotaSnapshot DTO 层额度快照
type OAuthQuotaSnapshot struct {
	Windows    []OAuthQuotaWindow `json:"windows" doc:"窗口用量列表"`
	PlanType   string             `json:"planType,omitempty" doc:"订阅类型"`
	CapturedAt time.Time          `json:"capturedAt" doc:"采集时间"`
}

// AuthorizeOAuthReq 发起订阅账号授权请求
type AuthorizeOAuthReq struct {
	Body *AuthorizeOAuthReqBody `json:"body" doc:"Request body"`
}

// AuthorizeOAuthReqBody 发起授权请求体
type AuthorizeOAuthReqBody struct {
	Provider string `json:"provider" required:"true" enum:"anthropic,openai" doc:"OAuth 提供方（anthropic=Claude Code / openai=Codex）"`
}

// AuthorizeOAuthRspBody 发起授权响应体
type AuthorizeOAuthRspBody struct {
	AuthorizeURL string `json:"authorizeURL" doc:"授权链接（前端新窗口打开）"`
	State        string `json:"state" doc:"授权流标识（回贴 code 时随附）"`
}

// ExchangeOAuthReq 完成订阅账号绑定请求
type ExchangeOAuthReq struct {
	Body *ExchangeOAuthReqBody `json:"body" doc:"Request body"`
}

// ExchangeOAuthReqBody 绑定请求体
type ExchangeOAuthReqBody struct {
	OwnerUserID *uint   `json:"ownerUserID,omitempty" minimum:"1" doc:"归属用户ID(仅管理员生效，缺省为当前用户)"`
	Name        string  `json:"name" required:"true" minLength:"1" maxLength:"64" doc:"端点名称"`
	Provider    string  `json:"provider" required:"true" enum:"anthropic,openai" doc:"OAuth 提供方"`
	Code        string  `json:"code" required:"true" doc:"授权码（Claude 贴页面显示的 code#state；Codex 贴浏览器地址栏整段回调 URL）"`
	State       *string `json:"state,omitempty" doc:"授权流标识（code 未内嵌 state 时使用）"`
}

// ExchangeOAuthRspBody 绑定响应体
type ExchangeOAuthRspBody struct {
	EndpointID uint `json:"endpointID" doc:"创建的订阅端点 ID"`
}

// ReauthOAuthReq 重新授权请求
type ReauthOAuthReq struct {
	ID   uint                `query:"id" required:"true" minimum:"1" doc:"Endpoint ID"`
	Body *ReauthOAuthReqBody `json:"body" doc:"Request body"`
}

// ReauthOAuthReqBody 重新授权请求体
type ReauthOAuthReqBody struct {
	Code  string  `json:"code" required:"true" doc:"授权码（同绑定格式）"`
	State *string `json:"state,omitempty" doc:"授权流标识（code 未内嵌 state 时使用）"`
}

// GetOAuthQuotaReq 查询订阅端点额度请求
type GetOAuthQuotaReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"Endpoint ID"`
}

// GetOAuthQuotaRspBody 额度查询响应体（凭据本身永不下发，仅状态与用量快照）
type GetOAuthQuotaRspBody struct {
	Status         string              `json:"status" doc:"凭据状态（active/reauth_required）"`
	PlanType       string              `json:"planType,omitempty" doc:"订阅类型"`
	LastError      string              `json:"lastError,omitempty" doc:"最近错误（脱敏展示）"`
	Quota          *OAuthQuotaSnapshot `json:"quota,omitempty" doc:"额度快照"`
	QuotaUpdatedAt *time.Time          `json:"quotaUpdatedAt,omitempty" doc:"额度快照采集时间"`
}
