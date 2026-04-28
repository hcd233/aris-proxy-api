package constant

// lint rule identifiers
const (
	RuleMagicNumber     = "magic.number"
	RuleMagicString     = "magic.string"
	RuleMagicDuration   = "magic.duration"
	RuleAnonymousStruct = "anonymous_struct"
	RuleLocalConst      = "style.local_const"
	RuleCommentedCode   = "style.commented_code"
	RuleImplementation  = "style.implementation_name"
	RuleTypeAlias       = "style.type_alias"

	RuleErrorDeprecatedConstant = "error.deprecated_constant"
	RuleConstantForwarding      = "constant.forwarding"

	RuleDomainDependency            = "architecture.domain_dependency"
	RuleDeprecatedApplicationImport = "architecture.deprecated_application_import"
	RuleHandlerDB                   = "architecture.handler_db"
	RuleRootContext                 = "architecture.root_context"
	RulePassthrough                 = "architecture.passthrough"

	RuleLoggingPrefix    = "logging.prefix"
	RuleLoggingFormat    = "logging.format"
	RuleLoggingChinese   = "logging.chinese"
	RuleLoggingSensitive = "logging.sensitive"

	RuleTestingInternalFile = "testing.internal_file"
	RuleTestingRootFile     = "testing.root_file"
	RuleTestingTestify      = "testing.testify"
	RuleTestingSleep        = "testing.sleep"
)
