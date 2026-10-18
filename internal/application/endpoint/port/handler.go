// Package port defines application-layer ports for endpoint use cases.
package port

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// CreateEndpointCommand 创建 Endpoint 命令
//
// OwnerUserID 归属用户（handler 层计算：普通用户=自身，admin 可指定）。
type CreateEndpointCommand struct {
	OwnerUserID                 uint
	Name                        string
	OpenaiBaseURL               string
	AnthropicBaseURL            string
	APIKey                      string
	SupportOpenAIChatCompletion bool
	SupportOpenAIResponse       bool
	SupportAnthropicMessage     bool
	SupportOpenAIDecision       bool
}

// CreateEndpointResult 创建命令结果
type CreateEndpointResult struct {
	EndpointID uint
}

// CreateEndpointHandler 创建命令处理器
type CreateEndpointHandler interface {
	Handle(ctx context.Context, cmd CreateEndpointCommand) (*CreateEndpointResult, error)
}

// UpdateEndpointCommand 更新 Endpoint 命令
//
// ScopeUserID 多租户隔离三态：nil（admin）不过滤；非 nil（含共享池 0）精确匹配 user_id。
type UpdateEndpointCommand struct {
	ScopeUserID                 *uint
	EndpointID                  uint
	Name                        *string
	OpenaiBaseURL               *string
	AnthropicBaseURL            *string
	APIKey                      *string
	SupportOpenAIChatCompletion *bool
	SupportOpenAIResponse       *bool
	SupportAnthropicMessage     *bool
	SupportOpenAIDecision       *bool
}

// UpdateEndpointHandler 更新命令处理器
type UpdateEndpointHandler interface {
	Handle(ctx context.Context, cmd UpdateEndpointCommand) error
}

// DeleteEndpointCommand 删除 Endpoint 命令
//
// ScopeUserID 语义同 UpdateEndpointCommand。
type DeleteEndpointCommand struct {
	ScopeUserID *uint
	EndpointID  uint
}

// DeleteEndpointHandler 删除命令处理器
type DeleteEndpointHandler interface {
	Handle(ctx context.Context, cmd DeleteEndpointCommand) error
}

// AuthorizeOAuthCommand 发起订阅账号 OAuth 授权
//
// UserID 由 handler 层从 JWT 注入；授权流记录归属该用户，回调兑换时校验。
type AuthorizeOAuthCommand struct {
	UserID   uint
	Provider enum.OAuthProvider
}

// AuthorizeOAuthResult 授权发起结果（前端据此打开授权链接）
type AuthorizeOAuthResult struct {
	AuthorizeURL string
	State        string
}

// AuthorizeOAuthHandler 订阅账号授权发起处理器
type AuthorizeOAuthHandler interface {
	Handle(ctx context.Context, cmd AuthorizeOAuthCommand) (*AuthorizeOAuthResult, error)
}
