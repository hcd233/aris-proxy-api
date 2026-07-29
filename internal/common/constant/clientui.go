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
	ClientUIIconInfo    = "·"
	ClientUIIconSection = "◆"
)

// aris 客户端 TUI 布局格式
const (
	ClientUIKeyPaddingFormat = "%-*s"
)
