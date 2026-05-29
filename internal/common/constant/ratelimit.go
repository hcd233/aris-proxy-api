package constant

import "time"

const (
	PeriodCallProxyLLM = 1 * time.Second
	LimitCallProxyLLM  = 100

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
