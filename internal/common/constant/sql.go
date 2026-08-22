package constant

const (
	FieldID                          = "id"
	FieldDeletedAt                   = "deleted_at"
	FieldCheckSum                    = "check_sum"
	FieldMessageIDs                  = "message_ids"
	FieldToolIDs                     = "tool_ids"
	FieldMetadata                    = "metadata"
	FieldScore                       = "score"
	FieldScoredAt                    = "scored_at"
	FieldName                        = "name"
	FieldKey                         = "key"
	FieldUserID                      = "user_id"
	FieldModel                       = "model"
	FieldAPIKey                      = "api_key"
	FieldAPIKeyName                  = "api_key_name"
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

	FieldSpec          = "spec"
	FieldCronName      = "cron_name"
	FieldStartedAt     = "started_at"
	FieldEndedAt       = "ended_at"
	FieldDurationMs    = "duration_ms"
	FieldStatus        = "status"
	FieldTriggerSource = "trigger_source"
	FieldDescription   = "description"

	FieldTraceID                  = "trace_id"
	FieldDedupKey                 = "dedup_key"
	FieldPayload                  = "payload"
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
	FieldEnabled                  = "enabled"
	FieldWord                     = "word"
	FieldHitCount                 = "hit_count"
	FieldAction                   = "action"
	FieldModule                   = "module"
	FieldPath                     = "path"
	FieldIP                       = "ip"
	FieldMessageCount             = "message_count"
	FieldToolCount                = "tool_count"

	FieldSessionID     = "session_id"
	FieldCWD           = "cwd"
	FieldSource        = "source"
	FieldEvent         = "event"
	FieldTurnID        = "turn_id"
	FieldAgent         = "agent"
	FieldParentTraceID = "parent_trace_id"

	DBSelectAll = "*"
)

const (
	WhereFieldID       = "id"
	WhereFieldCheckSum = "check_sum"
)

var (
	MessageRepoFieldsChecksum = []string{FieldID, FieldCheckSum}
	MessageRepoFieldsFull     = []string{FieldID, FieldModelID, FieldMessage, FieldCheckSum, FieldCreatedAt}
	MessageRepoFieldsDetail   = []string{FieldID, FieldModelID, FieldMessage, FieldCreatedAt}
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

	// 消息数桶固定边界（不含动态上限），与 SessionMessageCountBucketCase 对齐；末桶上限由动态 max 截断
	SessionMessageCountBucketEdges = []int{10, 50, 100, 200, 500}

	EndpointRepoFieldsFull = []string{FieldID, FieldName, FieldOpenaiBaseURL, FieldAnthropicBaseURL, FieldAPIKey,
		FieldSupportOpenAIChatCompletion, FieldSupportOpenAIResponse, FieldSupportAnthropicMessage,
		FieldCreatedAt, FieldUpdatedAt}

	ModelRepoFieldsFull  = []string{FieldID, FieldAlias, FieldModelID, FieldModelUpstreamModel, FieldEndpointID, FieldEnabled, FieldModelContextLength, FieldModelMaxOutputTokens, FieldModelCapabilities, FieldCreatedAt, FieldUpdatedAt}
	ModelRepoFieldsAlias = []string{FieldAlias}

	ProxyAPIKeyRepoFieldsFull = []string{FieldID, FieldUserID, FieldName, FieldKey, FieldCreatedAt}
	ProxyAPIKeyRepoFieldsAuth = []string{FieldID, FieldUserID}

	AuditRepoFieldIDQualified        = "model_call_audits.id"
	AuditRepoFieldCreatedAtQualified = "model_call_audits.created_at"

	AuditRepoFields = []string{AuditRepoFieldIDQualified, FieldAPIKeyID, FieldModelID, FieldUpstreamProtocol, FieldAPIProtocol, FieldEndpoint, FieldInputTokens, FieldOutputTokens, FieldCacheCreationInputTokens, FieldCacheReadInputTokens, FieldFirstTokenLatencyMs, FieldStreamDurationMs, FieldUserAgent, FieldUpstreamStatusCode, FieldErrorMessage, FieldTraceID, AuditRepoFieldCreatedAtQualified}

	AuditQueryFields = []string{FieldTraceID, FieldModelID}

	AuditFilterFieldUser   = "user"
	AuditFilterFieldModel  = "model"
	AuditFilterFieldStatus = "status"
	AuditFilterFieldUA     = "ua"

	TriggerRepoFieldsFull = []string{FieldID, FieldWord, FieldHitCount, FieldAction, FieldCreatedAt, FieldUpdatedAt}

	AuditMaxPageSize = 500

	SessionMaxPageSize = 500

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
	SessionSummarySelect = "id, created_at, updated_at, score, COALESCE(jsonb_array_length(message_ids::jsonb), 0) AS message_count, COALESCE(jsonb_array_length(tool_ids::jsonb), 0) AS tool_count, questions, model_ids, COUNT(*) OVER () AS total_count"

	// SessionKeywordFilterSQL session 列表 keyword 过滤 SQL 片段。
	//
	// 设计要点（refactor/session-list-keyword-perf-2026-06-07）：
	//   旧实现写成 "EXISTS (SELECT 1 FROM messages WHERE jsonb_exists(sessions.message_ids::jsonb,
	//   messages.id::text) AND messages.message::text ILIKE ?)"，messages 上没有任何能命中
	//   ILIKE 的索引，且 jsonb_exists 把 sessions 与 messages 强相关，planner 只能为每条
	//   候选 session 在 messages 全表上重跑一次 ILIKE 顺序扫描；外层再叠 COUNT(*)，复杂度
	//   接近 O(候选 sessions × messages)。
	//
	//   2026-06-07 曾改为 "jsonb_array_elements_text(sessions.questions::jsonb) arr(mid)
	//   JOIN messages ON messages.id = arr.mid::bigint"：语义上仍是 PK 回查，但 planner 会
	//   被 messages 上的 trigram 索引（idx_messages_message_trgm）误导，估计 %keyword% 命中
	//   很少行而选择 bitmap 路径；keyword 为高频词（如 "Task:"）时 bitmap 索引命中近万条
	//   候选，回表 366MB messages 大表 recheck，单次 2s+，实测接口超时 27.4s。
	//
	//   本次（bugfix/session-list-keyword-perf-2026-08-02）改为 IN 子查询形态：planner 以
	//   arr（每 session 少量 question id）为驱动侧对 messages_pkey 主键点查，彻底避开
	//   trigram bitmap 回表。生产实测 keyword=Task: 22ms、error 17ms、罕见词 19ms。
	//
	// 占位符约束：
	//   - 必须是 ILIKE ?（gorm 占位符），且整段 SQL 中只能有 1 个 '?'，
	//     否则会与 gorm 占位符撞车（参考 fix #59 的 jsonb_exists 由来）。
	//   - 不要写 messages.id = ANY(sessions.message_ids)：message_ids 在 PG 里是 jsonb 文本，
	//     不是原生数组，会触发 SQLSTATE 42809（参考 fix #58）。
	SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid) WHERE arr.mid::bigint IN (SELECT id FROM messages WHERE messages.message::text ILIKE ?))"

	DateTruncMinute = "date_trunc('minute', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncHour   = "date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncDay    = "date_trunc('day', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	DateTruncWeek   = "date_trunc('week', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"

	SQLConditionUpstreamSuccess = "upstream_status_code = 200"

	// ── Audit filter field SQL column names ──
	AuditFilterUserSQLColumn   = "u.name"
	AuditFilterModelSQLColumn  = "model_id"
	AuditFilterStatusSQLColumn = "upstream_status_code"
	AuditFilterUASQLColumn     = "user_agent"

	// ── Audit filter JOIN constants (for paginate queries without alias) ──
	AuditFilterJoinAPIKey = "JOIN proxy_api_keys ON model_call_audits.api_key_id = proxy_api_keys.id"
	AuditFilterJoinUser   = "JOIN users u ON proxy_api_keys.user_id = u.id"

	// ── Audit distinct query constants ──
	AuditDistinctTableMCA        = "model_call_audits mca"
	AuditDistinctSelectUser      = "DISTINCT u.name"
	AuditDistinctJoinAPIKey      = "JOIN proxy_api_keys pak ON mca.api_key_id = pak.id"
	AuditDistinctJoinUser        = "JOIN users u ON pak.user_id = u.id"
	AuditDistinctWhereUser       = "u.name LIKE ? OR u.email LIKE ?"
	AuditDistinctWhereModel      = "model_id LIKE ?"
	AuditDistinctWhereUA         = "user_agent LIKE ?"
	AuditDistinctWhereUANotEmpty = "user_agent <> ''"
	AuditDistinctSelectModel     = "DISTINCT model_id"
	AuditDistinctSelectStatus    = "DISTINCT upstream_status_code::text"
	AuditDistinctSelectUA        = "DISTINCT user_agent"
	AuditDistinctLimit           = 50

	AuditDistinctWhereDeletedAtZero = "mca.deleted_at = 0"
	AuditPaginateWhereDeletedAtZero = "model_call_audits.deleted_at = 0"

	AuditPaginateWhereCreatedAtGTE = "model_call_audits.created_at >= ?"
	AuditPaginateWhereCreatedAtLTE = "model_call_audits.created_at <= ?"

	AuditDistinctWhereCreatedAtGTE = "mca.created_at >= ?"
	AuditDistinctWhereCreatedAtLTE = "mca.created_at <= ?"

	WhereCreatedAtGTE = "created_at >= ?"
	WhereCreatedAtLTE = "created_at <= ?"

	// ── Session distinct score query ──
	SessionDistinctScoreSelect = "DISTINCT score"
	SessionDistinctScoreWhere  = "score IS NOT NULL"
	SessionDistinctScoreOrder  = "score ASC"

	// ── Session filter field constants ──
	SessionFilterFieldModel     = "model"
	SessionFilterModelSQLColumn = "model_ids"

	// ── Session distinct model query ──
	SessionDistinctModelSelect = "DISTINCT jsonb_array_elements_text(model_ids::jsonb) AS model"
	SessionDistinctModelWhere  = "model_ids IS NOT NULL AND model_ids::jsonb <> '[]'::jsonb"
	SessionDistinctModelLike   = "jsonb_array_elements_text(model_ids::jsonb) ILIKE ?"
	SessionDistinctModelOrder  = "model ASC"
	SessionDistinctModelLimit  = 50

	// ── Session message count filter & query constants ──
	SessionFilterFieldMessageCount = "messageCount"
	SessionMessageCountSQLExpr     = "jsonb_array_length(message_ids::jsonb)"

	// 消息数统计（options 接口用）：固定边界桶 0-10 / 11-50 / 51-100 / 101-200 / 201-500 / 501+，
	// 末桶上限由当前时间范围最大消息数动态截断。
	SessionMessageCountMaxSelect  = "COALESCE(MAX(jsonb_array_length(message_ids::jsonb)), 0)"
	SessionMessageCountBucketCase = "CASE WHEN cnt <= 10 THEN 0 WHEN cnt <= 50 THEN 1 WHEN cnt <= 100 THEN 2 WHEN cnt <= 200 THEN 3 WHEN cnt <= 500 THEN 4 ELSE 5 END"

	// 桶区间格式化模板（min-max），options 接口返回桶区间用
	SessionMessageCountBucketFormat = "%d-%d"

	// 桶子查询别名模板（db.Table 子查询用）
	SessionMessageCountSubqueryTable = "(?) AS sub"
	SessionMessageCountBucketIdx     = "bucket_idx"

	// ── Session export query constants ──
	// 导出行：id, score, message_ids, tool_ids, model_ids
	SessionExportSelect = "id, score, message_ids, tool_ids, model_ids"

	// 预览：score + 展开的 model（jsonb_array_elements_text）
	SessionExportPreviewSelect = "score, jsonb_array_elements_text(model_ids::jsonb) AS model"

	// model_ids JSONB 数组包含任意一个目标模型（? 展开为 ANY 数组）
	// 用 jsonb_path_exists 做数组包含判断：model_ids @> ?::jsonb
	SessionExportModelFilterSQL = "model_ids::jsonb @> ?::jsonb"

	// model_ids 非空过滤（预览查询用）
	SessionExportModelIsNotNull = "model_ids IS NOT NULL AND model_ids::jsonb <> '[]'::jsonb"

	// MigrateMessageBatchSize checksum 迁移与 dedup 每批处理记录数
	MigrateMessageBatchSize = 1000

	FieldTableCronJob       = "cron_jobs"
	FieldTableCronCallAudit = "cron_call_audits"

	CronAuditPaginateWhereDeletedAtZero = "cron_call_audits.deleted_at = 0"
	CronAuditDistinctWhereDeletedAtZero = "cca.deleted_at = 0"

	CronAuditPaginateWhereCreatedAtGTE = "cron_call_audits.created_at >= ?"
	CronAuditPaginateWhereCreatedAtLTE = "cron_call_audits.created_at <= ?"
	CronAuditDistinctWhereCreatedAtGTE = "cca.created_at >= ?"
	CronAuditDistinctWhereCreatedAtLTE = "cca.created_at <= ?"

	CronAuditDistinctSelectType   = "DISTINCT cron_name"
	CronAuditDistinctWhereType    = "cron_name LIKE ?"
	CronAuditDistinctOrderType    = "cron_name ASC"
	CronAuditDistinctSelectStatus = "DISTINCT status"
	CronAuditDistinctWhereStatus  = "status LIKE ?"
	CronAuditDistinctOrderStatus  = "status ASC"
	CronAuditDistinctLimit        = 50

	CronCallAuditRepoFieldIDQualified        = "cron_call_audits.id"
	CronCallAuditRepoFieldCreatedAtQualified = "cron_call_audits.created_at"

	CronCallAuditRepoFields = []string{
		CronCallAuditRepoFieldIDQualified,
		FieldCronName,
		FieldTraceID,
		FieldStartedAt,
		FieldEndedAt,
		FieldDurationMs,
		FieldStatus,
		FieldTriggerSource,
		FieldMessage,
		FieldMetadata,
		CronCallAuditRepoFieldCreatedAtQualified,
	}

	// ── Demo access audit ──
	DemoAccessAuditFilterFieldAction = "action"
	DemoAccessAuditFilterFieldModule = "module"

	CronJobWhereNameEquals = "name = ?"

	CronPanicMessageTemplate = "panic: %v"
	CronJobNotFoundMessage   = "cron job not found: "

	// ── Trace 分页/状态常量 ──
	TraceListPageSize  = 20
	TraceEventPageSize = 50
	TraceAgentCodex    = "codex"
	TraceAgentClaude   = "claude"

	// ── Trace hook 事件名 ──
	TraceEventSessionStart       = "SessionStart"
	TraceEventUserPromptSubmit   = "UserPromptSubmit"
	TraceEventPreToolUse         = "PreToolUse"
	TraceEventPermissionRequest  = "PermissionRequest"
	TraceEventPostToolUse        = "PostToolUse"
	TraceEventStop               = "Stop"
	TraceEventSubagentStart      = "SubagentStart"
	TraceEventSubagentStop       = "SubagentStop"
	TraceEventPreCompact         = "PreCompact"
	TraceEventPostCompact        = "PostCompact"
	TraceEventSessionEnd         = "SessionEnd"
	TraceEventPostToolUseFailure = "PostToolUseFailure"

	// ── Claude transcript 记录类型与事件名 ──
	TraceClaudeRecordUser                = "user"
	TraceClaudeRecordAssistant           = "assistant"
	TraceClaudeRecordAttachment          = "attachment"
	TraceClaudeRecordSystem              = "system"
	TraceClaudeRecordPermissionMode      = "permission-mode"
	TraceClaudeRecordFileHistorySnapshot = "file-history-snapshot"
	TraceClaudeRecordLastPrompt          = "last-prompt"
	TraceClaudeRecordSummary             = "summary"
	TraceClaudeRecordProgress            = "progress"

	TraceClaudeEventUserPrompt       = "user_prompt"
	TraceClaudeEventToolResult       = "tool_result"
	TraceClaudeEventAssistantMessage = "assistant_message"

	TraceClaudeBlockToolUse = "tool_use"

	TraceRecordSourceHook              = "hook"
	TraceRecordSourceRollout           = "rollout"
	TraceRecordTypeHookEvent           = "hook_event"
	TraceRecordTypeSessionMeta         = "session_meta"
	TraceRecordTypeTurnContext         = "turn_context"
	TraceRecordTypeResponseItem        = "response_item"
	TraceRecordTypeEventMsg            = "event_msg"
	TraceRecordStatusAccepted          = "accepted"
	TraceRecordStatusDuplicate         = "duplicate"
	TraceRecordStatusRejected          = "rejected"
	TraceRecordMessageInvalid          = "invalid record"
	TraceRecordMessageStorageFailed    = "storage failed"
	TraceRecordMessageTraceDeleted     = "trace deleted"
	TraceMetadataAgentID               = "agent_id"
	TraceMetadataAgentType             = "agent_type"
	TraceEventTaskStarted              = "task_started"
	TraceEventTaskComplete             = "task_complete"
	TraceEventTokenCount               = "token_count"
	TraceRecordMessageUnknown          = "unknown record type"
	TraceRolloutTypeSessionMeta        = "session_meta"
	TraceRolloutTypeTurnContext        = "turn_context"
	TraceRolloutTypeResponseItem       = "response_item"
	TraceRolloutTypeEventMsg           = "event_msg"
	TraceRolloutTypeUnknown            = "unknown"
	TracePayloadFieldType              = "type"
	TracePayloadFieldTurnID            = "turn_id"
	TracePayloadFieldCallID            = "call_id"
	TracePayloadFieldArguments         = "arguments"
	TraceConversationEventFunctionCall = "function_call"
	TraceNotFoundMessage               = "trace not found"
)
