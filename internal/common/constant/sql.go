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

	SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM messages WHERE messages.message::text ILIKE ? AND jsonb_exists(sessions.message_ids::jsonb, messages.id::text))"

	// SessionKeywordIndexSQLs session 列表 keyword 检索依赖的 PostgreSQL 索引 DDL。
	//
	// 背景（feature/session-keyword-trgm-perf-2026-06-07）：
	//   - session_keyword_filter SQL 形如 "messages.message::text ILIKE '%kw%'"，
	//     在没有 trigram 索引时是顺序扫描全表（按 2026-06-07 线上 messages 体量
	//     一次 keyword 列表查询可耗秒级）。
	//   - pg_trgm + GIN trgm_ops 把 ILIKE 退化为 trigram bitmap 扫描，
	//     对 2 字符及以上的子串检索都能命中索引。
	//
	// 三条语句依次为：
	//  1. 启用 pg_trgm 扩展（幂等，需要 superuser 或 db owner 权限）；
	//  2. 在 messages.message::text 上建 GIN trigram 索引（用于 ILIKE）；
	//  3. 在 sessions.message_ids::jsonb 上建 GIN jsonb_path_ops 索引（用于
	//     jsonb_exists 的反查，避免每条 session 反复做小数组顺序扫描）。
	//
	// 全部使用 IF NOT EXISTS，由 internal/infrastructure/database.EnsureSearchIndexes
	// 在 database migrate 阶段幂等执行。
	SessionKeywordIndexSQLs = []string{
		"CREATE EXTENSION IF NOT EXISTS pg_trgm",
		"CREATE INDEX IF NOT EXISTS idx_messages_message_trgm ON messages USING gin (message::text gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_message_ids_gin ON sessions USING gin (message_ids::jsonb jsonb_path_ops)",
	}

	DateTruncMinute = "date_trunc('minute', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncHour   = "date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncDay    = "date_trunc('day', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncWeek   = "date_trunc('week', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"

	SQLConditionUpstreamSuccess = "upstream_status_code = 200"
)
