package constant

const (
	FieldID                          = "id"
	FieldDeletedAt                   = "deleted_at"
	FieldCheckSum                    = "check_sum"
	FieldMessageIDs                  = "message_ids"
	FieldToolIDs                     = "tool_ids"
	FieldMetadata                    = "metadata"
	FieldSummary                     = "summary"
	FieldSummarizeError              = "summarize_error"
	FieldScoreVersion                = "score_version"
	FieldScore                       = "score"
	FieldScoredAt                    = "scored_at"
	FieldName                        = "name"
	FieldKey                         = "key"
	FieldUserID                      = "user_id"
	FieldModel                       = "model"
	FieldAPIKey                      = "api_key"
	FieldAPIKeyName                  = "api_key_name"
	FieldBaseURL                     = "base_url"
	FieldProvider                    = "provider"
	FieldAlias                       = "alias"
	FieldOpenaiBaseURL               = "openai_base_url"
	FieldAnthropicBaseURL            = "anthropic_base_url"
	FieldSupportOpenAIChatCompletion = "support_openai_chat_completion"
	FieldSupportOpenAIResponse       = "support_openai_response"
	FieldSupportAnthropicMessage     = "support_anthropic_message"
	FieldEndpointID                  = "endpoint_id"
	FieldLastLogin                   = "last_login"
	FieldCreatedAt                   = "created_at"
	FieldUpdatedAt                   = "updated_at"
	FieldMessage                     = "message"
	FieldTool                        = "tool"
	FieldEmail                       = "email"
	FieldAvatar                      = "avatar"
	FieldPermission                  = "permission"
	FieldGithubBindID                = "github_bind_id"
	FieldGoogleBindID                = "google_bind_id"

	FieldTraceID                  = "trace_id"
	FieldInputTokens              = "input_tokens"
	FieldOutputTokens             = "output_tokens"
	FieldFirstTokenLatencyMs      = "first_token_latency_ms"
	FieldStreamDurationMs         = "stream_duration_ms"
	FieldAPIKeyID                 = "api_key_id"
	FieldModelID                  = "model_id"
	FieldUpstreamProtocol         = "upstream_protocol"
	FieldAPIProtocol              = "api_protocol"
	FieldEndpoint                 = "endpoint"
	FieldCacheCreationInputTokens = "cache_creation_input_tokens"
	FieldCacheReadInputTokens     = "cache_read_input_tokens"
	FieldUserAgent                = "user_agent"
	FieldUpstreamStatusCode       = "upstream_status_code"
	FieldErrorMessage             = "error_message"
	FieldMessageCount             = "message_count"
	FieldToolCount                = "tool_count"
)

const (
	WhereFieldID           = "id"
	WhereFieldCheckSum     = "check_sum"
	WhereFieldSummary      = "summary"
	WhereFieldScoreVersion = "score_version"
	WhereFieldToolIDs      = "tool_ids"
)

var (
	MessageRepoFieldsChecksum = []string{FieldID, FieldCheckSum}
	MessageRepoFieldsFull     = []string{FieldID, FieldModel, FieldMessage, FieldCheckSum, FieldCreatedAt}
	MessageRepoFieldsDetail   = []string{FieldID, FieldModel, FieldMessage, FieldCreatedAt}
	MessageRepoFieldsContent  = []string{FieldID, FieldMessage}

	ToolRepoFieldsChecksum = []string{FieldID, FieldCheckSum}
	ToolRepoFieldsFull     = []string{FieldID, FieldTool, FieldCheckSum, FieldCreatedAt}
	ToolRepoFieldsDetail   = []string{FieldID, FieldTool, FieldCreatedAt}

	UserRepoFieldsFull  = []string{FieldID, FieldName, FieldEmail, FieldAvatar, FieldPermission, FieldLastLogin, FieldCreatedAt, FieldGithubBindID, FieldGoogleBindID}
	UserRepoFieldsBasic = []string{FieldID, FieldName}
	UserRepoFieldsAuth  = []string{FieldID, FieldName, FieldPermission}

	SessionRepoFieldsList       = []string{FieldID, FieldCreatedAt, FieldUpdatedAt, FieldSummary, FieldMessageIDs, FieldToolIDs}
	SessionRepoFieldsDetail     = []string{FieldID, FieldAPIKeyName, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs, FieldMetadata, FieldSummary, FieldSummarizeError, FieldScore, FieldScoredAt}
	SessionRepoFieldsReadList   = []string{FieldID, FieldCreatedAt, FieldUpdatedAt, FieldSummary, FieldScore}
	SessionRepoFieldsReadDetail = []string{FieldID, FieldAPIKeyName, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs, FieldMetadata, FieldScore, FieldScoredAt}
	SessionRepoFieldsDedup      = []string{FieldID, FieldMessageIDs, FieldToolIDs}
	SessionRepoFieldsSummarize  = []string{FieldID, FieldMessageIDs}

	EndpointRepoFieldsFull = []string{FieldID, FieldName, FieldOpenaiBaseURL, FieldAnthropicBaseURL, FieldAPIKey,
		FieldSupportOpenAIChatCompletion, FieldSupportOpenAIResponse, FieldSupportAnthropicMessage,
		FieldCreatedAt, FieldUpdatedAt}

	ModelRepoFieldsFull  = []string{FieldID, FieldAlias, FieldModel, FieldEndpointID, FieldCreatedAt, FieldUpdatedAt}
	ModelRepoFieldsAlias = []string{FieldAlias}

	ProxyAPIKeyRepoFieldsFull = []string{FieldID, FieldUserID, FieldName, FieldKey, FieldCreatedAt}
	ProxyAPIKeyRepoFieldsAuth = []string{FieldID, FieldUserID}

	AuditRepoFields = []string{FieldID, FieldAPIKeyID, FieldModelID, FieldModel, FieldUpstreamProtocol, FieldAPIProtocol, FieldEndpoint, FieldInputTokens, FieldOutputTokens, FieldCacheCreationInputTokens, FieldCacheReadInputTokens, FieldFirstTokenLatencyMs, FieldStreamDurationMs, FieldUserAgent, FieldUpstreamStatusCode, FieldErrorMessage, FieldTraceID, FieldCreatedAt}

	AuditQueryFields = []string{FieldTraceID, FieldModel}

	AuditMaxPageSize = 100

	SessionMaxPageSize = 200

	// SessionListINChunkSize session 列表「空 summary fallback」批量加载消息时，
	// 每条 SELECT ... WHERE id IN (?) 携带的 ID 上限。
	//
	// 选 5000 的原因：PG 单语句 bind param 上限是 65535；5000 远低于上限，
	// 保证每条 SQL 的 IN 列表与解析开销可控。当输入 ~12000 IDs 时切分为 3 块，
	// 远少于旧实现 FindInBatches(500) 的 24 次顺序往返。
	SessionListINChunkSize = 5000

	SessionSummarySelect = "id, created_at, updated_at, summary, score, COALESCE(jsonb_array_length(message_ids::jsonb), 0) AS message_count, COALESCE(jsonb_array_length(tool_ids::jsonb), 0) AS tool_count"

	SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM messages WHERE jsonb_exists(sessions.message_ids::jsonb, messages.id::text) AND messages.message::text ILIKE ?)"

	DateTruncMinute = "date_trunc('minute', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncHour   = "date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncDay    = "date_trunc('day', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncWeek   = "date_trunc('week', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"

	SQLConditionUpstreamSuccess = "upstream_status_code = 200"
)
