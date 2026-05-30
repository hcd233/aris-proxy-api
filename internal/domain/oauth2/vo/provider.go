// Package vo OAuth2 域值对象
package vo

import "github.com/hcd233/aris-proxy-api/internal/common/enum"

// OAuthProvider OAuth2 平台类型值对象
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type OAuthProvider string

// 平台常量
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
var (
	OAuthProviderGithub = OAuthProvider(enum.Oauth2PlatformGithub)
	OAuthProviderGoogle = OAuthProvider(enum.Oauth2PlatformGoogle)
)

// String 返回字符串形态
func (p OAuthProvider) String() string { return string(p) }

// IsValid 判断是否为支持的平台
//
//	@receiver p OAuthProvider
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (p OAuthProvider) IsValid() bool {
	return p == OAuthProviderGithub || p == OAuthProviderGoogle
}
