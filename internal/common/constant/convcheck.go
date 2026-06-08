package constant

// convcheck.go — lintconv 内部实现常量
// 路径常量
const (
	ConvCheckPathInternal   = "internal"
	ConvCheckPrefixBracket  = "["
	ConvCheckPathConstant   = "internal/common/constant"
	ConvCheckPathCommonEnum = "internal/common/enum"
	ConvCheckPathEnum       = "internal/enum"
	ConvCheckPathRouter     = "internal/router"
)

const (
	ConvCheckSuffixTestGo   = "_test.go"
	ConvCheckPathHandler    = "internal/handler"
	ConvCheckPathMiddleware = "internal/middleware"
	ConvCheckPathDTO        = "internal/dto"
	ConvCheckPathApp        = "internal/application"
	ConvCheckPathDomain     = "internal/domain"
	ConvCheckPathTool       = "internal/tool"
	ConvCheckPathTest       = "test/"
	ConvCheckPathCMD        = "cmd"
	ConvCheckPathIerr       = "internal/common/ierr"
	ConvCheckPathConfig     = "internal/config"
	ConvCheckVOSep          = "/vo/"
)

// 接收者名称常量
const (
	ConvCheckRecvLogger  = "logger"
	ConvCheckRecvContext = "context"
	ConvCheckRecvIerr    = "ierr"
	ConvCheckRecvZap     = "zap"
	ConvCheckRecvDAO     = "dao"
	ConvCheckRecvDB      = "database"
	ConvCheckRecvTime    = "time"
	ConvCheckRecvReflect = "reflect"
	ConvCheckRecvHTTP    = "http"
	ConvCheckRecvLog     = "log"
	ConvCheckRecvModel   = "model"
)

// logger 方法名
const (
	ConvCheckLogInfo  = "Info"
	ConvCheckLogError = "Error"
	ConvCheckLogWarn  = "Warn"
	ConvCheckLogDebug = "Debug"
)

// logger 链式方法名
const (
	ConvCheckLogWithCtx  = "WithCtx"
	ConvCheckLogWithFCtx = "WithFCtx"
)

// 通用方法名
const (
	ConvCheckMethodMaskSecret    = "MaskSecret"
	ConvCheckMethodString        = "String"
	ConvCheckMethodSchema        = "Schema"
	ConvCheckMethodTypeFor       = "TypeFor"
	ConvCheckMethodBackground    = "Background"
	ConvCheckMethodTODO          = "TODO"
	ConvCheckMethodSleep         = "Sleep"
	ConvCheckMethodGetDBInstance = "GetDBInstance"
	ConvCheckMethodNewError      = "NewError"
)

// ierr 构造方法名
const (
	ConvCheckIerrWrap  = "Wrap"
	ConvCheckIerrWrapf = "Wrapf"
	ConvCheckIerrNew   = "New"
	ConvCheckIerrNewf  = "Newf"
)

// error 检查常量
const (
	ConvCheckPathErrorGo           = "internal/common/constant/error.go"
	ConvCheckIdentConstant         = "constant"
	ConvCheckPrefixErr             = "Err"
	ConvCheckMsgDeprecatedConstErr = "constant.ErrXxx is deprecated, use ierr.ErrXxx.BizError()"
	ConvCheckTokConst              = "const"
	ConvCheckTokType               = "type"
	ConvCheckMsgForwardingConst    = "forwarding const X = pkg.Y is prohibited in constant/enum, use the original constant directly"
	ConvCheckErrorPrefix           = "Error"
)

// style 检查常量
const (
	ConvCheckPrefixComment   = "//"
	ConvCheckPrefixAtSign    = "@"
	ConvCheckPrefixPackage   = "Package "
	ConvCheckPrefixGoColon   = "go:"
	ConvCheckPrefixNolint    = "nolint"
	ConvCheckPrefixAuthor    = "author "
	ConvCheckPrefixUpdate    = "update "
	ConvCheckPrefixReceiver  = "receiver "
	ConvCheckPrefixParam     = "param "
	ConvCheckPrefixReturnDoc = "return "

	ConvCheckPrefixFunc      = "func "
	ConvCheckPrefixIf        = "if "
	ConvCheckPrefixFor       = "for "
	ConvCheckPrefixVar       = "var "
	ConvCheckPrefixTypeKw    = "type "
	ConvCheckPrefixConstKw   = "const "
	ConvCheckPrefixSwitch    = "switch "
	ConvCheckPrefixCase      = "case "
	ConvCheckPrefixReturn    = "return "
	ConvCheckPrefixErrAssign = "err :="
	ConvCheckPrefixErrEq     = "err ="
	ConvCheckPrefixCtxDot    = "ctx."
	ConvCheckPrefixReqDot    = "req."
	ConvCheckPrefixRspDot    = "rsp."

	ConvCheckNameStateMap             = "stateMap"
	ConvCheckNameChoiceMap            = "choiceMap"
	ConvCheckNameToolCallMap          = "toolCallMap"
	ConvCheckNameBlockMap             = "blockMap"
	ConvCheckNameBlackList            = "blackList"
	ConvCheckNameWhiteList            = "whiteList"
	ConvCheckNameAllowList            = "allowList"
	ConvCheckNameDenyList             = "denyList"
	ConvCheckNameBodyMap              = "bodyMap"
	ConvCheckNameDataMap              = "dataMap"
	ConvCheckNameMsgMap               = "msgMap"
	ConvCheckNameMessageMap           = "messageMap"
	ConvCheckNameToolMap              = "toolMap"
	ConvCheckNameExistingMap          = "existingMap"
	ConvCheckNameSchemaMap            = "SchemaMap"
	ConvCheckNameSpecialNameBlackList = "specialNameblackList"
	ConvCheckNameSpecialNameWhiteList = "specialNamewhiteList"

	ConvCheckSubPathVO = "/vo/"

	ConvCheckMinFunctionBodyLines = 2

	ConvCheckMsgCommentedCode     = "possible dead code in comment, please confirm whether to delete"
	ConvCheckMsgImplementation    = "variable naming may expose implementation details; consider using plural form"
	ConvCheckMsgLocalConst        = "local const blocks are prohibited in business packages; move to internal/common/constant or internal/enum"
	ConvCheckMsgTypeAlias         = "type alias (type X = Y) is only allowed in enum and vo packages"
	ConvCheckMsgTypeDef           = "type definition from another type (type X Y) is only allowed in enum and vo packages"
	ConvCheckMsgShortFunctionBody = "function body has fewer than 1 line; avoid empty wrappers and inline the logic instead"
)

// logging 检查常量
const (
	ConvCheckMsgShouldPrefix       = "log messages should use [ModuleName] prefix"
	ConvCheckMsgAfterModuleName    = "log message after [ModuleName] should be [ModuleName] PascalCaseMessage"
	ConvCheckMsgAfterModuleSpace   = "log message after [ModuleName] should be separated by space and start with uppercase"
	ConvCheckMsgMustStartUppercase = "log message after [ModuleName] must start with uppercase letter"
	ConvCheckMsgMustNotChinese     = "log messages must not contain Chinese characters"
	ConvCheckMsgUseMaskSecret      = "logging Key/Token/Secret/Password must use commonutil.MaskSecret()"
	ConvCheckMsgZapLoggerParam     = "*zap.Logger must not be used as a function parameter; get logger from context or logger package inside the function"
	ConvCheckTypeLogger            = "Logger"

	ConvCheckSensitiveAPIKey   = "apikey"
	ConvCheckSensitiveToken    = "token"
	ConvCheckSensitiveSecret   = "secret"
	ConvCheckSensitivePassword = "password"

	ConvCheckAllowedAPIKeyName        = "apikeyname"
	ConvCheckAllowedTokenType         = "tokentype"
	ConvCheckAllowedTokenExpir        = "tokenexpir"
	ConvCheckAllowedSessionAPIKeyName = "sessionapikeyname"
)

// testing 检查常量
const (
	ConvCheckPrefixTest         = "test/"
	ConvCheckSeparatorSlash     = "/"
	ConvCheckSuffixREQ          = "REQ"
	ConvCheckSuffixRSP          = "RSP"
	ConvCheckSuffixREQUEST      = "REQUEST"
	ConvCheckSuffixRESPONSE     = "RESPONSE"
	ConvCheckPrefixTestify      = "github.com/stretchr/testify"
	ConvCheckMsgTestingInternal = "*_test.go in internal/ is prohibited, place tests under test/ directory"
	ConvCheckMsgTestingRoot     = "*_test.go in test/ root is prohibited, place in a topic subdirectory"
	ConvCheckMsgTestingTestify  = "testify and third-party assertion libraries are prohibited, use standard testing package"
	ConvCheckMsgTestingSleep    = "time.Sleep() is prohibited in tests for synchronization, use channel/WaitGroup/deadline"
)

// architecture 检查常量
const (
	ConvCheckMethodWhere  = "Where"
	ConvCheckMethodFind   = "Find"
	ConvCheckMethodCreate = "Create"
	ConvCheckMethodSave   = "Save"
	ConvCheckFuncInit     = "init"

	ConvCheckImportInfra            = "github.com/hcd233/aris-proxy-api/internal/infrastructure/"
	ConvCheckImportInfraCache       = "github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	ConvCheckImportDatabaseModel    = "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	ConvCheckPathInfrastructure     = "internal/infrastructure"
	ConvCheckPathDatabaseModel      = "internal/infrastructure/database/model"
	ConvCheckMsgDatabaseModelDirect = "database model imports are only allowed in infrastructure layer; expose a domain/application port instead"
	ConvCheckLegacyCronSessionDedup = "internal/cron/session_dedup.go"
	ConvCheckLegacyMiddlewareAPIKey = "internal/middleware/apikey.go"
	ConvCheckLegacyMiddlewareJWT    = "internal/middleware/jwt.go"
	ConvCheckImportDTO              = "github.com/hcd233/aris-proxy-api/internal/dto"
	ConvCheckImportUtil             = "github.com/hcd233/aris-proxy-api/internal/util"
	ConvCheckImportZap              = "go.uber.org/zap"

	ConvCheckImportPathDomain = "github.com/hcd233/aris-proxy-api/internal/domain/"
	ConvCheckImportPathApp    = "github.com/hcd233/aris-proxy-api/internal/application/"
	ConvCheckImportPathPort   = "/port"
	ConvCheckImportHandler    = "github.com/hcd233/aris-proxy-api/internal/handler/"

	ConvCheckPathLintconv = "internal/tool/lintconv"

	ConvCheckDeprecatedImportService   = "github.com/hcd233/aris-proxy-api/internal/service"
	ConvCheckDeprecatedImportConverter = "github.com/hcd233/aris-proxy-api/internal/converter"
	ConvCheckDeprecatedImportProxy     = "github.com/hcd233/aris-proxy-api/internal/proxy"
	ConvCheckDeprecatedImportAgent     = "github.com/hcd233/aris-proxy-api/internal/agent/"
	ConvCheckDeprecatedImportJWT       = "github.com/hcd233/aris-proxy-api/internal/jwt/"
	ConvCheckDeprecatedImportOAuth2    = "github.com/hcd233/aris-proxy-api/internal/oauth2/"

	ConvCheckMsgDomainInfra         = "Domain layer must not depend on Infrastructure layer"
	ConvCheckMsgDomainDTO           = "Domain layer must not depend on DTO layer"
	ConvCheckMsgDomainUtil          = "Domain layer must not depend on internal/util, use internal/common/util instead"
	ConvCheckMsgAppInfraCache       = "Application layer must not depend on infrastructure/cache; define a port in application layer instead"
	ConvCheckMsgDeprecatedAppImport = "Application layer must not import deprecated internal/service/converter/proxy/agent/jwt/oauth2 packages"
	ConvCheckMsgHandlerDB           = "Handler layer must not operate DAO/DB directly; business logic should be in Service layer"
	ConvCheckMsgHandlerDomainDirect = "Handler layer must not import internal/domain directly; depend on application-layer services"
	ConvCheckMsgHandlerAppDirect    = "Handler layer must import only application/*/port packages from internal/application, not internal implementation packages"
	ConvCheckMsgDTODependency       = "DTO layer must not import infrastructure/application/domain/handler; DTO is a transport-layer concern"
	ConvCheckMsgRootContext         = "context.Background()/context.TODO() is prohibited in interface layer, pass context from the caller"
	ConvCheckMsgDBRootContext       = "binding root context to DB is prohibited, use the injected base DB and bind request context at operation time"
	ConvCheckMsgPassthrough         = "passthrough wrapper detected, inline the logic or merge the method"

	ConvCheckMsgDTONaming = "Req/Request/Rsp/Response struct types must be defined in internal/dto, not here"
)

// magic 检查常量
const (
	ConvCheckMagicNumberThreshold = 30

	ConvCheckMsgMagicNumber     = "magic number literal, should be extracted as a named constant"
	ConvCheckMsgMagicString     = "magic string literal, should be extracted as a named constant"
	ConvCheckMsgMagicDuration   = "magic duration multiplier, should be extracted as a named constant"
	ConvCheckMsgAnonymousStruct = "anonymous struct is prohibited, extract as a named type in the package"

	ConvCheckMsgHardcodedURL       = "hardcoded URL literal, move to internal/config and read from environment"
	ConvCheckMsgHardcodedErrorCode = "hardcoded business error code, add a sentinel in internal/common/ierr and use ierr.ErrXxx.BizError()"

	ConvCheckURLPrefixHTTPS = "https://"
	ConvCheckURLPrefixHTTP  = "http://"

	ConvCheckBacktickPrefix = "`"
	ConvCheckEmptyString    = ""
)

// source 常量
const (
	ConvCheckRuleSourceLoad  = "source.load"
	ConvCheckRuleSourceParse = "source.parse"
	ConvCheckGoExt           = ".go"
	ConvCheckCurrentDir      = "."
	ConvCheckRecursiveSuffix = "/..."
	ConvCheckRecursiveOnly   = "..."
	ConvCheckSkipGitDir      = ".git"
	ConvCheckSkipWorktrees   = ".worktrees"
	ConvCheckSkipVendor      = "vendor"
)

// diagnostic 常量
const (
	ConvCheckSeverityError = "error"
	ConvCheckSeverityWarn  = "warning"

	ConvCheckDiagnosticFormat = "%s:%d: [%s] %s: %s\n"
	ConvCheckAllPassed        = "All convention checks passed!"
	ConvCheckSummaryFormat    = "%d error(s), %d warning(s)\n"
	ConvCheckLogPrefix        = "[LintConv] "
	ConvCheckLogViolation     = "convention violation"
	ConvCheckLogPassed        = "All convention checks passed!"
	ConvCheckLogSummary       = "convention check summary"
)
