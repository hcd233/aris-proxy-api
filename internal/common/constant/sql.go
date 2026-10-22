package constant

const (
	FieldID                          = "id"
	FieldDeletedAt                   = "deleted_at"
	FieldCheckSum                    = "check_sum"
	FieldMessageIDs                  = "message_ids"
	FieldQuestions                   = "questions"
	FieldModelIDs                    = "model_ids"
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
	FieldSupportOpenAIDecision       = "support_openai_decision"
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

const (
	// ModelStatusEnabled 模型列表 status 筛选取值：仅启用
	ModelStatusEnabled = "enabled"
	// ModelStatusDisabled 模型列表 status 筛选取值：仅停用
	ModelStatusDisabled = "disabled"
	// ModelListDefaultSortField 模型列表默认排序列（配合 enum.SortDesc）
	ModelListDefaultSortField = "created_at"
)

const (
	// WhereModelEnabledEquals 按启用状态筛选
	WhereModelEnabledEquals = "enabled = ?"
	// WhereEndpointIDEquals 按所属端点筛选
	WhereEndpointIDEquals = "endpoint_id = ?"

	// WhereEndpointIDIn 按所属端点集合筛选（批量填充 OAuth 凭据状态等）
	WhereEndpointIDIn = "endpoint_id IN (?)"
	// WhereCapabilitiesLike 按输入模态筛选（capabilities 是 text 列 + serializer:json）
	WhereCapabilitiesLike = "capabilities LIKE ?"

	// WhereModelConfigMissing 仅看配置缺失模型（未计价或未填规格；OR 需括号避免与其他条件错拼）。
	// 判定口径须与 aggregate.Model.MissingConfig 保持一致（列表徽标与筛选同源）。
	WhereModelConfigMissing = "(" + FieldPricingCurrency + " = '' OR " + FieldModelContextLength + " = 0)"
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

	SessionRepoFieldsList = []string{FieldID, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs}
	// SessionRepoFieldsDetail / ReadDetail 含 FieldAPIKeyID：归属判定读该列，
	// 漏列会读出 0 并把合法访问误判为越权（名称列仅用于展示）。
	SessionRepoFieldsDetail     = []string{FieldID, FieldAPIKeyName, FieldAPIKeyID, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs, FieldMetadata, FieldScore, FieldScoredAt}
	SessionRepoFieldsReadList   = []string{FieldID, FieldCreatedAt, FieldUpdatedAt, FieldScore}
	SessionRepoFieldsReadDetail = []string{FieldID, FieldAPIKeyName, FieldAPIKeyID, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs, FieldMetadata, FieldScore, FieldScoredAt}
	SessionRepoFieldsDedup      = []string{FieldID, FieldMessageIDs, FieldToolIDs}
	SessionRepoFieldsSummarize  = []string{FieldID, FieldMessageIDs}

	// SessionRepoFieldsModelIDSync 模型 ID 历史同步的查询投影（只需定位与替换所需列，
	// 不载入 questions/metadata 等大 JSON 列）。
	SessionRepoFieldsModelIDSync = []string{FieldID, FieldModelIDs, FieldMessageIDs}

	// SessionRepoFieldsTerminalScan 终态清理窗口扫描的查询列（不需要 tool_ids）
	SessionRepoFieldsTerminalScan = []string{FieldID, FieldMessageIDs}

	// SessionFirstMessageIDCondition 按首条消息 ID 查同组会话的条件。
	// 表达式必须与生产索引 idx_sessions_first_msg 的索引表达式
	// ((message_ids::jsonb->>0) WHERE deleted_at = 0) 逐字一致，改动即退化全表扫描。
	SessionFirstMessageIDCondition = "message_ids::jsonb->>0 = ?"

	// 消息数桶固定边界（不含动态上限），与 SessionMessageCountBucketCase 对齐；末桶上限由动态 max 截断
	SessionMessageCountBucketEdges = []int{10, 50, 100, 200, 500}

	EndpointRepoFieldsFull = []string{FieldID, FieldUserID, FieldName, FieldOpenaiBaseURL, FieldAnthropicBaseURL, FieldAPIKey, FieldEndpointAuthType, FieldEndpointVisibility,
		FieldSupportOpenAIChatCompletion, FieldSupportOpenAIResponse, FieldSupportAnthropicMessage, FieldSupportOpenAIDecision,
		FieldCreatedAt, FieldUpdatedAt}

	ModelRepoFieldsFull  = []string{FieldID, FieldUserID, FieldAlias, FieldModelID, FieldModelUpstreamModel, FieldEndpointID, FieldEnabled, FieldModelPriority, FieldModelWeight, FieldModelContextLength, FieldModelMaxOutputTokens, FieldModelCapabilities, FieldModelPricingRules, FieldPricingCurrency, FieldCreatedAt, FieldUpdatedAt}
	ModelRepoFieldsAlias = []string{FieldAlias}

	OAuthCredentialRepoFieldsFull = []string{FieldID, FieldOAuthCredentialEndpointID, FieldOAuthCredentialProvider,
		FieldOAuthCredentialAccessTokenEnc, FieldOAuthCredentialRefreshTokenEnc, FieldOAuthCredentialExpiresAt,
		FieldOAuthCredentialAccountRef, FieldOAuthCredentialAccountEmail, FieldOAuthCredentialPlanType,
		FieldOAuthCredentialStatus, FieldOAuthCredentialLastError, FieldOAuthCredentialQuotaSnapshot,
		FieldOAuthCredentialQuotaUpdatedAt, FieldCreatedAt, FieldUpdatedAt}

	// OAuthCredentialRepoFieldsDisplay 凭据展示投影列（不含 token 密文列，读取时无需解密）
	OAuthCredentialRepoFieldsDisplay = []string{FieldOAuthCredentialEndpointID, FieldOAuthCredentialProvider,
		FieldOAuthCredentialAccountEmail, FieldOAuthCredentialPlanType, FieldOAuthCredentialStatus,
		FieldOAuthCredentialLastError, FieldOAuthCredentialQuotaSnapshot, FieldOAuthCredentialQuotaUpdatedAt}

	// OAuthCredentialUpdateColumns upsert 冲突时更新的列（endpoint_id 为冲突键，不在列内；
	// 最早创建时间不被覆盖，故不含 created_at）
	OAuthCredentialUpdateColumns = []string{FieldOAuthCredentialProvider, FieldOAuthCredentialAccessTokenEnc,
		FieldOAuthCredentialRefreshTokenEnc, FieldOAuthCredentialExpiresAt, FieldOAuthCredentialAccountRef,
		FieldOAuthCredentialAccountEmail, FieldOAuthCredentialPlanType, FieldOAuthCredentialStatus,
		FieldOAuthCredentialLastError, FieldOAuthCredentialQuotaSnapshot, FieldOAuthCredentialQuotaUpdatedAt,
		FieldUpdatedAt}

	ProxyAPIKeyRepoFieldsFull = []string{FieldID, FieldUserID, FieldName, FieldKey, FieldCreatedAt}

	// ProxyAPIKeyRepoFieldsAuth 鉴权路径投影。
	// FieldName 必须在列表内：APIKeyMiddleware 依赖 apiKey.Name 注入
	// CtxKeyAPIKeyName，缺列会让 sessions.api_key_name 静默写成空串
	// （2026-07 起 2623+ 条会话因此对所有普通用户不可见）。
	ProxyAPIKeyRepoFieldsAuth = []string{FieldID, FieldUserID, FieldName}

	AuditRepoFieldIDQualified        = "model_call_audits.id"
	AuditRepoFieldCreatedAtQualified = "model_call_audits.created_at"

	AuditRepoFields = []string{AuditRepoFieldIDQualified, FieldAPIKeyID, FieldModelID, FieldUpstreamProtocol, FieldAPIProtocol, FieldEndpoint, FieldInputTokens, FieldOutputTokens, FieldCacheCreationInputTokens, FieldCacheReadInputTokens, FieldFirstTokenLatencyMs, FieldStreamDurationMs, FieldUserAgent, FieldUpstreamStatusCode, FieldErrorMessage, FieldTraceID, FieldCostMicro, FieldPricingCurrency, AuditRepoFieldCreatedAtQualified}

	AuditQueryFields = []string{FieldTraceID, FieldModelID}

	AuditFilterFieldUser   = "user"
	AuditFilterFieldModel  = "model"
	AuditFilterFieldStatus = "status"
	AuditFilterFieldUA     = "ua"

	// ModelListSortFields 模型列表允许的排序列白名单。
	//
	// 不可用 util.SafeSortField 代替：它只校验字符集，api_key 之类敏感列同样放行。
	// 白名单外的取值回退 ModelListDefaultSortField，不报错（避免前端拼错导致整页 500）。
	ModelListSortFields = []string{"alias", "context_length", "max_output_tokens", "created_at", "endpoint_id", "enabled"}

	TriggerRepoFieldsFull = []string{FieldID, FieldWord, FieldHitCount, FieldAction, FieldCreatedAt, FieldUpdatedAt}

	AuditMaxPageSize = 500

	// AuditActiveCreatedIndexDDL admin 审计列表 COUNT(*)/分页的部分索引（手工执行）。
	//
	// GORM tag 无法表达 WHERE deleted_at = 0 的部分索引；2026-10-23 实测 82K 行上
	// COUNT(*) 走 Seq Scan 111ms，部分索引后可走 index-only scan（数 ms）。
	// 不接入 AutoMigrate：按运维约定在生产手工执行，CONCURRENTLY 建索引避免部署期锁表。
	AuditActiveCreatedIndexDDL = "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_mca_active_created ON model_call_audits (created_at DESC) WHERE deleted_at = 0"

	SessionMaxPageSize = 500

	// UpstreamGroupModelLimit upstream 分组视图单个分组返回的模型数上限（防御性兜底）。
	// ponytail: 上限写死 200；需要真分页时再引入组内游标
	UpstreamGroupModelLimit = 200

	// SessionTableName / TraceTableName 归属回填的目标表名（仅供包内常量拼接，
	// 不得接受外部输入，避免注入面）。
	SessionTableName = "sessions"
	TraceTableName   = "traces"

	// APIKeyIDBackfillSQLTemplate 按 api_key_name 幂等回填 api_key_id（%s 为目标表名，
	// 出现 4 次）。规则「宁漏勿越」：
	//   - 仅 api_key_id=0、api_key_name 非空、目标行未软删
	//   - 名称在 proxy_api_keys **全表（含已软删）** 恰好 1 条：历史上若有他人同名
	//     key（即使已删），该名称下的会话归属无法确定，必须跳过
	//   - 该唯一记录必须存活（deleted_at=0）
	// 第二次执行时目标行 api_key_id 已非 0，不再命中，天然幂等。
	// 相关子查询写法 sqlite 与 PostgreSQL 均支持。
	APIKeyIDBackfillSQLTemplate = `UPDATE %s SET api_key_id = (
		SELECT k.id FROM proxy_api_keys k
		WHERE k.name = %s.api_key_name AND k.deleted_at = 0
	)
	WHERE api_key_id = 0 AND api_key_name <> '' AND deleted_at = 0
	AND (SELECT COUNT(*) FROM proxy_api_keys k WHERE k.name = %s.api_key_name) = 1
	AND EXISTS (SELECT 1 FROM proxy_api_keys k WHERE k.name = %s.api_key_name AND k.deleted_at = 0)`

	// WhereUserIDEquals user_id 显式等值条件。
	// 必须用 "user_id = ?" 而非 Where("user_id", v)：GORM 的 Where("col", v) 在
	// 无占位符时 v 会被静默丢弃（v=0 时退化为 WHERE user_id，语义完全错误）。
	WhereUserIDEquals = "user_id = ?"

	// WhereAPIKeyIDEquals api_key_id 显式等值条件（会话归属组键、租户隔离过滤）
	WhereAPIKeyIDEquals = "api_key_id = ?"

	// WhereOAuthCredentialStatusEquals OAuth 凭据状态显式等值条件
	WhereOAuthCredentialStatusEquals = "status = ?"

	// WhereEndpointVisibilityEquals 端点可见性显式等值条件（参数为 visibility 枚举值）
	WhereEndpointVisibilityEquals = "visibility = ?"

	// WhereModelInPublicEndpoint 模型所属端点公开且未删除（参数为 visibility 枚举值）。
	// 用子查询而非 join：与读模型路径保持同一写法，models 表的过滤条件留在单表上。
	WhereModelInPublicEndpoint = "endpoint_id IN (SELECT id FROM endpoints WHERE visibility = ? AND deleted_at = 0)"

	// ── replaceHistoricalModelIDs（由 UpdateWithHistorySync 在事务内调用）条件与块大小 ──
	WhereModelIDEquals = "model_id = ?"
	WhereAPIKeyIDIn    = "api_key_id IN (?)"
	// WhereSessionKeyAndModel session 历史同步的圈定条件。LIKE 模式必须按存储字节
	// （JSON 转义形式）构造并转义通配符（见 repository.likeJSONSubstring），
	// ESCAPE 子句让 % _ \ 按字面匹配（sqlite/PG 均支持）。
	WhereSessionKeyAndModel = "api_key_name IN (?) AND model_ids LIKE ? ESCAPE '\\'"
	WhereMessageIDAndModel  = "model_id = ? AND id IN (?)"
	// WhereNameInAndUserIDNotEquals 查找与其他用户同名的 key（跨租户冲突名剔除）。
	WhereNameInAndUserIDNotEquals = "name IN (?) AND user_id <> ?"
	ModelIDSyncINChunkSize        = 500

	// SessionListINChunkSize session 列表「空 summary fallback」批量加载消息时，
	// 每条 SELECT ... WHERE id IN (?) 携带的 ID 上限。
	//
	// 选 5000 的原因：PG 单语句 bind param 上限是 65535；5000 远低于上限，
	// 保证每条 SQL 的 IN 列表与解析开销可控。当输入 ~12000 IDs 时切分为 3 块，
	// 远少于旧实现 FindInBatches(500) 的 24 次顺序往返。
	SessionListINChunkSize = 5000

	// SessionSummarySelect session 列表投影。
	//
	// 设计要点（perf/session-list-split-count-2026-10-23，BENCHMARK §3.6）：
	//   total 由独立的 COUNT(*) 查询产出，投影里**不再**带 COUNT(*) OVER ()。
	//   窗口函数在 LIMIT 之前求值：PG 必须对 WHERE 命中的全部行计算
	//   jsonb_array_length(message_ids)（每行一次 TOAST 解压）后再 top-N 排序；
	//   2026-10-23 实测 11K 会话表上 119.6ms，拆分后 COUNT 1.0ms + 分页 4.1ms。
	//   分页 SELECT 只解压当前页 20 行，message_count/tool_count 仍实时计算。
	SessionSummarySelect = "id, created_at, updated_at, score, COALESCE(jsonb_array_length(message_ids::jsonb), 0) AS message_count, COALESCE(jsonb_array_length(tool_ids::jsonb), 0) AS tool_count, questions, model_ids"

	// SessionKeywordMatchIDsSQL keyword 搜索第一段：关键词 → 用户消息 id 集合。
	//
	// 只搜 role='user' 的消息正文（->>'content'）：
	//   - 与「只搜用户消息」的产品语义一致，搜索面为 11.5K 行/127MB（全表 164K 行/1GB）；
	//   - 与部分索引 idx_messages_user_content_trgm（WHERE role='user'）谓词对齐，才能走索引；
	//   - 不搜 message 全文 JSON，避免 role/tool_call 等元数据造成误报（搜 "user" 命中全部用户消息）。
	//
	// 多模态内容（content 为 parts 数组）第一段 ILIKE 命中的是 JSON 文本，
	// 会误匹配 "type"/"text" 这类键名与 image/file 字段，故追加 CASE 只比对 text 字段：
	// 字符串 content 直接放行（ILIKE 比的已是真实正文），数组 content 要求某个 part 的
	// text 命中。粗筛仍走 trigram 索引，精筛只在候选行上执行。
	SessionKeywordMatchIDsSQL = "SELECT id FROM messages" +
		" WHERE message::jsonb->>'role' = 'user'" +
		" AND message::jsonb->>'content' ILIKE ? ESCAPE '\\'" +
		" AND CASE jsonb_typeof(message::jsonb->'content')" +
		" WHEN 'array' THEN EXISTS (SELECT 1 FROM jsonb_array_elements(message::jsonb->'content') AS part WHERE part->>'text' ILIKE ? ESCAPE '\\')" +
		" ELSE TRUE END"

	// PgTrgmExtensionDDL keyword 表达式索引依赖的操作符类扩展
	PgTrgmExtensionDDL = "CREATE EXTENSION IF NOT EXISTS pg_trgm"

	// MessageContentTrigramIndexDDL keyword 搜索第一段的 pg_trgm 表达式索引。
	//
	// GORM tag 表达不了「表达式 + 操作符类 + 部分索引谓词」，此前只在生产手工建过：
	// 新环境缺索引时 keyword 搜索退化为全表扫描。谓词必须与 SessionKeywordMatchIDsSQL
	// 的 role='user' 条件一致，否则查询用不上该部分索引。
	MessageContentTrigramIndexDDL = "CREATE INDEX IF NOT EXISTS idx_messages_user_content_trgm ON messages USING gin ((message::jsonb->>'content') gin_trgm_ops) WHERE message::jsonb->>'role' = 'user'"

	// SessionKeywordFilterSQL session 列表 keyword 过滤 SQL 片段（第二段）。
	//
	// 设计沿革：
	//   - 2026-06-07 之前：EXISTS + jsonb_exists 强相关，O(sessions × messages) 全表 ILIKE。
	//   - refactor/session-list-keyword-perf-2026-06-07 → JOIN messages 形态：planner 被
	//     trigram 索引误导选 bitmap 回表，高频词（Task:）27.4s 超时。
	//   - bugfix/session-list-keyword-perf-2026-08-02 → IN 子查询形态：当时统计下 planner
	//     以 arr 驱动主键点查（22ms）。但统计漂移后退化为 correlated SubPlan，
	//     messages 的 ILIKE 扫描对每条 session 重复执行（2026-10-15 生产实测 10s~60s+）。
	//   - bugfix/session-keyword-search-perf-2026-10-15（本次）→ = ANY(?::bigint[]) 常量数组：
	//     匹配集由应用层两段式查询（SessionKeywordMatchIDsSQL + 读缓存）传入，
	//     片段内无子查询，correlated SubPlan 从结构上不可能产生。
	//
	// 占位符约束（fix #58/#59）：
	//   - 整段 SQL 只能有 1 个 '?'（gorm 占位符），命中 id 数组以 "{1,2,3}" 字符串传入；
	//   - 不要写 messages.id = ANY(sessions.message_ids)：message_ids 是 jsonb 文本非
	//     原生数组，会触发 SQLSTATE 42809。
	SessionKeywordFilterSQL = "EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid) WHERE arr.mid::bigint = ANY(?::bigint[]))"

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
	AuditDistinctWhereUser       = "u.name LIKE ? ESCAPE '\\' OR u.email LIKE ? ESCAPE '\\'"
	AuditDistinctWhereModel      = "model_id LIKE ? ESCAPE '\\'"
	AuditDistinctWhereUA         = "user_agent LIKE ? ESCAPE '\\'"
	AuditDistinctWhereUANotEmpty = "user_agent <> ''"
	AuditDistinctSelectModel     = "DISTINCT model_id"
	AuditDistinctSelectStatus    = "DISTINCT upstream_status_code::text"

	// SqlLikeEscapeSuffix 动态列名的 LIKE 片段后缀（列名只能来自白名单常量，禁止拼用户输入）。
	// 统一显式 ESCAPE，不依赖数据库默认转义符——PG 默认是反斜杠，但 sqlite/其他方言不一致。
	SqlLikeEscapeSuffix = " LIKE ? ESCAPE '\\'"

	// DemoAuditDistinctWhereAction / Module demo 审计下拉选项的关键词过滤（与 cron 审计同形）
	DemoAuditDistinctWhereAction = "action LIKE ? ESCAPE '\\'"
	DemoAuditDistinctWhereModule = "module LIKE ? ESCAPE '\\'"
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
	SessionDistinctModelLike   = "jsonb_array_elements_text(model_ids::jsonb) ILIKE ? ESCAPE '\\'"
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
	CronAuditDistinctWhereType    = "cron_name LIKE ? ESCAPE '\\'"
	CronAuditDistinctOrderType    = "cron_name ASC"
	CronAuditDistinctSelectStatus = "DISTINCT status"
	CronAuditDistinctWhereStatus  = "status LIKE ? ESCAPE '\\'"
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
