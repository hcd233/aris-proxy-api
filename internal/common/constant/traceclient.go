package constant

import "time"

const (
	TraceClientOSDarwin = "darwin"
	TraceClientOSLinux  = "linux"

	TraceClientArchAMD64 = "amd64"
	TraceClientArchARM64 = "arm64"

	TraceClientRootDirName         = ".aris"
	TraceClientBinDirName          = "bin"
	TraceClientTraceDirName        = "trace"
	TraceClientSpoolDirName        = "spool"
	TraceClientPendingDirName      = "pending"
	TraceClientStateDirName        = "state"
	TraceClientRejectedDirName     = "rejected"
	TraceClientLogDirName          = "logs"
	TraceClientConfigFileName      = "config.json"
	TraceClientBinaryFileName      = "aris"
	TraceClientAgentCodex          = "codex"
	TraceClientCodexDirName        = ".codex"
	TraceClientCodexHooksFile      = "hooks.json"
	TraceClientCodexBackupSuffix   = ".bak"
	TraceClientClaudeDirName       = ".claude"
	TraceClientClaudeSettingsFile  = "settings.json"
	TraceClientHooksField          = "hooks"
	TraceClientHookTypeCommand     = "command"
	TraceClientIngestCommandSuffix = " trace ingest"
	TraceClientHookTimeout         = 30

	TraceClientSchemeHTTP  = "http"
	TraceClientSchemeHTTPS = "https"

	// TraceClientIngestPath/CheckPath 客户端上报接口绝对路径。
	//
	// 与服务端注册路径同源派生（CLI 前缀 + 组内 RoutePath），不得各自硬编码——
	// 路径改名只需改 CLIAPIPrefix 与对应 RoutePath（CR M1）。
	TraceClientCheckPath            = CLIAPIPrefix + TraceClientCheckRoutePath
	ClientModelsListPath            = ClientModelsAPIPrefix + ClientModelsRoutePath
	ClientModelsListMaxBodyBytes    = 1 << 20
	TraceClientHTTPTimeout          = 5 * time.Second
	TraceClientSpoolLimit           = 256 << 20
	TraceClientBatchMaxRecords      = 500
	TraceClientBatchMaxBytes        = 4 << 20
	TraceClientRejectedRetention    = 7 * 24 * time.Hour
	TraceClientIngestPath           = CLIAPIPrefix + TraceClientIngestRoutePath
	TraceClientRecordFileSuffix     = ".json"
	TraceClientSpoolLockFile        = "spool.lock"
	TraceClientStateLockFile        = "client.lock"
	TraceClientStateFileName        = "client.json"
	TraceClientSpoolIDRandomBytes   = 16
	TraceClientHookInputLimit       = 16 << 20
	TraceClientStopTrimKey          = "last_assistant_message"
	TraceClientHookDedupFormat      = "hook:%s:%d"
	TraceClientLogPrefix            = "trace-"
	TraceClientLogSuffix            = ".log"
	TraceClientLogDateFormat        = "2006-01-02"
	TraceClientLogCategoryIngest    = "ingest_failed"
	TraceClientLogCategoryRollout   = "rollout_failed"
	TraceClientLogLineFormat        = "%s %s\n"
	TraceClientTranscriptStateDir   = "transcripts"
	TraceClientTranscriptLockSuffix = ".lock"
	// TraceClientSessionMetaDir/Suffix/LockSuffix per-session hook 元数据状态文件
	// （codex hook 纯触发后 model/cwd/source 不再随 hook 记录上报，持久化后由 flush 读取）
	TraceClientSessionMetaDir         = "sessions"
	TraceClientSessionMetaSuffix      = ".meta"
	TraceClientSessionMetaLockSuffix  = ".lock"
	TraceClientRolloutFileSuffix      = ".jsonl"
	TraceClientRolloutDedupFormat     = "rollout:%s:%d:%s"
	TraceClientSessionMetaDedupFormat = "rollout:%s:session_meta:%s"
	TraceClientTokenCountDedupFormat  = "token_count:%s"
	TraceClientFileIdentityFormat     = "%d:%d"

	TraceClientAPIKeyEnv  = "ARIS_API_KEY"
	TraceClientDevTTYPath = "/dev/tty"

	TraceClientInitNonInteractiveMessage = "trace init requires an interactive terminal"
	TraceClientInstallOriginErrorMessage = "Failed to determine server origin."
	TraceClientInstallGenErrorMessage    = "Failed to generate install script."
	TraceClientInstallScriptTmplName     = "install"
	TraceClientInitMissingAPIKeyMessage  = "api key is required"
	TraceClientInitAPIKeyFailed          = "API key validation failed"
	TraceClientInitRetryPrompt           = "Connection failed. Retry?"
	TraceClientInitAPIKeyRetryPrompt     = "Retry API key?"
	TraceClientInitDone                  = "Trace configuration completed"
	TraceClientInitApprovalHint          = "In Codex, run /hooks and manually approve the new Aris hooks."
	TraceClientInitConfigFormat          = "Config: %s"
	TraceClientJSONIndent                = "  "

	TraceClientInitTitleConnect        = "Connect to server"
	TraceClientInitTitleAgent          = "Select agent"
	TraceClientInitTitleAPIKey         = "Configure API key"
	TraceClientInitTitleHook           = "Configure hooks"
	TraceClientInitSteps               = 4
	TraceClientInitConnectingFormat    = "Connecting to %s..."
	TraceClientReachableFormat         = "reachable (%s)"
	TraceClientInitAgentSelectTitle    = "Agent"
	TraceClientInitAgentOptionCodex    = "Codex"
	TraceClientInitAgentOptionClaude   = "Claude Code"
	TraceClientInitAgentRequired       = "select at least one agent"
	TraceClientInitContinueLabel       = "Continue"
	TraceClientInitCancelLabel         = "Cancel"
	TraceClientInitAPIKeyTitle         = "API key"
	TraceClientInitKeepAPIKeyHint      = "press Enter to keep current"
	TraceClientInitValidatingKey       = "Validating API key..."
	TraceClientInitInstallingHooks     = "Installing hooks..."
	TraceClientInitHooksFormat         = "%s hooks: %d/%d registered"
	TraceClientInitClaudeApprovalHint  = "Claude Code picks up ~/.claude/settings.json hooks automatically; review them with /hooks."
	TraceClientInitHostPrompt          = "Server host"
	TraceClientInitHostPlaceholder     = "https://aris.example.com"
	TraceClientInitHostSchemeMessage   = "host must start with http:// or https://"
	TraceClientInitHostRequiredMessage = "host is required"
)

// TraceClientCodexHookEvents aris hook 需要注册的 codex 事件
var TraceClientCodexHookEvents = []string{
	TraceEventSessionStart,
	TraceEventStop,
	TraceEventSubagentStop,
}

// TraceClientClaudeHookEvents aris hook 需要注册的 claude 事件
var TraceClientClaudeHookEvents = []string{
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
