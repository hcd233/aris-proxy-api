package constant

import "time"

const (
	CronDefaultModule         = "Cron"
	CronInvalidKey            = "invalid_key"
	CronModuleSoftDeletePurge = "SoftDeletePurgeCron"
	CronSpecSoftDeletePurge   = "0 4 * * 0"
	CronModuleThinkExtract    = "ThinkExtractCron"
	CronSpecThinkExtract      = "0 1 * * *"

	// CronLockDefaultTTL 默认 cron 任务分布式锁 TTL
	CronLockDefaultTTL = 5 * time.Minute
	// CronLockDefaultRenewDivisor 当 RenewInterval<=0 时回退到 TTL/Divisor
	CronLockDefaultRenewDivisor = 3
	// CronLockMaxConsecutiveRenewFailures 续期连续失败最大次数
	CronLockMaxConsecutiveRenewFailures = 3
)
