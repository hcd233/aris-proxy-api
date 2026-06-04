package constant

import "time"

const (
	// SessionDetailCacheTTL session 详情相关缓存（meta / message / tool）的统一 TTL
	SessionDetailCacheTTL = 60 * time.Minute

	SummarizeMaxRetries = 3
	SummarizeMaxTokens  = 20

	EmptySessionSummary = "空会话"

	CronModuleSessionSummarize   = "SessionSummarizeCron"
	CronModuleSessionDeduplicate = "SessionDeduplicateCron"

	CronSpecSessionSummarize   = "0 2 * * *"
	CronSpecSessionDeduplicate = "0 * * * *"
)
