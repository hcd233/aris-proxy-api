package vo

// OAuthUserInfo 第三方 OAuth 平台返回的用户信息值对象
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type OAuthUserInfo struct {
	ID     string // 平台方的用户唯一 ID（thirdPartyID）
	Name   string
	Email  string
	Avatar string
}

// IsEmpty 判断是否为空
//
//	@receiver u OAuthUserInfo
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (u OAuthUserInfo) IsEmpty() bool { return u.ID == "" }
