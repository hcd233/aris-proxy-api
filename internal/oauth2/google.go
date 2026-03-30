package oauth2

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var googleUserScopes = []string{
	"openid",
	"profile",
	"email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/userinfo.email",
}

// GoogleUserInfo Google用户信息结构体
type GoogleUserInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	PhotoURL string `json:"picture"`
}

// GetID 获取Google用户ID
//
//	@receiver u *GoogleUserInfo
//	@return string
//	@author centonhuang
//	@update 2025-10-31 14:48:46
func (u *GoogleUserInfo) GetID() string {
	return u.ID
}

// GetName 获取Google用户名
//
//	@receiver u *GoogleUserInfo
//	@return string
//	@author centonhuang
//	@update 2025-10-31 14:48:48
func (u *GoogleUserInfo) GetName() string {
	return u.Name
}

// GetEmail 获取Google用户邮箱
//
//	@receiver u *GoogleUserInfo
//	@return string
//	@author centonhuang
//	@update 2025-10-31 14:48:50
func (u *GoogleUserInfo) GetEmail() string {
	return u.Email
}

// GetAvatar 获取Google用户头像
//
//	@receiver u *GoogleUserInfo
//	@return string
//	@author centonhuang
//	@update 2025-10-31 14:48:52
func (u *GoogleUserInfo) GetAvatar() string {
	return u.PhotoURL
}

// googlePlatform Google OAuth2提供商实现
type googlePlatform struct {
	oauth2Config *oauth2.Config
}

// NewGooglePlatform Google提供商
//
//	@return Platform
//	@author centonhuang
//	@update 2025-10-31 14:57:11
func NewGooglePlatform() Platform {
	return &googlePlatform{
		oauth2Config: &oauth2.Config{
			Endpoint:     google.Endpoint,
			Scopes:       googleUserScopes,
			ClientID:     config.Oauth2GoogleClientID,
			ClientSecret: config.Oauth2GoogleClientSecret,
			RedirectURL:  config.Oauth2GoogleRedirectURL,
		},
	}
}

func (p *googlePlatform) GetAuthURL() string {
	return p.oauth2Config.AuthCodeURL(config.Oauth2StateString, oauth2.AccessTypeOffline)
}

func (p *googlePlatform) GetAuthURLWithState(state string) string {
	return p.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *googlePlatform) ExchangeToken(ctx context.Context, code string) (*oauth2.Token, error) {
	logger := logger.WithCtx(ctx)

	logger.Info("[GoogleOauth2] Exchanging code for token",
		zap.String("clientID", p.oauth2Config.ClientID),
		zap.String("redirectURL", p.oauth2Config.RedirectURL),
		zap.Strings("scopes", p.oauth2Config.Scopes))

	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		logger.Error("[GoogleOauth2] Token exchange failed", zap.Error(err))
		return nil, err
	}

	logger.Info("[GoogleOauth2] Token exchange successful")
	return token, nil
}

func (p *googlePlatform) GetUserInfo(ctx context.Context, token *oauth2.Token) (UserInfo, error) {
	logger := logger.WithCtx(ctx)

	// 使用HTTP客户端直接调用Google OAuth2 UserInfo API
	client := p.oauth2Config.Client(ctx, token)

	logger.Info("[GoogleOauth2] Calling Google UserInfo API")

	// 调用Google UserInfo API
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		logger.Error("[GoogleOauth2] Failed to call userinfo API", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	logger.Info("[GoogleOauth2] Userinfo API response",
		zap.Int("statusCode", resp.StatusCode))

	var userInfoResp struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}

	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&userInfoResp); err != nil {
		logger.Error("[GoogleOauth2] Failed to decode userinfo response", zap.Error(err))
		return nil, err
	}

	logger.Info("[GoogleOauth2] Successfully decoded user info",
		zap.String("userID", userInfoResp.ID),
		zap.String("userName", userInfoResp.Name),
		zap.String("userEmail", userInfoResp.Email))

	userInfo := &GoogleUserInfo{
		ID:       userInfoResp.ID,
		Name:     userInfoResp.Name,
		Email:    userInfoResp.Email,
		PhotoURL: userInfoResp.Picture,
	}

	return userInfo, nil
}
