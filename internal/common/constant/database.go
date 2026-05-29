package constant

import "time"

const (
	RedisDB = 0

	PostgresMaxIdleConns    = 10
	PostgresMaxOpenConns    = 100
	PostgresConnMaxLifetime = 5 * time.Hour

	DBConditionDeletedAtZero     = "deleted_at = 0"
	DBConditionDeletedAtNotZero  = "deleted_at != 0"
	DBConditionInTemplate        = "%s IN ?"
	DBConditionAPIKeyNameEqual   = "api_key_name = ?"
	DBConditionAPIKeyNameIn      = "api_key_name IN ?"
	DBOrderByIDAscLimitOffset    = " ORDER BY id ASC LIMIT ? OFFSET ?"
	DBQueryCountActiveSessions   = "SELECT COUNT(*) FROM sessions WHERE deleted_at = 0"
	DBQueryListActiveSessionRows = `SELECT id, created_at, updated_at, summary,
		COALESCE(jsonb_array_length(message_ids::jsonb), 0) AS message_count,
		COALESCE(jsonb_array_length(tool_ids::jsonb), 0) AS tool_count
		FROM sessions WHERE deleted_at = 0`

	AggregateTypeEndpoint       = "llmproxy.endpoint"
	AggregateTypeModel          = "llmproxy.model"
	AggregateTypeAPIKey         = "apikey.proxy_api_key"
	AggregateTypeUser           = "identity.user"
	AggregateTypeOAuthIdentity  = "oauth2.identity"
	AggregateTypeModelCallAudit = "modelcall.audit"
	AggregateTypeMessage        = "conversation.message"
	AggregateTypeTool           = "conversation.tool"
	AggregateTypeSession        = "session.session"
)
