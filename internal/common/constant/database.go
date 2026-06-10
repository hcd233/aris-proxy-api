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

	DBJSONConditionAssistantRole  = "(message::jsonb)->>'role' = 'assistant'"
	DBJSONConditionHasThinkTag    = "(message::jsonb)->>'content' LIKE '%<think>%'"
	DBJSONConditionReasoningEmpty = "((message::jsonb)->>'reasoning_content' IS NULL OR (message::jsonb)->>'reasoning_content' = '')"
)
