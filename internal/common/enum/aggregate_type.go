package enum

type AggregateType = string

const (
	AggregateTypeEndpoint       AggregateType = "llmproxy.endpoint"
	AggregateTypeModel          AggregateType = "llmproxy.model"
	AggregateTypeAPIKey         AggregateType = "apikey.proxy_api_key"
	AggregateTypeUser           AggregateType = "identity.user"
	AggregateTypeOAuthIdentity  AggregateType = "oauth2.identity"
	AggregateTypeModelCallAudit AggregateType = "modelcall.audit"
	AggregateTypeMessage        AggregateType = "conversation.message"
	AggregateTypeTool           AggregateType = "conversation.tool"
	AggregateTypeSession        AggregateType = "session.session"
	AggregateTypeBlocked        AggregateType = "blocked.blocked"
)
