package constant

import "time"

const (
	OAuthProviderGithub = "github"
	OAuthProviderGoogle = "google"

	GithubUserURL      = "https://api.github.com/user"
	GithubUserEmailURL = "https://api.github.com/user/emails"
	GoogleUserInfoURL  = "https://www.googleapis.com/oauth2/v2/userinfo"

	DefaultUserNamePrefix = "ArisUser"

	OAuthStateBytes = 32

	PeriodOAuth2Callback = 5 * time.Second
	LimitOAuth2Callback  = 16

	OAuthStateManagerTTL = 10 * time.Minute
	OAuthStateMaxPending = 100
)
