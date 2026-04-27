package vo

// TokenPair 访问令牌对值对象（AccessToken + RefreshToken）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type TokenPair struct {
	accessToken  string
	refreshToken string
}

// NewTokenPair 构造令牌对值对象
//
//	@param accessToken string
//	@param refreshToken string
//	@return TokenPair
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewTokenPair(accessToken, refreshToken string) TokenPair {
	return TokenPair{accessToken: accessToken, refreshToken: refreshToken}
}

// AccessToken 返回访问令牌
//
//	@receiver p TokenPair
//	@return string
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (p TokenPair) AccessToken() string { return p.accessToken }

// RefreshToken 返回刷新令牌
//
//	@receiver p TokenPair
//	@return string
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (p TokenPair) RefreshToken() string { return p.refreshToken }

// IsEmpty 判断是否为空
//
//	@receiver p TokenPair
//	@return bool
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (p TokenPair) IsEmpty() bool { return p.accessToken == "" && p.refreshToken == "" }
