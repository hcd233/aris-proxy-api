package constant

import "time"

const (
	CronDefaultModule         = "Cron"
	CronInvalidKey            = "invalid_key"
	CronModuleSoftDeletePurge = "SoftDeletePurgeCron"
	CronSpecSoftDeletePurge   = "0 4 * * 0"
	CronModuleThinkExtract    = "ThinkExtractCron"
	CronSpecThinkExtract      = "0 0 * * *"
	CronModuleBlockedHitSync  = "BlockedHitSyncCron"
	CronSpecBlockedHitSync    = "*/5 * * * *"

	CronTypeFunctional = "functional"
	CronTypeCore       = "core"

	FieldCronType = "type"

	CronAuditFilterFieldType   = "type"
	CronAuditFilterFieldStatus = "status"

	CronCallAuditStatusSuccess = "success"
	CronCallAuditStatusFailed  = "failed"
	CronCallAuditStatusPanic   = "panic"
	CronCallAuditStatusSkipped = "skipped"

	CronAuditFilterTypeSQLColumn   = "cron_name"
	CronAuditFilterStatusSQLColumn = "status"

	CronDescriptionSessionDeduplicate = "清理 MessageIDs 被其他 Session 包含的冗余 Session"
	CronDescriptionSoftDeletePurge    = "硬删除已软删除超过保留期的记录"
	CronDescriptionThinkExtract       = "提取并缓存会话中的 think 内容"
	CronDescriptionBlockedHitSync     = "同步敏感词命中计数到数据库"

	// CronLockDefaultTTL 默认 cron 任务分布式锁 TTL
	CronLockDefaultTTL = 5 * time.Minute
	// CronLockDefaultRenewDivisor 当 RenewInterval<=0 时回退到 TTL/Divisor
	CronLockDefaultRenewDivisor = 3
	// CronLockMaxConsecutiveRenewFailures 续期连续失败最大次数
	CronLockMaxConsecutiveRenewFailures = 3
)
