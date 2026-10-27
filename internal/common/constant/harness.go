package constant

// Agent harness 识别相关常量。
const (
	// AgentHarnessOther 未命中任何 harness 规则时的展示名；保留名，不能作为规则名。
	AgentHarnessOther = "Other"

	// AgentHarnessMaxPatternLength 规则 UA 前缀的最大字符数。
	AgentHarnessMaxPatternLength = 200

	// AgentHarnessMaxNameLength 规则展示名的最大字符数。
	AgentHarnessMaxNameLength = 64

	// AgentHarnessRuleListOrder 规则列表排序：与 SQL 侧标量子查询的首命中顺序保持一致。
	AgentHarnessRuleListOrder = "sort, id"

	// AgentHarnessRuleFingerprintSelect 规则指纹的聚合列（条数 + 最近更新时间）。
	AgentHarnessRuleFingerprintSelect = "count(*) AS total, max(updated_at) AS updated_at"

	// AgentHarnessRuleFingerprintFormat 规则指纹的输出格式：<条数>:<最近更新时间 UnixNano>。
	AgentHarnessRuleFingerprintFormat = "%d:%d"
)

// AgentHarnessRuleSeedRule 内置种子规则的形状。
type AgentHarnessRuleSeedRule struct {
	Pattern string
	Name    string
	Sort    int
}

// AgentHarnessRuleSeeds 首次启动写入的内置 harness 规则。
//
// 取值为生产审计表 2026-09-27 ~ 2026-10-27 实际 UA 的前缀分布（覆盖 99.8% 调用）。
// 只在规则表为空时插入；之后完全由 admin 后台维护。
var AgentHarnessRuleSeeds = []AgentHarnessRuleSeedRule{
	{Pattern: "deepseek-harness/", Name: "DSH", Sort: 10},
	{Pattern: "zcode/", Name: "ZCode", Sort: 20},
	{Pattern: "pi (", Name: "Pi", Sort: 30},
	{Pattern: "codex_cli_rs/", Name: "Codex", Sort: 40},
	{Pattern: "codex desktop/", Name: "Codex", Sort: 41},
	{Pattern: "claude-cli/", Name: "Claude Code", Sort: 50},
	{Pattern: "opencode/", Name: "OpenCode", Sort: 60},
}
