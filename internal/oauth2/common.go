// Package oauth2 Oauth2
package oauth2

import (
	"context"

	"golang.org/x/oauth2"
)

// UserInfo 用户信息
type UserInfo interface {
	GetID() string
	GetName() string
	GetEmail() string
	GetAvatar() string
}

// Platform OAuth2提供商接口
type Platform interface {
	// GetAuthURL 获取授权URL
	GetAuthURL() string
	// ExchangeToken 通过授权码获取Access Token
	ExchangeToken(ctx context.Context, code string) (*oauth2.Token, error)
	// GetUserInfo 获取用户信息
	GetUserInfo(ctx context.Context, token *oauth2.Token) (UserInfo, error)
}
