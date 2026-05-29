package constant

import "time"

const (
	// SessionDetailCacheTTL session 详情相关缓存（meta / message / tool）的统一 TTL
	SessionDetailCacheTTL = 60 * time.Minute

	SummarizeMaxRetries = 3
	SummarizeMaxTokens  = 20

	ScoreMaxRetries = 3
	ScoreMaxTokens  = 200
	ScoreVersion    = "v1.0.0"

	EmptySessionSummary = "空会话"

	CronModuleSessionSummarize   = "SessionSummarizeCron"
	CronModuleSessionScore       = "SessionScoreCron"
	CronModuleSessionDeduplicate = "SessionDeduplicateCron"

	CronSpecSessionSummarize   = "0 2 * * *"
	CronSpecSessionScore       = "0 3 * * *"
	CronSpecSessionDeduplicate = "0 * * * *"
)
