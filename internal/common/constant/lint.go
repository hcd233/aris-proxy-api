package constant

// lint rule identifiers
const (
	RuleMagicNumber       = "magic.number"
	RuleMagicString       = "magic.string"
	RuleMagicDuration     = "magic.duration"
	RuleAnonymousStruct   = "anonymous_struct"
	RuleLocalConst        = "style.local_const"
	RuleCommentedCode     = "style.commented_code"
	RuleImplementation    = "style.implementation_name"
	RuleTypeAlias         = "style.type_alias"
	RuleShortFunctionBody = "style.short_function_body"

	RuleErrorDeprecatedConstant = "error.deprecated_constant"
	RuleConstantForwarding      = "constant.forwarding"

	RuleDomainDependency            = "architecture.domain_dependency"
	RuleApplicationDependency       = "architecture.application_dependency"
	RuleDeprecatedApplicationImport = "architecture.deprecated_application_import"
	RuleHandlerDB                   = "architecture.handler_db"
	RuleHandlerDomainDirect         = "architecture.handler_domain_direct"
	RuleHandlerAppDirect            = "architecture.handler_app_direct"
	RuleDTODependency               = "architecture.dto_dependency"
	RuleDatabaseModelDependency     = "architecture.database_model_dependency"
	RuleRootContext                 = "architecture.root_context"
	RuleDBRootContext               = "architecture.db_root_context"
	RulePassthrough                 = "architecture.passthrough"

	RuleHardcodedURL       = "hardcoded.url"
	RuleHardcodedErrorCode = "hardcoded.error_code"

	RuleLoggingPrefix         = "logging.prefix"
	RuleLoggingFormat         = "logging.format"
	RuleLoggingChinese        = "logging.chinese"
	RuleLoggingSensitive      = "logging.sensitive"
	RuleLoggingZapLoggerParam = "logging.zap_logger_param"

	RuleTestingInternalFile = "testing.internal_file"
	RuleTestingRootFile     = "testing.root_file"
	RuleTestingTestify      = "testing.testify"
	RuleTestingSleep        = "testing.sleep"

	RuleDTONaming = "architecture.dto_naming"
)
