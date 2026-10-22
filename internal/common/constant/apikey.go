package constant

import "time"

const (
	APIKeyPrefix       = "sk-aris-"
	APIKeyCharset      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	APIKeyRandomLength = 24
	APIKeyMaxCount     = 5

	PeriodManageAPIKey = 1 * time.Minute
	LimitManageAPIKey  = 20

	// APIKeyAuthCacheTTL API Key 鉴权缓存有效期。
	//
	// 命中时每请求省掉 2 次 DB 查询（key + user）；代价是吊销/轮换最迟 TTL 后生效——
	// 写路径不做主动失效（命令层不持有 Redis，见 docs/loadtest/QPS-OPTIMIZATION.md §3.2）。
	APIKeyAuthCacheTTL = 30 * time.Second
)
