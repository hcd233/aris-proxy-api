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
	// APIKeyAuthCacheTTLJitter 鉴权缓存 TTL 的随机抖动上限（实际 TTL = 基准 + [0, jitter)）
	//
	// 没有抖动时，同批写入的条目（进程重启预热、压测同时打同一批 Key）会在同一秒集体
	// 过期，把平滑的回源变成周期尖峰。取基准的 20%，与 cache.ReadCache 的抖动比例一致。
	APIKeyAuthCacheTTLJitter = 6 * time.Second
)
