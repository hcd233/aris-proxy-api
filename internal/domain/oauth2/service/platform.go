// Package service OAuth2 域领域服务
//
// 定义 OAuth2 平台策略接口 + State 管理器接口。具体实现在 internal/oauth2，
// Step 6 会搬迁到 internal/infrastructure/oauth2。
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
package service

import (
	"context"

	"golang.org/x/oauth2"

	"github.com/hcd233/aris-proxy-api/internal/domain/oauth2/vo"
)

// Platform OAuth2 平台策略接口
//
// 具体实现：GithubPlatform / GooglePlatform（internal/infrastructure/oauth2/*.go）
// 安全约束：必须通过 GetAuthURLWithState 携带一次性 state，防止 CSRF。
//
//	@author centonhuang
//	@update 2026-04-24 14:00:00
type Platform interface {
	// GetAuthURLWithState 获取携带一次性 state 的授权 URL
	GetAuthURLWithState(state string) string
	// ExchangeToken 通过授权码获取 Access Token
	ExchangeToken(ctx context.Context, code string) (*oauth2.Token, error)
	// GetUserInfo 获取用户信息
	GetUserInfo(ctx context.Context, token *oauth2.Token) (vo.OAuthUserInfo, error)
}

// StateManager OAuth2 State 管理器接口（防 CSRF）
//
//	@author centonhuang
//	@update 2026-04-26 14:00:00
type StateManager interface {
	// GenerateState 生成一个一次性 state
	GenerateState() (string, error)
	// VerifyState 校验 state 有效性并标记为已使用（一次性）
	// 返回 error 以区分「state 无效」和「存储故障」
	VerifyState(state string) error
}
