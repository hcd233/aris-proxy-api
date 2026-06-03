// Package port defines application-layer ports for identity use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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
