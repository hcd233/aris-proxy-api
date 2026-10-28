package constant

import "time"

const (
	CronDefaultModule         = "Cron"
	CronInvalidKey            = "invalid_key"
	CronModuleSoftDeletePurge = "SoftDeletePurgeCron"
	CronSpecSoftDeletePurge   = "0 4 * * 0"
	CronModuleThinkExtract    = "ThinkExtractCron"
	CronSpecThinkExtract      = "0 0 * * *"
	CronModuleTriggerHitSync  = "TriggerHitSyncCron"
	CronSpecTriggerHitSync    = "*/5 * * * *"

	// CronTriggerSourceScheduled 定时触发
	CronTriggerSourceScheduled = "scheduled"
	// CronTriggerSourceManual 手动触发
	CronTriggerSourceManual = "manual"

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

	CronDescriptionSessionTerminalCleanup = "Scan sessions created in the last 24h interrupted at an assistant tool_call and remove them"
	CronDescriptionSoftDeletePurge        = "Permanently delete soft-deleted records past retention period"
	CronDescriptionThinkExtract           = "Extract and cache think content from sessions"
	CronDescriptionTriggerHitSync         = "Sync trigger word hit counts to database"

	// CronLockDefaultTTL 默认 cron 任务分布式锁 TTL
	CronLockDefaultTTL = 5 * time.Minute
	// CronTerminalCleanupWindow 终态清理扫描窗口（最近 24 小时）
	CronTerminalCleanupWindow = 24 * time.Hour
	// CronTerminalCleanupIdleThreshold 终态清理的空闲阈值：updated_at 早于 now-阈值的会话才算「中断」。
	// 续写把快照前滚到同一行并刷新 updated_at，agent 工具循环进行中末条消息常为 tool_call，
	// 不设阈值会把进行中的会话整行删掉，下一轮只能新建会话 ID。
	CronTerminalCleanupIdleThreshold = 30 * time.Minute
	// CronLockDefaultRenewDivisor 当 RenewInterval<=0 时回退到 TTL/Divisor
	CronLockDefaultRenewDivisor = 3
	// CronLockMaxConsecutiveRenewFailures 续期连续失败最大次数
	CronLockMaxConsecutiveRenewFailures = 3

	// ── Cron Pub/Sub action ──
	CronReloadActionRestart = "restart"
	CronReloadActionDisable = "disable"
	CronReloadActionEnable  = "enable"
)

// OAuth 订阅账号维护任务常量
const (
	// CronModuleOAuthMaintenance 订阅账号维护任务的模块名
	CronModuleOAuthMaintenance = "OAuthMaintenanceCron"
	// CronSpecOAuthMaintenance 订阅账号维护任务调度（每 10 分钟）
	CronSpecOAuthMaintenance = "*/10 * * * *"
	// CronDescriptionOAuthMaintenance 订阅账号维护任务描述
	CronDescriptionOAuthMaintenance = "Refresh expiring subscription tokens and probe quota windows"
)
