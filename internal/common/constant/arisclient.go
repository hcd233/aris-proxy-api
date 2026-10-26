package constant

import "time"

const (
	ArisClientOSDarwin = "darwin"
	ArisClientOSLinux  = "linux"

	ArisClientArchAMD64 = "amd64"
	ArisClientArchARM64 = "arm64"

	ArisClientRootDirName    = ".aris"
	ArisClientConfigFileName = "config.json"
	// ArisClientLegacyConfigDirName 旧版客户端配置目录名，仅用于兼容读取。
	ArisClientLegacyConfigDirName = "trace"
	ArisClientBinaryFileName      = "aris"

	ArisClientSchemeHTTP  = "http"
	ArisClientSchemeHTTPS = "https"

	// ArisClientCheckPath 客户端校验 API Key 的绝对路径。
	//
	// 与服务端注册路径同源派生（CLI 前缀 + 组内 RoutePath），不得各自硬编码——
	// 路径改名只需改 CLIAPIPrefix 与对应 RoutePath（CR M1）。
	ArisClientCheckPath = CLIAPIPrefix + ArisClientCheckRoutePath

	ClientModelsListPath         = ClientModelsAPIPrefix + ClientModelsRoutePath
	ClientModelsListMaxBodyBytes = 1 << 20

	ArisClientHTTPTimeout = 5 * time.Second

	ArisClientAPIKeyEnv  = "ARIS_API_KEY"
	ArisClientDevTTYPath = "/dev/tty"

	ArisClientInitNonInteractiveMessage = "aris init requires an interactive terminal"
	ArisClientInstallOriginErrorMessage = "Failed to determine server origin."
	ArisClientInstallGenErrorMessage    = "Failed to generate install script."
	ArisClientInstallScriptTmplName     = "install"
	ArisClientInitMissingAPIKeyMessage  = "api key is required"
	ArisClientInitAPIKeyFailed          = "API key validation failed"
	ArisClientInitRetryPrompt           = "Connection failed. Retry?"
	ArisClientInitAPIKeyRetryPrompt     = "Retry API key?"
	ArisClientInitDone                  = "Configuration completed"
	ArisClientInitConfigFormat          = "Config: %s"
	ArisClientJSONIndent                = "  "

	ArisClientInitTitleConnect        = "Connect to server"
	ArisClientInitTitleAPIKey         = "Configure API key"
	ArisClientInitSteps               = 3
	ArisClientInitConnectingFormat    = "Connecting to %s..."
	ArisClientReachableFormat         = "reachable (%s)"
	ArisClientInitAPIKeyTitle         = "API key"
	ArisClientInitKeepAPIKeyHint      = "press Enter to keep current"
	ArisClientInitValidatingKey       = "Validating API key..."
	ArisClientInitContinueLabel       = "Continue"
	ArisClientInitCancelLabel         = "Cancel"
	ArisClientInitHostPrompt          = "Server host"
	ArisClientInitHostPlaceholder     = "https://aris.example.com"
	ArisClientInitHostSchemeMessage   = "host must start with http:// or https://"
	ArisClientInitHostRequiredMessage = "host is required"
)
