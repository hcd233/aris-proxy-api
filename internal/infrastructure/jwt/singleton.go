package jwt

import "github.com/hcd233/aris-proxy-api/internal/config"

var (
	accessTokenSvc  *tokenSigner
	refreshTokenSvc *tokenSigner
)

// GetAccessTokenSigner 获取jwt access token服务
func GetAccessTokenSigner() TokenSigner {
	return accessTokenSvc
}

// GetRefreshTokenSigner 获取jwt refresh token服务
func GetRefreshTokenSigner() TokenSigner {
	return refreshTokenSvc
}

func init() {
	accessTokenSvc = &tokenSigner{
		JwtTokenSecret:  config.JwtAccessTokenSecret,
		JwtTokenExpired: config.JwtAccessTokenExpired,
	}

	refreshTokenSvc = &tokenSigner{
		JwtTokenSecret:  config.JwtRefreshTokenSecret,
		JwtTokenExpired: config.JwtRefreshTokenExpired,
	}
}
