// Package oauthprovider 实现订阅账号（Claude Code / Codex）的 OAuth 协议客户端。
//
// 上游端点、client_id、scope 等协议事实集中在 constant/oauth.go；本包承载请求形态
// （body 编码、响应字段映射、鉴权头）。真实授权/刷新需用户浏览器交互无法自动化，
// 单测以 httptest 模拟上游（见 test/unit/oauth_flow/provider_test.go）。
package oauthprovider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
)

// NewClient 按 provider 构造默认客户端；非法 provider 返回 nil（调用方先经 IsValid 校验）。
func NewClient(provider enum.OAuthProvider) service.OAuthProviderClient {
	switch provider {
	case enum.OAuthProviderAnthropic:
		return NewAnthropicClient()
	case enum.OAuthProviderOpenAI:
		return NewOpenAIClient()
	default:
		return nil
	}
}

// NewClients 构造全部 provider 客户端（fx 装配）
func NewClients() map[enum.OAuthProvider]service.OAuthProviderClient {
	return map[enum.OAuthProvider]service.OAuthProviderClient{
		enum.OAuthProviderAnthropic: NewAnthropicClient(),
		enum.OAuthProviderOpenAI:    NewOpenAIClient(),
	}
}

// NewClientWithURLs 以自定义端点构造客户端（测试注入；亦支持自建镜像场景）。
func NewClientWithURLs(provider enum.OAuthProvider, authorizeURL, tokenURL, usageURL string) service.OAuthProviderClient {
	switch provider {
	case enum.OAuthProviderAnthropic:
		return &anthropicOAuthClient{authorizeURL: authorizeURL, tokenURL: tokenURL, usageURL: usageURL}
	case enum.OAuthProviderOpenAI:
		return &openaiOAuthClient{authorizeURL: authorizeURL, tokenURL: tokenURL, usageURL: usageURL}
	default:
		return nil
	}
}

// oauthErrorBody provider 错误响应体（OAuth 标准错误字段）
type oauthErrorBody struct {
	Error string `json:"error"`
}

// protocolError 把非 2xx 响应映射为 OAuthProtocolError（读取体量受限；
// 非 JSON 响应时 reason 退化为原始体，便于排障）。
func protocolError(resp *http.Response) *service.OAuthProtocolError {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, constant.OAuthErrorBodyLimit)) //nolint:errcheck // 错误路径尽力而为
	var parsed oauthErrorBody
	_ = sonic.Unmarshal(raw, &parsed) //nolint:errcheck // 非 JSON 时 reason 留空走兜底
	if parsed.Error == "" {
		parsed.Error = string(raw)
	}
	return &service.OAuthProtocolError{StatusCode: resp.StatusCode, Reason: sanitizeErrorReason(parsed.Error)}
}

// parseTimeBestEffort 解析 RFC3339 时间；失败返回零值（展示用途，不阻断探测）
func parseTimeBestEffort(raw string) time.Time {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// ──────────────────────────── Claude（Anthropic）────────────────────────────

// anthropicOAuthClient Claude Code 订阅 OAuth 客户端
type anthropicOAuthClient struct {
	authorizeURL string
	tokenURL     string
	usageURL     string
}

// NewAnthropicClient 构造 Claude 订阅客户端（默认端点）
func NewAnthropicClient() service.OAuthProviderClient {
	return &anthropicOAuthClient{
		authorizeURL: constant.OAuthClaudeAuthorizeURL,
		tokenURL:     constant.OAuthClaudeTokenURL,
		usageURL:     constant.OAuthClaudeUsageURL,
	}
}

// Provider 返回提供方标识
func (c *anthropicOAuthClient) Provider() enum.OAuthProvider { return enum.OAuthProviderAnthropic }

// AuthorizeURL 生成 Claude 授权链接（PKCE S256 + state）
func (c *anthropicOAuthClient) AuthorizeURL(challenge, state string) string {
	query := url.Values{}
	query.Set(constant.OAuthParamClientID, constant.OAuthClaudeClientID)
	query.Set(constant.OAuthParamResponseType, constant.OAuthParamValueAuthorizationCode)
	query.Set(constant.OAuthParamRedirectURI, constant.OAuthClaudeRedirectURI)
	query.Set(constant.OAuthParamScope, constant.OAuthClaudeScope)
	query.Set(constant.OAuthParamCodeChallenge, challenge)
	query.Set(constant.OAuthParamCodeChallengeMethod, constant.OAuthParamValueS256)
	query.Set(constant.OAuthCallbackParamState, state)
	return c.authorizeURL + "?" + query.Encode()
}

// claudeExchangePayload Claude 换 token 请求体（JSON 形态）
type claudeExchangePayload struct {
	Code         string `json:"code"`
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state,omitempty"`
}

// claudeRefreshPayload Claude 刷新请求体
type claudeRefreshPayload struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
}

// claudeOrganization / claudeAccount Claude token 响应中的账号信息块
type claudeOrganization struct {
	UUID string `json:"uuid"`
}

type claudeAccount struct {
	EmailAddress string `json:"email_address"`
}

// claudeTokenResult Claude token 响应（换取与刷新共用字段）
type claudeTokenResult struct {
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	ExpiresIn    int64              `json:"expires_in"`
	Organization claudeOrganization `json:"organization"`
	Account      claudeAccount      `json:"account"`
}

// ExchangeCode 用授权码换 token
func (c *anthropicOAuthClient) ExchangeCode(ctx context.Context, code, verifier, state string) (*service.OAuthTokenSet, error) {
	body, err := sonic.Marshal(claudeExchangePayload{
		Code:         code,
		GrantType:    constant.OAuthParamValueAuthorizationCode,
		ClientID:     constant.OAuthClaudeClientID,
		RedirectURI:  constant.OAuthClaudeRedirectURI,
		CodeVerifier: verifier,
		State:        state,
	})
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "marshal claude exchange request")
	}
	return c.postToken(ctx, body)
}

// Refresh 刷新 access token
func (c *anthropicOAuthClient) Refresh(ctx context.Context, refreshToken string) (*service.OAuthTokenSet, error) {
	body, err := sonic.Marshal(claudeRefreshPayload{
		GrantType:    constant.OAuthParamValueRefreshToken,
		RefreshToken: refreshToken,
		ClientID:     constant.OAuthClaudeClientID,
	})
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "marshal claude refresh request")
	}
	return c.postToken(ctx, body)
}

// postToken 发送 JSON token 请求并映射响应
func (c *anthropicOAuthClient) postToken(ctx context.Context, body []byte) (*service.OAuthTokenSet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "build claude token request")
	}
	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	req.Header.Set(constant.HTTPHeaderAccept, constant.HTTPAcceptJSONAny)
	setClaudeClientHeaders(req.Header)

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "claude oauth token request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // 只读响应，关闭失败无副作用

	if resp.StatusCode != http.StatusOK {
		return nil, protocolError(resp)
	}

	var parsed claudeTokenResult
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "decode claude token response")
	}
	if parsed.AccessToken == "" {
		return nil, ierr.New(ierr.ErrProxyRequest, "claude token response missing access_token")
	}
	if parsed.ExpiresIn <= 0 {
		// 未报有效期时不能默认"永不过期"：必须报错，否则凭据会以"已过期"入表，
		// 每次请求都触发一轮刷新
		return nil, ierr.New(ierr.ErrProxyRequest, "claude token response has non-positive expires_in")
	}
	return &service.OAuthTokenSet{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second),
		AccountRef:   parsed.Organization.UUID,
		AccountEmail: parsed.Account.EmailAddress,
	}, nil
}

// claudeUsageWindow Claude 单窗口用量
type claudeUsageWindow struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

// claudeUsageResult Claude 订阅用量响应
type claudeUsageResult struct {
	FiveHour       *claudeUsageWindow `json:"five_hour"`
	SevenDay       *claudeUsageWindow `json:"seven_day"`
	SevenDaySonnet *claudeUsageWindow `json:"seven_day_sonnet"`
}

// ProbeQuota 探测 Claude 订阅窗口用量（5h / 7d / 7d Sonnet）
func (c *anthropicOAuthClient) ProbeQuota(ctx context.Context, accessToken, _ string) (*vo.QuotaSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.usageURL, http.NoBody)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "build claude usage request")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+accessToken)
	req.Header.Set(constant.HTTPHeaderAccept, constant.HTTPAcceptJSONAny)
	req.Header.Set(constant.HTTPHeaderAnthropicBeta, constant.OAuthClaudeBetaHeader)
	setClaudeClientHeaders(req.Header)

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "claude oauth usage request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // 只读响应，关闭失败无副作用

	if resp.StatusCode != http.StatusOK {
		return nil, protocolError(resp)
	}

	var parsed claudeUsageResult
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "decode claude usage response")
	}

	windows := make([]vo.QuotaWindow, 0, 3)
	appendWindow := func(key string, minutes int, window *claudeUsageWindow) {
		if window == nil {
			return
		}
		windows = append(windows, vo.QuotaWindow{
			Key: key,
			// 上游 utilization 已是 0~100 百分比（如 14.0 表示 14%），与 Codex 侧同量纲，直接透传
			UsedPercent:   window.Utilization,
			ResetsAt:      parseTimeBestEffort(window.ResetsAt),
			WindowMinutes: minutes,
		})
	}
	appendWindow(constant.OAuthQuotaWindowFiveHour, constant.OAuthQuotaWindowFiveHourMinutes, parsed.FiveHour)
	appendWindow(constant.OAuthQuotaWindowSevenDay, constant.OAuthQuotaWindowSevenDayMinutes, parsed.SevenDay)
	appendWindow(constant.OAuthQuotaWindowSevenDaySonnet, constant.OAuthQuotaWindowSevenDayMinutes, parsed.SevenDaySonnet)
	return &vo.QuotaSnapshot{Windows: windows, CapturedAt: time.Now()}, nil
}

// sanitizeErrorReason 清洗上游错误摘要：去换行、按字符限长（该值会经 last_error 落库并下发管理端）。
//
// 必须按字符而非字节截断：上游错误体可能含中文，按字节切会得到非法 UTF-8 而写入失败。
func sanitizeErrorReason(raw string) string {
	cleaned := strings.TrimSpace(strings.NewReplacer("\n", " ", "\r", " ").Replace(raw))
	return commonutil.TruncateRunes(cleaned, constant.OAuthErrorReasonMaxLength)
}
