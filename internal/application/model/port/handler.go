// Package port defines application-layer ports for model use cases.
package port

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// CreateModelCommand 创建 Model 命令
//
// ModelID 为业务模型 ID（缺省 nil 时默认 = alias，与领域层 CreateModel 行为一致）。
type CreateModelCommand struct {
	ScopeUserID     *uint
	Alias           string
	ModelID         *string
	UpstreamModel   string
	EndpointID      uint
	ContextLength   int
	MaxOutputTokens int
	Capabilities    []enum.InputModality
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
//
// ID 为 Model 数据库主键（路由 id），ModelID 为业务模型 ID（默认=alias，可更新）。
type UpdateModelCommand struct {
	ScopeUserID     *uint
	ID              uint
	Alias           *string
	UpstreamModel   *string
	EndpointID      *uint
	Enabled         *bool
	ContextLength   *int
	MaxOutputTokens *int
	Capabilities    *[]enum.InputModality
	ModelID         *string
	SyncHistory     *bool // 为 true 且 ModelID 实际变化时，同步替换归属 user 的历史数据
}

// UpdateModelResult 更新结果（含历史同步影响行数；未同步时全 0）
type UpdateModelResult struct {
	AuditCount   int64
	SessionCount int64
	MessageCount int64
}

// UpdateModelHandler 更新命令处理器
type UpdateModelHandler interface {
	Handle(ctx context.Context, cmd UpdateModelCommand) (UpdateModelResult, error)
}

// DeleteModelCommand 删除 Model 命令
type DeleteModelCommand struct {
	ScopeUserID *uint
	ModelID     uint
}

// DeleteModelHandler 删除命令处理器
type DeleteModelHandler interface {
	Handle(ctx context.Context, cmd DeleteModelCommand) error
}
