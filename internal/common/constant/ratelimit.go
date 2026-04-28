package constant

import "time"

const (
	PeriodCallProxyLLM = 1 * time.Second
	LimitCallProxyLLM  = 100

	PeriodRefreshToken = 1 * time.Minute
	LimitRefreshToken  = 10

	RateLimitKeyByIP = "ip"
)
