package constant

import "time"

const (
	PeriodCallProxyLLM = 1 * time.Second
	LimitCallProxyLLM  = 100

	PeriodRefreshToken = 1 * time.Minute
	LimitRefreshToken  = 10

	RateLimitKeyByIP = "ip"

	PeriodGetShareContent = 1 * time.Minute
	LimitGetShareContent  = 30

	ShareTTL = 24 * time.Hour
)
