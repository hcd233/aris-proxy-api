package constant

const (
	FieldID                          = "id"
	FieldDeletedAt                   = "deleted_at"
	FieldCheckSum                    = "check_sum"
	FieldMessageIDs                  = "message_ids"
	FieldToolIDs                     = "tool_ids"
	FieldMetadata                    = "metadata"
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
	FieldQuestions                = "questions"
)

const (
	WhereFieldID           = "id"
	WhereFieldCheckSum     = "check_sum"
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

	SessionRepoFieldsList       = []string{FieldID, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs}
	SessionRepoFieldsDetail     = []string{FieldID, FieldAPIKeyName, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs, FieldMetadata, FieldScore, FieldScoredAt}
	SessionRepoFieldsReadList   = []string{FieldID, FieldCreatedAt, FieldUpdatedAt, FieldScore}
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

	// SessionSummarySelect session 列表投影。
	//
	// 设计要点（perf/session-list-trigram-and-windowcount-2026-06-08）：
	//   把 COUNT(*) OVER () AS total_count 折进同一条 SELECT，省掉一次独立 COUNT(*)
	//   roundtrip 与一次 WHERE 评估。对带 keyword 的请求尤其受益——EXISTS 子查询
	//   原来要跑两遍（COUNT 一次、SELECT 一次），现在一次搞定。
	//   sessionSummaryRow.TotalCount 接收每行（窗口函数对所有行返回相同值）。
	//
	//   message_count 和 tool_count 从 message_ids / tool_ids 实时计算，不再物化冗余列。
	SessionSummarySelect = "id, created_at, updated_at, score, COALESCE(jsonb_array_length(message_ids::jsonb), 0) AS message_count, COALESCE(jsonb_array_length(tool_ids::jsonb), 0) AS tool_count, questions, COUNT(*) OVER () AS total_count"

	// SessionKeywordFilterSQL session 列表 keyword 过滤 SQL 片段。
	//
	// 设计要点（refactor/session-list-keyword-perf-2026-06-07）：
	//   旧实现写成 "EXISTS (SELECT 1 FROM messages WHERE jsonb_exists(sessions.message_ids::jsonb,
	//   messages.id::text) AND messages.message::text ILIKE ?)"，messages 上没有任何能命中
	//   ILIKE 的索引，且 jsonb_exists 把 sessions 与 messages 强相关，planner 只能为每条
	//   候选 session 在 messages 全表上重跑一次 ILIKE 顺序扫描；外层再叠 COUNT(*)，复杂度
	//   接近 O(候选 sessions × messages)。
	//
	//   新实现把方向反过来：先把这条 session 自己的 message_ids 数组（通常 5～50 条）
	//   通过 jsonb_array_elements_text 在内存里展开，再按 PK 回查 messages（messages.id 走
	//   主键索引，O(log N)），最后只对这 K 行做 ILIKE。
	//
	//   复杂度从 "候选 sessions × M（messages 总量）"
	//   降到 "候选 sessions × K（每 session 的 messages 数）"，K << M。
	//   COUNT(*) 跑同一个 WHERE，同步受益。
	//
	// 占位符约束：
	//   - 必须是 ILIKE ?（gorm 占位符），且整段 SQL 中只能有 1 个 '?'，
	//     否则会与 gorm 占位符撞车（参考 fix #59 的 jsonb_exists 由来）。
	//   - 不要写 messages.id = ANY(sessions.message_ids)：message_ids 在 PG 里是 jsonb 文本，
	//     不是原生数组，会触发 SQLSTATE 42809（参考 fix #58）。
	SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid) JOIN messages ON messages.id = arr.mid::bigint WHERE messages.message::text ILIKE ?)"

	DateTruncMinute = "date_trunc('minute', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncHour   = "date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncDay    = "date_trunc('day', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncWeek   = "date_trunc('week', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"

	SQLConditionUpstreamSuccess = "upstream_status_code = 200"
)
