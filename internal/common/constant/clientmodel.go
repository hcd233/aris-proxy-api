// Package constant 客户端模型导出常量
package constant

import "github.com/hcd233/aris-proxy-api/internal/common/enum"

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
	// ClientModelTargetDSH DSH（DeepSeek Harness）目标标识
	ClientModelTargetDSH = "dsh"

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
	ClientModelKeyModels = "models"
	ClientModelKeyName   = "name"
	ClientModelKeyAPIKey = "apiKey"
	ClientModelKeyAPI    = "api"
	ClientModelAPIOpenAI = "openai-completions"
	ClientModelKeyID     = "id"
	// ClientModelKeyProviders Pi models.json 顶层 provider 包裹键
	ClientModelKeyProviders = "providers"
	// ClientModelKeyBaseUrl Pi provider 的 baseUrl 字段（Pi 用小写 url，与 OpenCode 的 baseURL 不同）
	ClientModelKeyBaseUrl = "baseUrl"
	// ClientModelKeyProvider OpenCode opencode.json 顶层 provider 包裹键
	ClientModelKeyProvider = "provider"
	// ClientModelKeyNPM OpenCode provider 的 npm 包字段
	ClientModelKeyNPM = "npm"
	// ClientModelKeyOptions OpenCode provider 的 options 块
	ClientModelKeyOptions = "options"
	// ClientModelKeyBaseURL OpenCode provider 的 baseURL 字段（大小写与 Pi 的 baseUrl 不同）
	ClientModelKeyBaseURL = "baseURL"
	// ClientModelKeyHeaders OpenCode provider 的 headers 块
	ClientModelKeyHeaders = "headers"
	// ClientModelKeyContextWindow DSH 模型上下文窗口
	ClientModelKeyContextWindow = "contextWindow"
	// ClientModelKeyMaxTokens DSH 模型单次输出上限
	ClientModelKeyMaxTokens = "maxTokens"
	// ClientModelKeyInput DSH 模型输入模态
	ClientModelKeyInput = "input"
	// ClientModelKeyDisplayName DSH provider 展示名
	ClientModelKeyDisplayName = "displayName"
	// ClientModelKeyAPIKeyEnv DSH provider 的凭证引用名
	ClientModelKeyAPIKeyEnv = "apiKeyEnv"
	// ClientModelKeyConfig DSH patch 条目的 config 块
	ClientModelKeyConfig = "config"
	// ClientModelKeyRefs DSH 凭证文件的名字引用块
	ClientModelKeyRefs = "refs"
)

// ClientModelPaths 各 harness 默认配置路径片段
const (
	ClientModelOpenCodePath = ".config/opencode/opencode.json"
	ClientModelPiPath       = ".pi/agent/models.json"
	// ClientModelCodexPath codex config 路径
	ClientModelCodexPath = ".codex/config.toml"
	// ClientModelClaudeCodePath claude code settings 路径
	ClientModelClaudeCodePath = ".claude/settings.json"
	// ClientModelDSHProfilesPath DSH profile 根目录（相对 home）
	ClientModelDSHProfilesPath = ".dsh/profiles"
	// ClientModelDSHPatchFile DSH profile 的补丁层文件名
	ClientModelDSHPatchFile = "cordis.patch.yml"
	// ClientModelDSHCredentialsFile DSH 凭证文件名（相对 DSH home）
	ClientModelDSHCredentialsFile = ".credentials.yaml"
	// ClientModelDSHDefaultProfile 无显式 profile 时优先使用的 profile 名
	ClientModelDSHDefaultProfile = "desktop"
	// ClientModelDSHEntryID DSH llm provider 条目的 patch id
	ClientModelDSHEntryID = "llm-pi-ai"
	// ClientModelDSHEntryName 该条目对应的 DSH 插件包名
	ClientModelDSHEntryName = "@deepseek-ai/dsh-llm-pi-ai"
	// ClientModelDSHProviderID 本工具写入 DSH providers 的 provider 标识
	ClientModelDSHProviderID = "aris"
	// ClientModelDSHProviderDisplayName provider 缺省展示名
	ClientModelDSHProviderDisplayName = "Aris"
	// ClientModelDSHCredentialRef 凭证引用名：同时写进 apiKeyEnv 与 credentials.refs
	ClientModelDSHCredentialRef = "ARIS_API_KEY"
	// ClientModelDSHPatchEntryPrefix patch 条目的 id 行前缀
	ClientModelDSHPatchEntryPrefix = "- id: "
	// ClientModelYAMLListItemPrefix YAML 顶层列表项前缀
	ClientModelYAMLListItemPrefix = "- "
	// ClientModelYAMLIndent YAML 两空格缩进
	ClientModelYAMLIndent = "  "
	// ClientModelYAMLKeySep YAML 键值分隔符
	ClientModelYAMLKeySep = ": "
	// ClientModelDSHCredentialsHeader 凭证文件缺失时的最小骨架
	ClientModelDSHCredentialsHeader = "version: 1\nrecords: {}\n"
	// ClientModelDSHNameWordSep 模型展示名的分词分隔符（alias 按 `-` 分词后逐词首字母大写）
	ClientModelDSHNameWordSep = "-"
)

// ClientModelDisplayNameBrands alias 分词 → 品牌惯例写法：只收录首字母大写无法还原的品牌
// （如 deepseek → DeepSeek、mimo → MiMo、gpt → GPT、glm → GLM），其余分词按首字母大写处理
var ClientModelDisplayNameBrands = map[string]string{
	"deepseek": "DeepSeek",
	"glm":      "GLM",
	"gpt":      "GPT",
	"mimo":     "MiMo",
}

// ClientModelDisplayNames harness 展示名
const (
	ClientModelLabelOpenCode   = "OpenCode"
	ClientModelLabelPi         = "Pi"
	ClientModelLabelCodex      = "Codex"
	ClientModelLabelClaudeCode = "Claude Code"
	// ClientModelLabelDSH DSH 展示名
	ClientModelLabelDSH = "DSH"
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

// ClientModelPiInputModalities Pi models.json 的 input 仅接受 text/image，其余模态导出时丢弃
var ClientModelPiInputModalities = []string{enum.InputModalityText, enum.InputModalityImage}

// ClientModelAttachmentModalities 任一命中即开启 OpenCode attachment（附件上传入口）
var ClientModelAttachmentModalities = []string{enum.InputModalityImage, enum.InputModalityPDF}

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
	ClientModelExportNonInteractiveMessage = "aris model export requires an interactive terminal"
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
	ClientModelExportFetchDoneFormat       = "✓ %d model(s) available"
	ClientModelExportSelectedTitle         = "Selected models"
	ClientModelExportSelectedFormat        = "  • %s"
	ClientModelExportTargetResultFormat    = "✓ Target: %s"
	ClientModelExportDSHProfileMissing     = "DSH profile not found under ~/.dsh/profiles; run DSH once, then retry"
)

// ClientInitMessages aris init 瘦身后的新文案
const (
	ArisClientInitTitleSave        = "Save configuration"
	ArisClientInitSavingConfig     = "Saving configuration..."
	ArisClientNextHintTraceInstall = "Next: aris trace install  # register hooks in Codex/Claude Code"
	ArisClientNextHintModelExport  = "Next: aris model export    # configure models for your agents"
)

// TraceInstallMessages aris trace install 文案
const (
	ArisClientInstallNeedInitMessage       = "Not initialized. Run `aris init` first."
	ArisClientInstallNonInteractiveMessage = "aris trace install requires an interactive terminal"
	ArisClientInstallDone                  = "✓ Hooks installation complete"
)

// ClientStatusMessages status 面板 Providers 节文案
const (
	ClientUISectionProviders             = "Providers"
	ClientUIStatusProvidersLabel         = "agent configs"
	ClientUIStatusProvidersMissingPrefix = " · missing: "
	ClientUIStatusProvidersNoneHint      = "none configured — run aris model export"
)
