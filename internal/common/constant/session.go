package constant

const (
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
	CronSpecSessionDeduplicate = "0 1 * * *"
)
