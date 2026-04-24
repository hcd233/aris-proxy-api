package vo

// TokenPair 访问令牌对值对象（AccessToken + RefreshToken）
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// IsEmpty 判断是否为空
//
//	@receiver p TokenPair
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (p TokenPair) IsEmpty() bool { return p.AccessToken == "" && p.RefreshToken == "" }
