package constant

import "time"

const (
	PeriodCallProxyLLM = 1 * time.Minute
	LimitCallProxyLLM  = 100

	// LimitCallProxyLLMToken 刻意等效不限（1 亿 TPM 远超任何真实用量）：
	// TPM 上限在此层有意留空，兜底约束在用户配额（APIKeyQuota）与上游自身限流。
	// 这是配置决定而非笔误，勿随意调回小值（会误伤长上下文/批量场景）。
	PeriodCallProxyLLMToken = 1 * time.Minute
	LimitCallProxyLLMToken  = 100000000

	PeriodRefreshToken = 1 * time.Minute
	LimitRefreshToken  = 10

	RateLimitKeyByIP = "ip"

	PeriodGetShareMetadata = 1 * time.Minute
	LimitGetShareMetadata  = 60

	PeriodListShareMessages = 1 * time.Minute
	LimitListShareMessages  = 60

	PeriodListShareTools = 1 * time.Minute
	LimitListShareTools  = 60
)
