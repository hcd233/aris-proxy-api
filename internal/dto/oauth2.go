// Package dto OAuth2 DTO
package dto

import "github.com/hcd233/aris-proxy-api/internal/common/enum"

// LoginReq represents a request to initiate OAuth2 login flow
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type LoginReq struct {
	Platform enum.Oauth2Platform `json:"platform" query:"platform" enum:"github,google" required:"true" doc:"OAuth2 platform name (github or google)"`
}

// LoginResp represents the response containing the OAuth2 authorization URL
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type LoginResp struct {
	CommonRsp
	RedirectURL string `json:"redirectURL,omitempty" doc:"URL to redirect the user to for OAuth2 authorization"`
}

// CallbackReq represents a request to handle OAuth2 callback with authorization code
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type CallbackReq struct {
	Body *CallbackReqBody `json:"body" doc:"Request body containing the authorization code and state"`
}

// CallbackReqBody contains the authorization code and state for OAuth2 callback
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type CallbackReqBody struct {
	Platform enum.Oauth2Platform `json:"platform" enum:"github,google" required:"true" doc:"OAuth2 platform name (github or google)"`
	Code     string              `json:"code" query:"code" required:"true" doc:"Authorization code returned by the OAuth2 platform"`
	State    string              `json:"state" query:"state" required:"true" doc:"State parameter for CSRF protection, must match the initial state"`
}

// CallbackRsp represents the response containing access and refresh tokens after successful OAuth2 authentication
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type CallbackRsp struct {
	CommonRsp
	AccessToken  string `json:"accessToken,omitempty" doc:"JWT access token for API authentication"`
	RefreshToken string `json:"refreshToken,omitempty" doc:"JWT refresh token for obtaining future access tokens"`
}
