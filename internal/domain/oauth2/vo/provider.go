// Package vo OAuth2 域值对象
package vo

import "github.com/hcd233/aris-proxy-api/internal/common/constant"

// OAuthProvider OAuth2 平台类型值对象
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type OAuthProvider string

// 平台常量 — 使用 var 以便从 constant 包做类型强转；变量语义实际上只读。
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
var (
	OAuthProviderGithub = OAuthProvider(constant.OAuthProviderGithub)
	OAuthProviderGoogle = OAuthProvider(constant.OAuthProviderGoogle)
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
