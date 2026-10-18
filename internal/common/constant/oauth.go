package constant

import "time"

const (
	GithubUserURL      = "https://api.github.com/user"
	GithubUserEmailURL = "https://api.github.com/user/emails"
	GoogleUserInfoURL  = "https://www.googleapis.com/oauth2/v2/userinfo"

	DefaultUserNamePrefix = "ArisUser"

	OAuthStateBytes = 32

	PeriodOAuth2Callback = 5 * time.Second
	LimitOAuth2Callback  = 16

	// Demo 登录入口限流（IP 维度，防刷）
	PeriodDemoLogin = 5 * time.Second
	LimitDemoLogin  = 8

	// Demo 登录后的接口访问限流（IP 维度，防单访客刷爆共享账户）
	PeriodDemoAccess = 5 * time.Second
	LimitDemoAccess  = 30

	OAuthStateManagerTTL = 10 * time.Minute
	OAuthStateMaxPending = 100

	// StateProviderPrefix + platform + StateProviderSeparator + random hex
	StateProviderPrefix    = "provider:"
	StateProviderSeparator = ":"

	// RedisOAuthStateKeyPrefix OAuth state 的 Redis key 前缀
	RedisOAuthStateKeyPrefix = "oauth:state:"

	// RedisOAuthStateVerifyScript Lua 脚本：原子地 GET + DEL
	// 如果 key 不存在返回空，存在则删除并返回创建时间戳
	RedisOAuthStateVerifyScript = `
local val = redis.call("GET", KEYS[1])
if val then
    redis.call("DEL", KEYS[1])
    return val
end
return nil
`

	// DecimalBase10 十进制基数，用于 strconv.ParseInt
	DecimalBase10 = 10

	// BitSize64 64 位整数，用于 strconv.ParseInt 的 bitSize 参数
	BitSize64 = 64
)

// 订阅账号 OAuth 协议常量（Claude Code / Codex）。
//
// 取值为上游协议事实（授权端点、client_id、scope 等），来源为官方客户端公开行为
// （见设计文档 docs/superpowers/specs/2026-10-18-oauth-subscription-pooling-design.md §4.1）；
// 上游变更时只改本文件，禁止把字面量散落到业务代码。
const (
	// ── Anthropic（Claude Code 订阅）──

	// OAuthClaudeClientID Claude Code 公开 OAuth 客户端 ID
	OAuthClaudeClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	// OAuthClaudeAuthorizeURL Claude 授权页
	OAuthClaudeAuthorizeURL = "https://claude.com/cai/oauth/authorize"
	// OAuthClaudeTokenURL Claude token 端点（换取与刷新共用）
	OAuthClaudeTokenURL = "https://platform.claude.com/v1/oauth/token"
	// OAuthClaudeRedirectURI Claude 固定回调（页面展示 code，用户手动回贴）
	OAuthClaudeRedirectURI = "https://platform.claude.com/oauth/code/callback"
	// OAuthClaudeScope Claude 授权 scope（空格连接）
	OAuthClaudeScope = "user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"
	// OAuthClaudeUsageURL Claude 订阅窗口用量查询
	OAuthClaudeUsageURL = "https://api.anthropic.com/api/oauth/usage"
	// OAuthClaudeBetaHeader OAuth 调用必需的 anthropic-beta 值
	OAuthClaudeBetaHeader = "oauth-2025-04-20"

	// ── OpenAI（Codex 订阅）──

	// OAuthOpenAIClientID Codex CLI 公开 OAuth 客户端 ID
	OAuthOpenAIClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	// OAuthOpenAIAuthorizeURL OpenAI 授权页
	OAuthOpenAIAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	// OAuthOpenAITokenURL OpenAI token 端点（换取与刷新共用）
	OAuthOpenAITokenURL = "https://auth.openai.com/oauth/token"
	// OAuthOpenAIRedirectURI Codex 固定回调（浏览器打不开 localhost 时用户从地址栏复制整段 URL）
	OAuthOpenAIRedirectURI = "http://localhost:1455/auth/callback"
	// OAuthOpenAIScope OpenAI 换取授权码时的 scope
	OAuthOpenAIScope = "openid profile email offline_access"
	// OAuthOpenAIRefreshScope OpenAI 刷新 token 时的 scope（不含 offline_access）
	OAuthOpenAIRefreshScope = "openid profile email"
	// OAuthOpenAIUsageURL Codex 订阅窗口用量查询
	OAuthOpenAIUsageURL = "https://chatgpt.com/backend-api/wham/usage"
)

// 订阅账号 OAuth 生命周期与熵参数
const (
	// OAuthFlowTTL 授权流临时记录存活时间（用户需在此窗口内完成浏览器授权并回贴 code）
	OAuthFlowTTL = 10 * time.Minute

	// OAuthTokenRefreshSkew access token 刷新提前量（请求路径判定"即将过期"即刷新）
	OAuthTokenRefreshSkew = 3 * time.Minute

	// OAuthClaudeVerifierBytes Claude PKCE verifier 随机字节数（base64url 后 43 字符）
	OAuthClaudeVerifierBytes = 32
	// OAuthOpenAIVerifierBytes OpenAI PKCE verifier 随机字节数（hex 后 128 字符）
	OAuthOpenAIVerifierBytes = 64

	// OAuthURLSchemeHTTPPrefix / OAuthURLSchemeHTTPSPrefix 回调 URL 形态判定前缀
	OAuthURLSchemeHTTPPrefix  = "http://"
	OAuthURLSchemeHTTPSPrefix = "https://"
	// OAuthCallbackParamCode / OAuthCallbackParamState 回调 URL 的 query 参数名
	OAuthCallbackParamCode  = "code"
	OAuthCallbackParamState = "state"
)
