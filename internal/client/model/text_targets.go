package model

import (
	"os"
	"regexp"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// ─── Claude Code ───

// claudeSettings Claude Code settings.json（env 之外的键经 Raw 保留）
type claudeSettings struct {
	Raw map[string]any `json:"-"`
}

func (c *claudeSettings) UnmarshalJSON(data []byte) error { return sonicUnmarshal(data, &c.Raw) }
func (c *claudeSettings) MarshalJSON() ([]byte, error)    { return sonicMarshal(c.Raw) }

// ClaudeCodeTarget 写入 ~/.claude/settings.json 的 env 块
type ClaudeCodeTarget struct{}

func (ClaudeCodeTarget) Key() string   { return constant.ClientModelTargetClaudeCode }
func (ClaudeCodeTarget) Label() string { return constant.ClientModelLabelClaudeCode }
func (ClaudeCodeTarget) ConfigPath(home string) string {
	return filepathJoin(home, constant.ClientModelClaudeCodePath)
}

// Write merge ANTHROPIC_* env 到 settings.json；context ≥ 1M 的模型 alias 加 [1m] 后缀
func (ClaudeCodeTarget) Write(path, host, apiKey string, models []TargetModel) error {
	var cfg claudeSettings
	if err := readJSONFile(path, &cfg); err != nil {
		return err
	}
	if cfg.Raw == nil {
		cfg.Raw = map[string]any{}
	}
	env, _ := cfg.Raw[constant.ClientModelKeyEnv].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	env[constant.ClaudeEnvBaseURL] = host
	env[constant.ClaudeEnvAuthToken] = apiKey
	for tier, key := range constant.ClaudeTierEnvKeys {
		m := findBestModelForTier(models, tier)
		if m == nil {
			continue
		}
		alias := m.Alias
		if m.ContextLength >= constant.ClientModelOneMContext {
			alias += constant.ClientModelClaudeOneMSuffix
		}
		env[key] = alias
	}
	cfg.Raw[constant.ClientModelKeyEnv] = env

	data, err := sonicMarshal(&cfg)
	if err != nil {
		return err
	}
	return backupAndWrite(path, data)
}

// findBestModelForTier 按 tier 语义挑选模型：opus 取 context 最大，sonnet 次之，haiku 最小。
// 简化策略：按 contextLength 排序后取第 N 个（N 为 tier 序号），不足时返回 nil。
func findBestModelForTier(models []TargetModel, tier string) *TargetModel {
	sorted := make([]TargetModel, len(models))
	copy(sorted, models)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].ContextLength > sorted[j-1].ContextLength; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	index := -1
	for i, t := range constant.ClaudeTierOrder {
		if t == tier {
			index = i
			break
		}
	}
	if index < 0 || index >= len(sorted) {
		return nil
	}
	return &sorted[index]
}

// ─── Codex ───

// CodexTarget 写入 ~/.codex/config.toml（TOML 文本处理）
type CodexTarget struct{}

func (CodexTarget) Key() string   { return constant.ClientModelTargetCodex }
func (CodexTarget) Label() string { return constant.ClientModelLabelCodex }
func (CodexTarget) ConfigPath(home string) string {
	return filepathJoin(home, constant.ClientModelCodexPath)
}

// tomlTableHeader 匹配任意 TOML 表头（含数组表与注释）
var tomlTableHeader = regexp.MustCompile(`^\s*\[{1,2}\s*[A-Za-z0-9_."'-]+(?:\s*\.\s*[A-Za-z0-9_."'-]+)*\s*\]{1,2}\s*(?:#.*)?$`)

// tomlProviderHeader 匹配本工具 provider 表头（裸键或引号键）
var tomlProviderHeader = regexp.MustCompile(`^\s*\[\s*model_providers\s*\.\s*(?:"?` + regexp.QuoteMeta(constant.ClientModelProviderID) + `"?)\s*\]\s*$`)

// tomlMemoriesHeader 匹配 [memories] 表头
var tomlMemoriesHeader = regexp.MustCompile(`^\s*\[\s*"?memories"?\s*\]\s*(?:#.*)?$`)

// tomlRootModelKeys 匹配 root 层 model/model_provider/model_context_window 行
var tomlRootModelKeys = regexp.MustCompile(`^\s*(model|model_provider|model_context_window)\s*=`)

// tomlMemoryModelKey 匹配 memories 表内 extract_model/consolidation_model 行
var tomlMemoryModelKey = regexp.MustCompile(`^\s*(?:extract_model|consolidation_model)\s*=`)

// tomlQuote 字符串值加双引号并转义内部引号
func tomlQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// Write 清理旧 provider/root 配置后写入新的 root 与 provider TOML 块
func (CodexTarget) Write(path, host, apiKey string, models []TargetModel) error {
	original, err := os.ReadFile(path) //nolint:gosec // path is a fixed config path from caller
	if err != nil && !os.IsNotExist(err) {
		return ierrWrapRead(path, err)
	}

	model := ""
	contextWindow := constant.ClientModelDefaultContext
	if len(models) > 0 {
		model = models[0].Alias
		if models[0].ContextLength > 0 {
			contextWindow = models[0].ContextLength
		}
	}

	lines := splitLines(string(original))
	memoryModel := codexMemoryModel(lines)
	cleaned := cleanCodexConfig(lines)

	rootBlock := []string{
		"model = " + tomlQuote(model),
		"model_provider = " + tomlQuote(constant.ClientModelProviderID),
		"model_context_window = " + itoa(contextWindow),
	}
	providerBlock := []string{
		`[model_providers.` + tomlQuote(constant.ClientModelProviderID) + `]`,
		`name = ` + tomlQuote(constant.ClientModelLabelArisProxy),
		`base_url = ` + tomlQuote(host),
		`wire_api = "responses"`,
		`experimental_bearer_token = ` + tomlQuote(apiKey),
	}
	memoryBlock := []string{
		"extract_model = " + tomlQuote(memoryModel),
		"consolidation_model = " + tomlQuote(memoryModel),
	}

	root := trimTrailingBlank(cleaned.root)
	memories := trimTrailingBlank(cleaned.memories)

	var out []string
	out = append(out, root...)
	if len(root) > 0 {
		out = append(out, "")
	}
	out = append(out, rootBlock...)
	if len(memories) > 0 {
		out = append(out, "", "[memories]")
		out = append(out, memories...)
		out = append(out, memoryBlock...)
	}
	out = append(out, "")
	out = append(out, providerBlock...)

	return backupAndWrite(path, []byte(strings.Join(out, "\n")))
}

// codexCleanResult 清理后的分区内容
type codexCleanResult struct {
	root     []string // root 层行（已剔除旧 model* 键）
	memories []string // [memories] 段（已剔除 extract/consolidation_model，保留其余行）
}

// cleanCodexConfig 移除旧同名 provider 段、root 层旧 model 键；重建 [memories] 模型键。
// 其余表头之后的内容原样保留。
func cleanCodexConfig(lines []string) codexCleanResult {
	result := codexCleanResult{root: []string{}, memories: []string{}}

	inProvider := false
	inMemories := false
	inOtherTable := false
	for _, line := range lines {
		if inProvider {
			if tomlTableHeader.MatchString(line) {
				inProvider = false
			} else {
				continue
			}
		}
		if tomlProviderHeader.MatchString(line) {
			inProvider = true
			continue
		}
		switch {
		case tomlMemoriesHeader.MatchString(line):
			inMemories = true
			inOtherTable = false
			result.memories = append(result.memories, line)
			continue
		case tomlTableHeader.MatchString(line):
			inMemories = false
			inOtherTable = true
		}
		if inMemories && tomlMemoryModelKey.MatchString(line) {
			continue
		}
		if !inMemories && !inOtherTable && tomlRootModelKeys.MatchString(line) {
			continue
		}
		if inMemories {
			result.memories = append(result.memories, line)
		} else {
			result.root = append(result.root, line)
		}
	}
	return result
}

// codexMemoryModel 从原配置提取首个 root model 值作为 memories 模型缺省
// tomlRootModelValue 匹配 root 层 model = 行（仅精确键 model）
var tomlRootModelValue = regexp.MustCompile(`^\s*model\s*=`)

func codexMemoryModel(lines []string) string {
	for _, line := range lines {
		if tomlRootModelValue.MatchString(line) {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				v := strings.Trim(strings.TrimSpace(parts[1]), `"`)
				if v != "" {
					return v
				}
			}
		}
	}
	return constant.ClientModelDefaultMemoryModel
}

// trimTrailingBlank 移除尾部空行
func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
