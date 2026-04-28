package constant

const (
	FieldID             = "id"
	FieldDeletedAt      = "deleted_at"
	FieldCheckSum       = "check_sum"
	FieldMessageIDs     = "message_ids"
	FieldToolIDs        = "tool_ids"
	FieldMetadata       = "metadata"
	FieldSummary        = "summary"
	FieldSummarizeError = "summarize_error"
	FieldScoreVersion   = "score_version"
	FieldCoherenceScore = "coherence_score"
	FieldDepthScore     = "depth_score"
	FieldValueScore     = "value_score"
	FieldTotalScore     = "total_score"
	FieldScoredAt       = "scored_at"
	FieldScoreError     = "score_error"
	FieldName           = "name"
	FieldKey            = "key"
	FieldUserID         = "user_id"
	FieldModel          = "model"
	FieldAPIKey         = "api_key"
	FieldAPIKeyName     = "api_key_name"
	FieldBaseURL        = "base_url"
	FieldProvider       = "provider"
	FieldAlias          = "alias"
	FieldLastLogin      = "last_login"
	FieldCreatedAt      = "created_at"
	FieldUpdatedAt      = "updated_at"
	FieldMessage        = "message"
	FieldTool           = "tool"
	FieldEmail          = "email"
	FieldAvatar         = "avatar"
	FieldPermission     = "permission"
	FieldGithubBindID   = "github_bind_id"
	FieldGoogleBindID   = "google_bind_id"
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
	SessionRepoFieldsDetail     = []string{FieldID, FieldAPIKeyName, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs, FieldMetadata, FieldSummary, FieldSummarizeError, FieldCoherenceScore, FieldDepthScore, FieldValueScore, FieldTotalScore, FieldScoreVersion, FieldScoredAt, FieldScoreError}
	SessionRepoFieldsReadList   = []string{FieldID, FieldCreatedAt, FieldUpdatedAt, FieldSummary, FieldMessageIDs, FieldToolIDs}
	SessionRepoFieldsReadDetail = []string{FieldID, FieldAPIKeyName, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs, FieldMetadata}
	SessionRepoFieldsDedup      = []string{FieldID, FieldMessageIDs, FieldToolIDs}
	SessionRepoFieldsScore      = []string{FieldID, FieldMessageIDs}
	SessionRepoFieldsSummarize  = []string{FieldID, FieldMessageIDs}

	EndpointRepoFieldsFull       = []string{FieldID, FieldAlias, FieldModel, FieldAPIKey, FieldBaseURL, FieldProvider}
	EndpointRepoFieldsAlias      = []string{FieldAlias}
	EndpointRepoFieldsCredential = []string{FieldModel, FieldAPIKey, FieldBaseURL}

	ProxyAPIKeyRepoFieldsFull = []string{FieldID, FieldUserID, FieldName, FieldKey, FieldCreatedAt}
	ProxyAPIKeyRepoFieldsAuth = []string{FieldID, FieldUserID}
)
