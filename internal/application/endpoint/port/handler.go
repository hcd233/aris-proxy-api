// Package port defines application-layer ports for endpoint use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// CreateEndpointCommand 创建 Endpoint 命令
type CreateEndpointCommand struct {
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
type UpdateEndpointCommand struct {
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
type DeleteEndpointCommand struct {
	EndpointID uint
}

// DeleteEndpointHandler 删除命令处理器
type DeleteEndpointHandler interface {
	Handle(ctx context.Context, cmd DeleteEndpointCommand) error
}

// EndpointView Endpoint 只读投影
type EndpointView struct {
	ID                          uint
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
type ListEndpointsQuery struct {
	model.CommonParam
}

// ListEndpointsHandler 查询处理器
type ListEndpointsHandler interface {
	Handle(ctx context.Context, q ListEndpointsQuery) ([]*EndpointView, *model.PageInfo, error)
}
