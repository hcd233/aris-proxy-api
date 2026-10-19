package model

// OAuthCredentialError 订阅账号凭据不可用（刷新失败/凭据缺失/需重新授权）。
//
// 语义上属于"该端点当前不可用"而非请求本身错误：由 usecase.CanSwitchEndpoint
// 判定为可切换，fallback 切到备选端点重试。
type OAuthCredentialError struct {
	Reason string
}

// Error 实现 error 接口
func (e *OAuthCredentialError) Error() string {
	return "oauth credential unavailable: " + e.Reason
}
