package oauthprovider

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
)

// ──────────────────────────── Codex（OpenAI）────────────────────────────

// openaiOAuthClient Codex 订阅 OAuth 客户端
type openaiOAuthClient struct {
	authorizeURL string
	tokenURL     string
	usageURL     string
}

// NewOpenAIClient 构造 Codex 订阅客户端（默认端点）
func NewOpenAIClient() service.OAuthProviderClient {
	return &openaiOAuthClient{
		authorizeURL: constant.OAuthOpenAIAuthorizeURL,
		tokenURL:     constant.OAuthOpenAITokenURL,
		usageURL:     constant.OAuthOpenAIUsageURL,
	}
}

// Provider 返回提供方标识
func (c *openaiOAuthClient) Provider() enum.OAuthProvider { return enum.OAuthProviderOpenAI }

// AuthorizeURL 生成 OpenAI 授权链接（PKCE S256 + state + 组织信息与简化流开关）
func (c *openaiOAuthClient) AuthorizeURL(challenge, state string) string {
	query := url.Values{}
	query.Set(constant.OAuthParamClientID, constant.OAuthOpenAIClientID)
	query.Set(constant.OAuthParamResponseType, constant.OAuthParamValueAuthorizationCode)
	query.Set(constant.OAuthParamRedirectURI, constant.OAuthOpenAIRedirectURI)
	query.Set(constant.OAuthParamScope, constant.OAuthOpenAIScope)
	query.Set(constant.OAuthParamCodeChallenge, challenge)
	query.Set(constant.OAuthParamCodeChallengeMethod, constant.OAuthParamValueS256)
	query.Set(constant.OAuthCallbackParamState, state)
	query.Set(constant.OAuthOpenAIAddOrganizationsParam, constant.OAuthOpenAIBoolTrue)
	query.Set(constant.OAuthOpenAISimplifiedFlowParam, constant.OAuthOpenAIBoolTrue)
	return c.authorizeURL + "?" + query.Encode()
}

// openaiTokenResult OpenAI token 响应（id_token 携带账号信息）
type openaiTokenResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	IDToken      string `json:"id_token"`
}

// openaiAuthClaims id_token 的 OpenAI 认证声明块
type openaiAuthClaims struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	ChatGPTPlanType  string `json:"chatgpt_plan_type"`
}

// openaiIDTokenClaims id_token 载荷（只解码不验签：与官方 CLI 行为一致，
// 自用场景信任 TLS 通道；此处不做签名校验是明确取舍）
type openaiIDTokenClaims struct {
	Auth  openaiAuthClaims `json:"https://api.openai.com/auth"`
	Email string           `json:"email"`
}

// parseOpenAIIDToken 解码 id_token 载荷（base64url 第二段）
func parseOpenAIIDToken(idToken string) (*openaiIDTokenClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != constant.OAuthIDTokenSegmentCount {
		return nil, ierr.New(ierr.ErrValidation, "invalid id_token format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "decode id_token payload")
	}
	var claims openaiIDTokenClaims
	if err := sonic.Unmarshal(payload, &claims); err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "unmarshal id_token claims")
	}
	return &claims, nil
}

// ExchangeCode 用授权码换 token（form 编码；state 已由服务端 flow 校验，不下发）
func (c *openaiOAuthClient) ExchangeCode(ctx context.Context, code, verifier, _ string) (*service.OAuthTokenSet, error) {
	form := url.Values{}
	form.Set(constant.OAuthParamGrantType, constant.OAuthParamValueAuthorizationCode)
	form.Set(constant.OAuthParamClientID, constant.OAuthOpenAIClientID)
	form.Set(constant.OAuthCallbackParamCode, code)
	form.Set(constant.OAuthParamRedirectURI, constant.OAuthOpenAIRedirectURI)
	form.Set(constant.OAuthParamCodeVerifier, verifier)
	return c.postTokenForm(ctx, form)
}

// Refresh 刷新 access token（scope 不含 offline_access）
func (c *openaiOAuthClient) Refresh(ctx context.Context, refreshToken string) (*service.OAuthTokenSet, error) {
	form := url.Values{}
	form.Set(constant.OAuthParamGrantType, constant.OAuthParamValueRefreshToken)
	form.Set(constant.OAuthParamClientID, constant.OAuthOpenAIClientID)
	form.Set(constant.OAuthParamRefreshToken, refreshToken)
	form.Set(constant.OAuthParamScope, constant.OAuthOpenAIRefreshScope)
	return c.postTokenForm(ctx, form)
}

// postTokenForm 发送 form token 请求并映射响应（携带 Codex 客户端身份头）
func (c *openaiOAuthClient) postTokenForm(ctx context.Context, form url.Values) (*service.OAuthTokenSet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "build openai token request")
	}
	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeFormURLEncoded)
	req.Header.Set(constant.HTTPHeaderUserAgent, constant.OAuthCodexUserAgent)
	req.Header.Set(constant.HTTPHeaderOriginator, constant.OAuthCodexOriginator)

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "openai oauth token request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // 只读响应，关闭失败无副作用

	if resp.StatusCode != http.StatusOK {
		return nil, protocolError(resp)
	}

	var parsed openaiTokenResult
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "decode openai token response")
	}
	if parsed.AccessToken == "" {
		return nil, ierr.New(ierr.ErrProxyRequest, "openai token response missing access_token")
	}

	set := &service.OAuthTokenSet{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second),
	}
	if parsed.IDToken != "" {
		claims, claimsErr := parseOpenAIIDToken(parsed.IDToken)
		if claimsErr == nil {
			set.AccountRef = claims.Auth.ChatGPTAccountID
			set.AccountEmail = claims.Email
			set.PlanType = claims.Auth.ChatGPTPlanType
		}
	}
	return set, nil
}

// openaiUsageWindow Codex 单窗口用量
type openaiUsageWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	ResetAt            string  `json:"reset_at"`
}

// openaiRateLimit Codex 速率窗口组
type openaiRateLimit struct {
	PrimaryWindow   *openaiUsageWindow `json:"primary_window"`
	SecondaryWindow *openaiUsageWindow `json:"secondary_window"`
}

// openaiUsageResult Codex 订阅用量响应
type openaiUsageResult struct {
	RateLimit openaiRateLimit `json:"rate_limit"`
}

// ProbeQuota 探测 Codex 订阅窗口用量（primary 5h / secondary 7d）
func (c *openaiOAuthClient) ProbeQuota(ctx context.Context, accessToken, accountRef string) (*vo.QuotaSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.usageURL, http.NoBody)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "build openai usage request")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+accessToken)
	req.Header.Set(constant.HTTPHeaderChatGPTAccountID, accountRef)

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "openai oauth usage request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // 只读响应，关闭失败无副作用

	if resp.StatusCode != http.StatusOK {
		return nil, protocolError(resp)
	}

	var parsed openaiUsageResult
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "decode openai usage response")
	}

	windows := make([]vo.QuotaWindow, 0, 2)
	appendWindow := func(key string, window *openaiUsageWindow) {
		if window == nil {
			return
		}
		minutes := int(window.LimitWindowSeconds / constant.OAuthSecondsPerMinute)
		windows = append(windows, vo.QuotaWindow{
			Key:           key,
			UsedPercent:   window.UsedPercent,
			ResetsAt:      parseTimeBestEffort(window.ResetAt),
			WindowMinutes: minutes,
		})
	}
	appendWindow(constant.OAuthQuotaWindowPrimary, parsed.RateLimit.PrimaryWindow)
	appendWindow(constant.OAuthQuotaWindowSecondary, parsed.RateLimit.SecondaryWindow)
	return &vo.QuotaSnapshot{Windows: windows, CapturedAt: time.Now()}, nil
}
