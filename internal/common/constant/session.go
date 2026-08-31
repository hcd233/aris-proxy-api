package constant

import "time"

const (
	SessionDetailCacheTTL = 60 * time.Minute

	CronModuleSessionTerminalCleanup = "SessionTerminalCleanupCron"

	CronSpecSessionTerminalCleanup = "0 * * * *"
)
