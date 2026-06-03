// Package port defines application-layer ports for oauth2 use cases.
package port

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
)

// InitiateLoginCommand 登录发起命令
type InitiateLoginCommand struct {
	Platform string
}

// InitiateLoginResult 登录发起结果
type InitiateLoginResult struct {
	RedirectURL string
}

// InitiateLoginHandler 登录发起命令处理器
type InitiateLoginHandler interface {
	Handle(ctx context.Context, cmd InitiateLoginCommand) (*InitiateLoginResult, error)
}

// HandleCallbackCommand 回调处理命令
type HandleCallbackCommand struct {
	Platform string
	Code     string
	State    string
}

// HandleCallbackResult 回调处理结果（供 handler 写响应）
type HandleCallbackResult struct {
	TokenPair *vo.TokenPair
	UserID    uint
	IsNewUser bool
}

// ObjectStorageDirCreator 对象存储目录创建器（跨域适配接口）
type ObjectStorageDirCreator interface {
	CreateDir(ctx context.Context, userID uint) error
}

// HandleCallbackHandler 回调命令处理器
type HandleCallbackHandler interface {
	Handle(ctx context.Context, cmd HandleCallbackCommand) (*HandleCallbackResult, error)
}
