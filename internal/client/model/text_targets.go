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
	// Claude Code 会在 base URL 后追加 /v1/messages，故用不带 /v1 的 ClaudeCodeBaseURLPrefix
	env[constant.ClaudeEnvBaseURL] = host + constant.ClaudeCodeBaseURLPrefix
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
var tomlProviderHeader = regexp.MustCompile(`^\s*\[\s*model_providers\s*\.\s*(?:"?` + regexp.QuoteMeta(constant.ClientModelProviderID) + `"?)\s*\]\s*(?:#.*)?$`)

// tomlProfilesHeader 匹配 [profiles.*] 表头：Codex profile 合法使用 model / model_provider 同名键
var tomlProfilesHeader = regexp.MustCompile(`^\s*\[\s*profiles\s*\.`)

// tomlMemoriesHeader 匹配 [memories] 表头
var tomlMemoriesHeader = regexp.MustCompile(`^\s*\[\s*"?memories"?\s*\]\s*(?:#.*)?$`)

// tomlRootModelKeys 匹配 root 层 model/model_provider/model_context_window 行
var tomlRootModelKeys = regexp.MustCompile(`^\s*(model|model_provider|model_context_window)\s*=`)

// tomlMemoryModelKey 匹配 memories 表内 extract_model/consolidation_model 行
var tomlMemoryModelKey = regexp.MustCompile(`^\s*(?:extract_model|consolidation_model)\s*=`)

// tomlMultiline 跟踪 TOML 三引号多行字符串状态：字符串内部的行不当作配置行处理
type tomlMultiline struct {
	delim string
}

// track 推进状态并返回该行是否属于多行字符串（含起止行）
func (m *tomlMultiline) track(line string) bool {
	if m.delim != "" {
		if strings.Contains(line, m.delim) {
			m.delim = ""
		}
		return true
	}
	delim := ""
	switch {
	case strings.Contains(line, `"""`):
		delim = `"""`
	case strings.Contains(line, `'''`):
		delim = `'''`
	}
	if delim == "" {
		return false
	}
	// 同行闭合（如 a = """text"""）不算开启
	if strings.Contains(strings.SplitN(line, delim, 2)[1], delim) {
		return false
	}
	m.delim = delim
	return true
}

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
		`base_url = ` + tomlQuote(host+constant.OpenAIProxyPrefix),
		`wire_api = "responses"`,
		`experimental_bearer_token = ` + tomlQuote(apiKey),
	}
	memoryBlock := []string{
		"extract_model = " + tomlQuote(memoryModel),
		"consolidation_model = " + tomlQuote(memoryModel),
	}

	head := trimTrailingBlank(cleaned.head)
	tail := trimTrailingBlank(cleaned.tail)
	memories := trimTrailingBlank(cleaned.memories)

	var out []string
	// root 键必须写在任何表头之前：TOML 里表头之后的裸键属于该表，写在文件末尾会落进别的表
	out = append(out, head...)
	if len(head) > 0 {
		out = append(out, "")
	}
	out = append(out, rootBlock...)
	if len(tail) > 0 {
		out = append(out, "")
		out = append(out, tail...)
	}
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
	head     []string // 首个表头之前的顶层键行（旧 model* 键已剔除）
	tail     []string // 首个表头起的其余内容（非 profiles 表内的 root 脏键已剔除）
	memories []string // [memories] 段内容（表头与 extract/consolidation_model 已剔除）
}

// append 按当前分区归属追加行：memories 段 → 其余表 → 顶层
func (r *codexCleanResult) append(line string, inMemories, seenTable bool) {
	switch {
	case inMemories:
		r.memories = append(r.memories, line)
	case seenTable:
		r.tail = append(r.tail, line)
	default:
		r.head = append(r.head, line)
	}
}

// cleanCodexConfig 移除旧同名 provider 段与旧 model 键；重建 [memories] 模型键。
// root 同名键只在顶层与 [profiles.*] 内合法：非 profiles 表内出现即历史版本把 root 块写进了
// 最后一张表（2026-09-14 之前），必须一并剔除，否则升级后重跑导出仍会留下 TOML duplicate key。
// 三引号多行字符串内部不参与表头与键判定，避免误删用户配置文本。
func cleanCodexConfig(lines []string) codexCleanResult {
	result := codexCleanResult{head: []string{}, tail: []string{}, memories: []string{}}

	var (
		inProvider   bool
		inMemories   bool
		seenTable    bool
		dropRootKeys = true
		literal      tomlMultiline
	)
	for _, line := range lines {
		inLiteral := literal.track(line)

		if inProvider {
			if !tomlTableHeader.MatchString(line) {
				continue
			}
			inProvider = false
		}

		if !inLiteral {
			switch {
			case tomlProviderHeader.MatchString(line):
				inProvider, seenTable = true, true
				continue
			case tomlMemoriesHeader.MatchString(line):
				// 表头统一由 Write 输出一次，避免每次导出都累积一个重复表头
				inMemories, dropRootKeys, seenTable = true, false, true
				continue
			case tomlTableHeader.MatchString(line):
				inMemories = false
				dropRootKeys = !tomlProfilesHeader.MatchString(line)
				seenTable = true
			}
			if inMemories && tomlMemoryModelKey.MatchString(line) {
				continue
			}
			if dropRootKeys && tomlRootModelKeys.MatchString(line) {
				continue
			}
		}
		result.append(line, inMemories, seenTable)
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
