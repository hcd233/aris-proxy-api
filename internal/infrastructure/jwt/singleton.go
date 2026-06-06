package jwt

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/config"
)

func NewAccessTokenSigner() TokenSigner {
	return newTokenSigner(config.JwtAccessTokenSecret, config.JwtAccessTokenExpired)
}

func NewRefreshTokenSigner() TokenSigner {
	return newTokenSigner(config.JwtRefreshTokenSecret, config.JwtRefreshTokenExpired)
}

func newTokenSigner(secret string, expired time.Duration) TokenSigner {
	return &tokenSigner{
		JwtTokenSecret:  secret,
		JwtTokenExpired: expired,
	}
}
