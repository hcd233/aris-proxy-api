package oauth2

import (
	"context"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/oauth2/vo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

var githubUserScopes = []string{"user:email", "repo", "read:org"}

// GithubUserInfo Github用户信息结构体
type GithubUserInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// GetID 获取Github用户ID
//
//	@receiver u *GithubUserInfo
//	@return string
//	@author centonhuang
//	@update 2025-08-25 12:45:36
func (u *GithubUserInfo) GetID() string {
	return strconv.FormatInt(u.ID, 10)
}

// GetName 获取Github用户名
//
//	@receiver u *GithubUserInfo
//	@return string
//	@author centonhuang
//	@update 2025-08-25 12:45:38
func (u *GithubUserInfo) GetName() string {
	return u.Login
}

// GetEmail 获取Github用户邮箱
//
//	@receiver u *GithubUserInfo
//	@return string
//	@author centonhuang
//	@update 2025-08-25 12:45:41
func (u *GithubUserInfo) GetEmail() string {
	return u.Email
}

// GetAvatar 获取Github用户头像
//
//	@receiver u *GithubUserInfo
//	@return string
//	@author centonhuang
//	@update 2025-08-25 12:45:43
func (u *GithubUserInfo) GetAvatar() string {
	return u.AvatarURL
}

// GithubEmail Github邮箱信息结构体
type GithubEmail struct {
	Email   string `json:"email"`
	Primary bool   `json:"primary"`
}

// githubPlatform GitHub OAuth2提供商实现
type githubPlatform struct {
	oauth2Config *oauth2.Config
}

// NewGithubPlatform Github提供商
//
//	@return Platform
//	@author centonhuang
//	@update 2025-10-31 14:56:59
func NewGithubPlatform() Platform {
	return &githubPlatform{
		oauth2Config: &oauth2.Config{
			Endpoint:     github.Endpoint,
			Scopes:       githubUserScopes,
			ClientID:     config.Oauth2GithubClientID,
			ClientSecret: config.Oauth2GithubClientSecret,
			RedirectURL:  config.Oauth2GithubRedirectURL,
		},
	}
}

func (p *githubPlatform) GetAuthURLWithState(state string) string {
	return p.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *githubPlatform) ExchangeToken(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.oauth2Config.Exchange(ctx, code)
}

func (p *githubPlatform) GetUserInfo(ctx context.Context, token *oauth2.Token) (vo.OAuthUserInfo, error) {
	client := p.oauth2Config.Client(ctx, token)

	// 获取用户基本信息
	resp, err := client.Get(constant.GithubUserURL)
	if err != nil {
		return vo.NewOAuthUserInfo("", "", "", ""), err
	}
	defer func() { _ = resp.Body.Close() }()

	var userInfo GithubUserInfo
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return vo.NewOAuthUserInfo("", "", "", ""), err
	}

	// 获取用户邮箱信息
	emailResp, err := client.Get(constant.GithubUserEmailURL)
	if err != nil {
		return vo.NewOAuthUserInfo("", "", "", ""), err
	}
	defer func() { _ = emailResp.Body.Close() }()

	var emails []GithubEmail
	if err := sonic.ConfigDefault.NewDecoder(emailResp.Body).Decode(&emails); err != nil {
		return vo.NewOAuthUserInfo("", "", "", ""), err
	}

	// 选择主邮箱
	for _, email := range emails {
		if email.Primary {
			userInfo.Email = email.Email
			break
		}
	}

	return vo.NewOAuthUserInfo(
		userInfo.GetID(),
		userInfo.GetName(),
		userInfo.GetEmail(),
		userInfo.GetAvatar(),
	), nil
}
