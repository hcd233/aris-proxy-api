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
	DBOrderByIDAsc              = "id ASC"
	DBConditionDedupKeyNotZero  = "dedup_key <> ''"
	DBLockStrengthUpdate        = "UPDATE"

	DBJSONConditionAssistantRole  = "(message::jsonb)->>'role' = 'assistant'"
	DBJSONConditionHasThinkTag    = "(message::jsonb)->>'content' LIKE '%<think>%'"
	DBJSONConditionReasoningEmpty = "((message::jsonb)->>'reasoning_content' IS NULL OR (message::jsonb)->>'reasoning_content' = '')"

	// DBJSONConditionHasToolCalls message 的 tool_calls 为非空数组。
	//
	// jsonb_typeof 前置守卫是必需的：tool_calls 键缺失时 jsonb_array_length(NULL)
	// 返回 NULL 尚可，但该键为非数组类型时会直接报错。
	DBJSONConditionHasToolCalls = "jsonb_typeof((message::jsonb)->'tool_calls') = 'array' AND jsonb_array_length((message::jsonb)->'tool_calls') > 0"

	// 用户级多租户化迁移（database migrate-data）使用的 SQL 片段与索引定义。
	DBConditionPermissionAdmin = "permission = ?"
	DBConditionUserIDZero      = "user_id = 0"

	SQLDropIndex         = "DROP INDEX IF EXISTS "
	SQLCreateUniqueIndex = "CREATE UNIQUE INDEX IF NOT EXISTS "

	IndexEndpointName = "idx_endpoint_name_deleted"
	ColsEndpointName  = "user_id, name, deleted_at"
	TableEndpoints    = "endpoints"
	TableModels       = "models"
	SQLIndexOn        = " ON "
	IndexModelAliasEp = "idx_model_alias_endpoint_deleted"
	ColsModelAliasEp  = "user_id, alias, endpoint_id, deleted_at"
)
