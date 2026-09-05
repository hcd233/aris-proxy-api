package constant

import "time"

const (
	ArisClientOSDarwin = "darwin"
	ArisClientOSLinux  = "linux"

	ArisClientArchAMD64 = "amd64"
	ArisClientArchARM64 = "arm64"

	ArisClientRootDirName         = ".aris"
	ArisClientBinDirName          = "bin"
	ArisClientTraceDirName        = "trace"
	ArisClientSpoolDirName        = "spool"
	ArisClientPendingDirName      = "pending"
	ArisClientStateDirName        = "state"
	ArisClientRejectedDirName     = "rejected"
	ArisClientLogDirName          = "logs"
	ArisClientConfigFileName      = "config.json"
	ArisClientBinaryFileName      = "aris"
	ArisClientAgentCodex          = "codex"
	ArisClientCodexDirName        = ".codex"
	ArisClientCodexHooksFile      = "hooks.json"
	ArisClientCodexBackupSuffix   = ".bak"
	ArisClientClaudeDirName       = ".claude"
	ArisClientClaudeSettingsFile  = "settings.json"
	ArisClientHooksField          = "hooks"
	ArisClientHookTypeCommand     = "command"
	ArisClientIngestCommandSuffix = " trace ingest"
	ArisClientHookTimeout         = 30

	ArisClientSchemeHTTP  = "http"
	ArisClientSchemeHTTPS = "https"

	// ArisClientIngestPath/CheckPath 客户端上报接口绝对路径。
	//
	// 与服务端注册路径同源派生（CLI 前缀 + 组内 RoutePath），不得各自硬编码——
	// 路径改名只需改 CLIAPIPrefix 与对应 RoutePath（CR M1）。
	ArisClientCheckPath            = CLIAPIPrefix + ArisClientCheckRoutePath
	ClientModelsListPath           = ClientModelsAPIPrefix + ClientModelsRoutePath
	ClientModelsListMaxBodyBytes   = 1 << 20
	ArisClientHTTPTimeout          = 5 * time.Second
	ArisClientSpoolLimit           = 256 << 20
	ArisClientBatchMaxRecords      = 500
	ArisClientBatchMaxBytes        = 4 << 20
	ArisClientRejectedRetention    = 7 * 24 * time.Hour
	ArisClientIngestPath           = CLIAPIPrefix + ArisClientIngestRoutePath
	ArisClientRecordFileSuffix     = ".json"
	ArisClientSpoolLockFile        = "spool.lock"
	ArisClientStateLockFile        = "client.lock"
	ArisClientStateFileName        = "client.json"
	ArisClientSpoolIDRandomBytes   = 16
	ArisClientHookInputLimit       = 16 << 20
	ArisClientStopTrimKey          = "last_assistant_message"
	ArisClientHookDedupFormat      = "hook:%s:%d"
	ArisClientLogPrefix            = "trace-"
	ArisClientLogSuffix            = ".log"
	ArisClientLogDateFormat        = "2006-01-02"
	ArisClientLogCategoryIngest    = "ingest_failed"
	ArisClientLogCategoryRollout   = "rollout_failed"
	ArisClientLogLineFormat        = "%s %s\n"
	ArisClientTranscriptStateDir   = "transcripts"
	ArisClientTranscriptLockSuffix = ".lock"
	// ArisClientSessionMetaDir/Suffix/LockSuffix per-session hook 元数据状态文件
	// （codex hook 纯触发后 model/cwd/source 不再随 hook 记录上报，持久化后由 flush 读取）
	ArisClientSessionMetaDir         = "sessions"
	ArisClientSessionMetaSuffix      = ".meta"
	ArisClientSessionMetaLockSuffix  = ".lock"
	ArisClientRolloutFileSuffix      = ".jsonl"
	ArisClientRolloutDedupFormat     = "rollout:%s:%d:%s"
	ArisClientSessionMetaDedupFormat = "rollout:%s:session_meta:%s"
	ArisClientTokenCountDedupFormat  = "token_count:%s"
	ArisClientFileIdentityFormat     = "%d:%d"

	ArisClientAPIKeyEnv  = "ARIS_API_KEY"
	ArisClientDevTTYPath = "/dev/tty"

	ArisClientInitNonInteractiveMessage = "trace init requires an interactive terminal"
	ArisClientInstallOriginErrorMessage = "Failed to determine server origin."
	ArisClientInstallGenErrorMessage    = "Failed to generate install script."
	ArisClientInstallScriptTmplName     = "install"
	ArisClientInitMissingAPIKeyMessage  = "api key is required"
	ArisClientInitAPIKeyFailed          = "API key validation failed"
	ArisClientInitRetryPrompt           = "Connection failed. Retry?"
	ArisClientInitAPIKeyRetryPrompt     = "Retry API key?"
	ArisClientInitDone                  = "Trace configuration completed"
	ArisClientInitApprovalHint          = "In Codex, run /hooks and manually approve the new Aris hooks."
	ArisClientInitConfigFormat          = "Config: %s"
	ArisClientJSONIndent                = "  "

	ArisClientInitTitleConnect        = "Connect to server"
	ArisClientInitTitleAgent          = "Select agent"
	ArisClientInitTitleAPIKey         = "Configure API key"
	ArisClientInitTitleHook           = "Configure hooks"
	ArisClientInitSteps               = 4
	ArisClientInitConnectingFormat    = "Connecting to %s..."
	ArisClientReachableFormat         = "reachable (%s)"
	ArisClientInitAgentSelectTitle    = "Agent"
	ArisClientInitAgentOptionCodex    = "Codex"
	ArisClientInitAgentOptionClaude   = "Claude Code"
	ArisClientInitAgentRequired       = "select at least one agent"
	ArisClientInitContinueLabel       = "Continue"
	ArisClientInitCancelLabel         = "Cancel"
	ArisClientInitAPIKeyTitle         = "API key"
	ArisClientInitKeepAPIKeyHint      = "press Enter to keep current"
	ArisClientInitValidatingKey       = "Validating API key..."
	ArisClientInitInstallingHooks     = "Installing hooks..."
	ArisClientInitHooksFormat         = "%s hooks: %d/%d registered"
	ArisClientInitClaudeApprovalHint  = "Claude Code picks up ~/.claude/settings.json hooks automatically; review them with /hooks."
	ArisClientInitHostPrompt          = "Server host"
	ArisClientInitHostPlaceholder     = "https://aris.example.com"
	ArisClientInitHostSchemeMessage   = "host must start with http:// or https://"
	ArisClientInitHostRequiredMessage = "host is required"
)

// ArisClientCodexHookEvents aris hook 需要注册的 codex 事件
var ArisClientCodexHookEvents = []string{
	TraceEventSessionStart,
	TraceEventStop,
	TraceEventSubagentStop,
}

// ArisClientClaudeHookEvents aris hook 需要注册的 claude 事件
var ArisClientClaudeHookEvents = []string{
	TraceEventSessionStart,
	TraceEventUserPromptSubmit,
	TraceEventPreToolUse,
	TraceEventPostToolUse,
	TraceEventPostToolUseFailure,
	TraceEventStop,
	TraceEventSubagentStart,
	TraceEventSubagentStop,
	TraceEventPreCompact,
	TraceEventPostCompact,
	TraceEventSessionEnd,
}
