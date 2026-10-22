package constant

import "time"

const (
	GuardStrikeThreshold = 5
	GuardStrikeWindow    = 1 * time.Minute
	GuardBanDuration     = 1 * time.Hour

	// GuardCleanCacheTTL 进程内「该 IP 未被封禁」负缓存有效期。
	//
	// 每请求一次的 Redis EXISTS 是固定成本，而封禁是低频事件：缓存 1s 内的干净结论，
	// 把 Redis 调用摊薄到每 IP 每秒 1 次。代价是封禁最迟 TTL 后生效。
	GuardCleanCacheTTL = 1 * time.Second
	// GuardCleanCacheLimit 负缓存条目上限；超限整体清空（O(1)，防伪造 IP 撑爆内存）。
	GuardCleanCacheLimit = 4096
)
