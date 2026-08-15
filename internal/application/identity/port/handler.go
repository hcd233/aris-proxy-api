// Package port defines application-layer ports for identity use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
)

// RefreshTokensCommand 刷新 token 对命令
type RefreshTokensCommand struct {
	RefreshToken string
}

// RefreshTokensHandler 刷新命令处理器
type RefreshTokensHandler interface {
	Handle(ctx context.Context, cmd RefreshTokensCommand) (*vo.TokenPair, error)
}

// UpdateProfileCommand 更新用户档案命令
type UpdateProfileCommand struct {
	UserID uint
	Name   string
	Email  string
	Avatar string
}

// UpdateProfileHandler 更新档案命令处理器
type UpdateProfileHandler interface {
	Handle(ctx context.Context, cmd UpdateProfileCommand) error
}

// UserView 用户详情只读投影
type UserView struct {
	ID         uint
	Name       string
	Email      string
	Avatar     string
	Permission enum.Permission
	CreatedAt  time.Time
	LastLogin  time.Time
}

// GetCurrentUserQuery 查询当前用户命令
type GetCurrentUserQuery struct {
	UserID uint
}

// GetCurrentUserHandler 查询处理器
type GetCurrentUserHandler interface {
	Handle(ctx context.Context, q GetCurrentUserQuery) (*UserView, error)
}

// ListUsersQuery 用户列表查询（管理员视图）
type ListUsersQuery struct {
	model.CommonParam
	Permission enum.Permission
}

// ListUsersHandler 用户列表查询处理器
type ListUsersHandler interface {
	Handle(ctx context.Context, q ListUsersQuery) ([]*UserView, *model.PageInfo, error)
}

// ApproveUserCommand 审核用户命令
type ApproveUserCommand struct {
	OperatorID uint // 操作者
	UserID     uint // 目标用户
}

// ApproveUserHandler 审核用户命令处理器
type ApproveUserHandler interface {
	Handle(ctx context.Context, cmd ApproveUserCommand) error
}

// DemoteUserCommand 降级用户命令
type DemoteUserCommand struct {
	OperatorID uint // 操作者
	UserID     uint // 目标用户
}

// DemoteUserHandler 降级用户命令处理器
type DemoteUserHandler interface {
	Handle(ctx context.Context, cmd DemoteUserCommand) error
}

// DeleteUserCommand 删除用户命令
type DeleteUserCommand struct {
	OperatorID uint // 操作者
	UserID     uint // 目标用户
}

// DeleteUserHandler 删除用户命令处理器
type DeleteUserHandler interface {
	Handle(ctx context.Context, cmd DeleteUserCommand) error
}

// SetDemoUserCommand 设置全局单例 Demo 账户命令
type SetDemoUserCommand struct {
	OperatorID uint // 操作者
	UserID     uint // 目标用户
}

// SetDemoUserHandler 设置 Demo 账户命令处理器
type SetDemoUserHandler interface {
	Handle(ctx context.Context, cmd SetDemoUserCommand) error
}

// RestoreDemoUserCommand 恢复 Demo 账户为普通用户命令
type RestoreDemoUserCommand struct {
	OperatorID uint // 操作者
	UserID     uint // 目标用户
}

// RestoreDemoUserHandler 恢复 Demo 账户命令处理器
type RestoreDemoUserHandler interface {
	Handle(ctx context.Context, cmd RestoreDemoUserCommand) error
}
