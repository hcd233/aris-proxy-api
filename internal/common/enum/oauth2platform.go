package enum

type (
	// Oauth2Platform string 平台
	//	update 2024-09-21 01:34:12
	Oauth2Platform = string
)

const (
	// Oauth2PlatformGithub github user
	//	update 2024-06-22 10:05:13
	Oauth2PlatformGithub Oauth2Platform = "github"

	// Oauth2PlatformGoogle google user
	Oauth2PlatformGoogle Oauth2Platform = "google"
)
