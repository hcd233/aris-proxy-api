package constant

import "time"

const (
	RedisDB = 0

	PostgresMaxIdleConns    = 10
	PostgresMaxOpenConns    = 100
	PostgresConnMaxLifetime = 5 * time.Hour

	DBConditionDeletedAtZero    = "deleted_at = 0"
	DBConditionDeletedAtNotZero = "deleted_at != 0"
	DBConditionInTemplate       = "%s IN ?"
	DBConditionIDGreaterThan    = "id > ?"
	DBConditionWhereIDIn        = "id IN ?"
	DBOrderByID                 = "id"
	DBLockStrengthUpdate        = "UPDATE"

	// TableMessages messages 表名
	TableMessages = "messages"
	// DBConditionBlobMigrateCandidate 迁移游标条件（不带 deleted_at，软删行一并处理）
	DBConditionBlobMigrateCandidate = "id > ? AND message LIKE ?"
	// DBConditionIDInAndMessageLike 指定消息范围内按 message 文本预过滤
	DBConditionIDInAndMessageLike = "id IN ? AND message LIKE ?"
	// DBConditionMessageLike 全表按 message 文本预过滤（不带 deleted_at，软删行也算引用）
	DBConditionMessageLike = "message LIKE ?"
	// DBConditionBlobRetryable 可被改写为 uploaded/重试/失败的状态
	DBConditionBlobRetryable = "id = ? AND status IN ?"
	// DBConditionBlobDue 到期待重试
	DBConditionBlobDue = "status = ? AND next_retry_at <= ?"
	// DBConditionBlobStatusEquals 按状态过滤
	DBConditionBlobStatusEquals = "status = ?"
	// DBOrderNextRetryAtAsc 重试按到期时间升序
	DBOrderNextRetryAtAsc = "next_retry_at ASC"

	DBJSONConditionAssistantRole  = "(message::jsonb)->>'role' = 'assistant'"
	DBJSONConditionHasThinkTag    = "(message::jsonb)->>'content' LIKE '%<think>%'"
	DBJSONConditionReasoningEmpty = "((message::jsonb)->>'reasoning_content' IS NULL OR (message::jsonb)->>'reasoning_content' = '')"

	// DBJSONConditionHasToolCalls message 的 tool_calls 为非空数组。
	//
	// jsonb_typeof 前置守卫是必需的：tool_calls 键缺失时 jsonb_array_length(NULL)
	// 返回 NULL 尚可，但该键为非数组类型时会直接报错。
	DBJSONConditionHasToolCalls = "jsonb_typeof((message::jsonb)->'tool_calls') = 'array' AND jsonb_array_length((message::jsonb)->'tool_calls') > 0"

	// SharedPoolUserID 共享池归属：user_id=0 的存量/共享配置，仅供网关共享池回退路径使用。
	SharedPoolUserID uint = 0
)
