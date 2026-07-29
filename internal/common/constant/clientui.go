package constant

// aris 客户端 TUI 语义色（明暗终端自适应；主色呼应 Web 端 Anthropic clay 主题）
const (
	ClientUIColorPrimaryLight = "#C15F3C"
	ClientUIColorPrimaryDark  = "#D97757"
	ClientUIColorSuccessLight = "#3E7C4F"
	ClientUIColorSuccessDark  = "#6FBF73"
	ClientUIColorWarningLight = "#B8860B"
	ClientUIColorWarningDark  = "#E5C07B"
	ClientUIColorErrorLight   = "#B3372F"
	ClientUIColorErrorDark    = "#E06C75"
	ClientUIColorMutedLight   = "#8A8A8A"
	ClientUIColorMutedDark    = "#6E6E6E"
)

// aris 客户端 TUI 状态图标（unicode 符号，非 emoji）
const (
	ClientUIIconOK      = "✓"
	ClientUIIconFail    = "✗"
	ClientUIIconWarn    = "!"
	ClientUIIconSection = "◆"
)

// aris 客户端 TUI 布局格式
const (
	ClientUIRowIndent      = "  "
	ClientUISeparatorComma = ", "
	ClientUIMaskPrefix     = "••••"
	ClientUIBytesBFormat   = "%d B"
	ClientUIBytesKBFormat  = "%.1f KB"
	ClientUIBytesMBFormat  = "%.1f MB"
)

// aris status 面板文案
const (
	ClientUIStatusTitle              = "aris status"
	ClientUIStatusChecking           = "Checking status..."
	ClientUISectionServer            = "Server"
	ClientUISectionAuth              = "Auth"
	ClientUISectionAgent             = "Agent"
	ClientUISectionQueue             = "Local queue"
	ClientUISectionDiagnostics       = "Diagnostics"
	ClientUIStatusNotInitialized     = "not initialized"
	ClientUIStatusRunInitHint        = "run `aris init` to configure"
	ClientUIStatusNotConfigured      = "not configured"
	ClientUIStatusKeyValid           = "API key valid"
	ClientUIStatusKeyInvalid         = "API key invalid"
	ClientUIStatusHooksFormat        = "hooks %d/%d registered"
	ClientUIStatusHooksMissingSuffix = ", missing: "
	ClientUIStatusQueueClear         = "no pending records"
	ClientUIStatusQueueFormat        = "%d pending (%s) · %d rejected"
	ClientUIStatusNoErrors           = "no recent errors"
	ClientUIStatusRecentErrorsFormat = "%d recent error(s)"
	ClientUIStatusLogsHintFormat     = "see %s"
)
