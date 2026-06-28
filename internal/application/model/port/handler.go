// Package port defines application-layer ports for model use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// CreateModelCommand 创建 Model 命令
type CreateModelCommand struct {
	Alias           string
	ModelName       string
	EndpointID      uint
	ContextLength   int
	MaxOutputTokens int
}

// CreateModelResult 创建命令结果
type CreateModelResult struct {
	ModelID uint
}

// CreateModelHandler 创建命令处理器
type CreateModelHandler interface {
	Handle(ctx context.Context, cmd CreateModelCommand) (*CreateModelResult, error)
}

// UpdateModelCommand 更新 Model 命令
type UpdateModelCommand struct {
	ModelID         uint
	Alias           *string
	ModelName       *string
	EndpointID      *uint
	Enabled         *bool
	ContextLength   *int
	MaxOutputTokens *int
}

// UpdateModelHandler 更新命令处理器
type UpdateModelHandler interface {
	Handle(ctx context.Context, cmd UpdateModelCommand) error
}

// DeleteModelCommand 删除 Model 命令
type DeleteModelCommand struct {
	ModelID uint
}

// DeleteModelHandler 删除命令处理器
type DeleteModelHandler interface {
	Handle(ctx context.Context, cmd DeleteModelCommand) error
}

// EndpointView Endpoint 只读投影（用于 ModelView 嵌套）
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

// ModelView Model 只读投影
type ModelView struct {
	ID              uint
	Alias           string
	ModelName       string
	Enabled         bool
	ContextLength   int
	MaxOutputTokens int
	Endpoint        *EndpointView
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ListModelsQuery 列出 Models 查询命令
type ListModelsQuery struct {
	model.CommonParam
}

// ListModelsHandler 查询处理器
type ListModelsHandler interface {
	Handle(ctx context.Context, q ListModelsQuery) ([]*ModelView, *model.PageInfo, error)
}
