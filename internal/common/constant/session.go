package constant

import "time"

const (
	SessionDetailCacheTTL = 60 * time.Minute

	CronModuleSessionDeduplicate = "SessionDeduplicateCron"

	CronSpecSessionDeduplicate = "0 * * * *"
)
