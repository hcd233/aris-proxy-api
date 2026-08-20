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
