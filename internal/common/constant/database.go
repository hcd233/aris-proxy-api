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
