// Package dto 令牌DTO
package dto

// RefreshTokenReq represents a request to refresh an access token using a refresh token
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type RefreshTokenReq struct {
	Body *RefreshTokenReqBody `json:"body" doc:"Request body containing the refresh token"`
}

// RefreshTokenReqBody contains the refresh token used to obtain a new access token
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type RefreshTokenReqBody struct {
	RefreshToken string `json:"refreshToken" required:"true" minLength:"1" doc:"JWT refresh token used to obtain a new access token"`
}

// RefreshTokenRsp represents the response containing new access and refresh tokens
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type RefreshTokenRsp struct {
	CommonRsp
	AccessToken  string `json:"accessToken,omitempty" doc:"New JWT access token for API authentication"`
	RefreshToken string `json:"refreshToken,omitempty" doc:"New JWT refresh token for obtaining future access tokens"`
}
