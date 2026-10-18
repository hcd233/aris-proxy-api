package enum

// OAuthCredentialStatus 订阅账号 OAuth 凭据状态
type OAuthCredentialStatus string

const (
	// OAuthCredentialStatusActive 凭据可用（access token 可刷新、可转发）
	OAuthCredentialStatusActive OAuthCredentialStatus = "active"

	// OAuthCredentialStatusReauthRequired 刷新被上游拒绝（invalid_grant 等），需用户重新授权；
	// 该状态下端点从解析候选中剔除（fail-closed），由 fallback 切备选端点
	OAuthCredentialStatusReauthRequired OAuthCredentialStatus = "reauth_required"
)
