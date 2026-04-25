package vo

// OAuthUserInfo 第三方 OAuth 平台返回的用户信息值对象
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type OAuthUserInfo struct {
	id     string // 平台方的用户唯一 ID（thirdPartyID）
	name   string
	email  string
	avatar string
}

// NewOAuthUserInfo 构造 OAuth 用户信息值对象
//
//	@param id string
//	@param name string
//	@param email string
//	@param avatar string
//	@return OAuthUserInfo
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewOAuthUserInfo(id, name, email, avatar string) OAuthUserInfo {
	return OAuthUserInfo{id: id, name: name, email: email, avatar: avatar}
}

// ID 返回平台方用户唯一 ID
//
//	@receiver u OAuthUserInfo
//	@return string
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (u OAuthUserInfo) ID() string { return u.id }

// Name 返回用户名
//
//	@receiver u OAuthUserInfo
//	@return string
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (u OAuthUserInfo) Name() string { return u.name }

// Email 返回邮箱
//
//	@receiver u OAuthUserInfo
//	@return string
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (u OAuthUserInfo) Email() string { return u.email }

// Avatar 返回头像 URL
//
//	@receiver u OAuthUserInfo
//	@return string
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (u OAuthUserInfo) Avatar() string { return u.avatar }

// IsEmpty 判断是否为空
//
//	@receiver u OAuthUserInfo
//	@return bool
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (u OAuthUserInfo) IsEmpty() bool { return u.id == "" }
