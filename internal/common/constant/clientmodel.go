// Package constant 客户端模型导出常量
package constant

// ClientModelExport 模型导出目标与默认值
const (
	// ClientModelProviderID 本工具写入各 harness 的 provider 标识
	ClientModelProviderID = "aris-proxy"

	// ClientModelTargetOpenCode OpenCode 目标标识
	ClientModelTargetOpenCode = "opencode"
	// ClientModelTargetPi Pi 目标标识
	ClientModelTargetPi = "pi"
	// ClientModelTargetCodex Codex 目标标识
	ClientModelTargetCodex = "codex"
	// ClientModelTargetClaudeCode Claude Code 目标标识
	ClientModelTargetClaudeCode = "claude-code"

	// ClientModelDefaultContext context 缺省值
	ClientModelDefaultContext = 128000
	// ClientModelOpenCodeDefaultOutput OpenCode output 缺省值
	ClientModelOpenCodeDefaultOutput = 64000
	// ClientModelPiDefaultMaxTokens Pi maxTokens 缺省值
	ClientModelPiDefaultMaxTokens = 16384

	// ClientModelOpenCodeNPM OpenCode provider npm 包名
	ClientModelOpenCodeNPM = "@ai-sdk/openai-compatible"
	// ClientModelOneMContext 1M 上下文阈值（达到则 Claude Code alias 加 [1m] 后缀）
	ClientModelOneMContext = 1_000_000
	// ClientModelClaudeOneMSuffix Claude Code 1M 上下文模型名后缀
	ClientModelClaudeOneMSuffix = "[1m]"
)

// ClientModelJSONKeys harness 配置 JSON 键名
const (
	ClientModelKeyModels  = "models"
	ClientModelKeyName    = "name"
	ClientModelKeyBaseURL = "baseURL"
	ClientModelKeyAPIKey  = "apiKey"
	ClientModelKeyAPI     = "api"
	ClientModelAPIOpenAI  = "openai-completions"
	ClientModelKeyID      = "id"
)

// ClientModelPaths 各 harness 默认配置路径片段
const (
	ClientModelOpenCodePath = ".config/opencode/opencode.json"
	ClientModelPiPath       = ".pi/agent/models.json"
	// ClientModelCodexPath codex config 路径
	ClientModelCodexPath = ".codex/config.toml"
	// ClientModelClaudeCodePath claude code settings 路径
	ClientModelClaudeCodePath = ".claude/settings.json"
)

// ClientModelDisplayNames harness 展示名
const (
	ClientModelLabelOpenCode   = "OpenCode"
	ClientModelLabelPi         = "Pi"
	ClientModelLabelCodex      = "Codex"
	ClientModelLabelClaudeCode = "Claude Code"
)

// ClientModelAuthPrefix Bearer 认证头前缀与相对路径标记
const (
	ClientModelAuthBearer   = "Bearer "
	ClientModelParentRel    = ".."
	ClientModelParentRelSep = "../"
)

// ClientModelCodexExtra Codex/Claude Code 配置细节常量
const (
	// ClientModelLabelArisProxy Codex provider 展示名
	ClientModelLabelArisProxy = "Aris Proxy"
	// ClientModelDefaultMemoryModel memories 表模型缺省值
	ClientModelDefaultMemoryModel = ""
	// ClientModelKeyEnv Claude Code settings.json 的 env 键
	ClientModelKeyEnv = "env"
)

// ClaudeCodeEnvKeys Claude Code tier → 环境变量名
const (
	ClaudeEnvBaseURL    = "ANTHROPIC_BASE_URL"
	ClaudeEnvAuthToken  = "ANTHROPIC_AUTH_TOKEN"
	ClaudeTierOpusKey   = "ANTHROPIC_DEFAULT_OPUS_MODEL"
	ClaudeTierSonnetKey = "ANTHROPIC_DEFAULT_SONNET_MODEL"
	ClaudeTierHaikuKey  = "ANTHROPIC_DEFAULT_HAIKU_MODEL"
)

// ClaudeTierOrder / ClaudeTierEnvKeys tier 顺序与对应环境变量
var ClaudeTierOrder = []string{"opus", "sonnet", "haiku"}

var ClaudeTierEnvKeys = map[string]string{
	"opus":   ClaudeTierOpusKey,
	"sonnet": ClaudeTierSonnetKey,
	"haiku":  ClaudeTierHaikuKey,
}

// ClientModelMisc 通用杂项常量
const (
	ClientModelCRLF     = "\r\n"
	ClientModelLF       = "\n"
	ClientModelModelKey = "model"
	ClientModelSpaceSep = " "
)

// ClientModelExportMessages aris model export 交互文案
const (
	ClientModelExportNeedInitMessage       = "Not initialized. Run `aris init` first."
	ClientModelExportFetchingMessage       = "Fetching models from server..."
	ClientModelExportEmptyModelsMessage    = "No enabled models found on the server."
	ClientModelExportSelectModelsTitle     = "Select models to export"
	ClientModelExportModelsRequiredMessage = "At least one model must be selected"
	ClientModelExportSelectTargetTitle     = "Select target agent"
	ClientModelExportOptionFormat          = "%s · %s · ctx %d / out %d"
	ClientModelExportWritingFormat         = "Writing %s configuration..."
	ClientModelExportDoneFormat            = "✓ %s configured with %d model(s)"
	ClientModelExportConfigPathPrefix      = "Config: "
	ClientModelExportBackupHint            = "Previous config backed up as *.bak"
)
