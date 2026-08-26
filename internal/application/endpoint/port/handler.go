// Package port defines application-layer ports for endpoint use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
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
// ScopeUserID 多租户隔离：>0 时限定该用户；==0（admin）不过滤。
type UpdateEndpointCommand struct {
	ScopeUserID                 uint
	EndpointID                  uint
	Name                        *string
	OpenaiBaseURL               *string
	AnthropicBaseURL            *string
	APIKey                      *string
	SupportOpenAIChatCompletion *bool
	SupportOpenAIResponse       *bool
	SupportAnthropicMessage     *bool
}

// UpdateEndpointHandler 更新命令处理器
type UpdateEndpointHandler interface {
	Handle(ctx context.Context, cmd UpdateEndpointCommand) error
}

// DeleteEndpointCommand 删除 Endpoint 命令
//
// ScopeUserID 语义同 UpdateEndpointCommand。
type DeleteEndpointCommand struct {
	ScopeUserID uint
	EndpointID  uint
}

// DeleteEndpointHandler 删除命令处理器
type DeleteEndpointHandler interface {
	Handle(ctx context.Context, cmd DeleteEndpointCommand) error
}

// EndpointView Endpoint 只读投影
type EndpointView struct {
	ID                          uint
	Username                    string // 归属用户名（admin 全局视图辨认归属）
	Name                        string
	OpenaiBaseURL               string
	AnthropicBaseURL            string
	MaskedAPIKey                string
	SupportOpenAIChatCompletion bool
	SupportOpenAIResponse       bool
	SupportAnthropicMessage     bool
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

// ListEndpointsQuery 列出 Endpoints 查询命令
//
// ScopeUserID 多租户隔离：>0 时只返回该用户的配置；==0（admin 视角）不过滤。
type ListEndpointsQuery struct {
	model.CommonParam
	IsDemo      bool
	ScopeUserID uint
	Username    string // 仅 admin 视角生效：按归属用户名过滤
}

// ListEndpointsHandler 查询处理器
type ListEndpointsHandler interface {
	Handle(ctx context.Context, q ListEndpointsQuery) ([]*EndpointView, *model.PageInfo, error)
}
