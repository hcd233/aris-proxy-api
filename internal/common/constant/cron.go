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

	CronDescriptionSessionDeduplicate = "Clean up redundant sessions whose message IDs are contained by other sessions"
	CronDescriptionSoftDeletePurge    = "Permanently delete soft-deleted records past retention period"
	CronDescriptionThinkExtract       = "Extract and cache think content from sessions"
	CronDescriptionBlockedHitSync     = "Sync blocked word hit counts to database"

	// CronLockDefaultTTL 默认 cron 任务分布式锁 TTL
	CronLockDefaultTTL = 5 * time.Minute
	// CronLockDefaultRenewDivisor 当 RenewInterval<=0 时回退到 TTL/Divisor
	CronLockDefaultRenewDivisor = 3
	// CronLockMaxConsecutiveRenewFailures 续期连续失败最大次数
	CronLockMaxConsecutiveRenewFailures = 3

	// ── Cron Pub/Sub action ──
	CronReloadActionRestart = "restart"
	CronReloadActionDisable = "disable"
	CronReloadActionEnable  = "enable"

	// ── Cron Audit Metadata 字段名 ──
	CronMetadataKeyCheckedSessions   = "checked_sessions_count"
	CronMetadataKeyDedupedSessions   = "deduped_sessions_count"
	CronMetadataKeyPurgedMessages    = "purged_messages_count"
	CronMetadataKeyPurgedTools       = "purged_tools_count"
	CronMetadataKeyScannedMessages   = "scanned_messages_count"
	CronMetadataKeyExtractedMessages = "extracted_messages_count"
	CronMetadataKeySyncedHits        = "synced_hits_count"
)
