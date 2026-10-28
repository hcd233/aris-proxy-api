package constant

// Agent harness 识别相关常量。
const (
	// AgentHarnessOther 未命中任何 harness 规则时的展示名；保留名，不能作为规则名。
	AgentHarnessOther = "Other"

	// AgentHarnessMaxPatternLength 规则 UA 前缀的最大字符数。
	AgentHarnessMaxPatternLength = 200

	// AgentHarnessMaxNameLength 规则展示名的最大字符数。
	AgentHarnessMaxNameLength = 64

	// AgentHarnessRuleListOrder 规则列表排序：越具体的规则越靠前，与命中优先级同序，
	// 让后台表格自上而下读起来就是「谁先接管」。
	AgentHarnessRuleListOrder = "length(pattern) DESC, pattern"

	// AgentHarnessRuleFingerprintSelect 规则指纹的聚合列（条数 + 最近更新时间）。
	AgentHarnessRuleFingerprintSelect = "count(*) AS total, max(updated_at) AS updated_at"

	// AgentHarnessRuleFingerprintFormat 规则指纹的输出格式：<条数>:<最近更新时间 UnixNano>。
	AgentHarnessRuleFingerprintFormat = "%d:%d"

	// SessionFilterFieldHarness 会话筛选的 harness 维度（Filter DSL 字段名）。
	SessionFilterFieldHarness = "harness"

	// AuditHarnessGroupSQL 审计侧 harness 分组表达式：「UA → 最长命中规则名」，未命中归 Other。
	AuditHarnessGroupSQL = "COALESCE((SELECT r.name FROM agent_harness_rules r WHERE lower(user_agent) LIKE r.pattern || '%' ORDER BY length(r.pattern) DESC LIMIT 1), 'Other')"

	// SessionHarnessResolveSQL 会话侧「UA → 最长命中规则名」的标量子查询。
	//
	// 与 Go 侧 MatchRule 同语义（ORDER BY length(pattern) DESC LIMIT 1）；未命中时返回 NULL，
	// 因此 `IS NULL` 等价于 Other，且空 UA 天然落入。
	SessionHarnessResolveSQL = "(SELECT r.name FROM agent_harness_rules r WHERE lower(client_ua) LIKE r.pattern || '%' ORDER BY length(r.pattern) DESC LIMIT 1)"
)

// AgentHarnessRuleSeedRule 内置种子规则的形状。
type AgentHarnessRuleSeedRule struct {
	Pattern string
	Name    string
}

// AgentHarnessRuleSeeds 首次启动写入的内置 harness 规则。
//
// 取值为生产审计表 2026-09-27 ~ 2026-10-27 实际 UA 的前缀分布（覆盖 99.8% 调用）。
// 只在规则表为空时插入；之后完全由 admin 后台维护。
var AgentHarnessRuleSeeds = []AgentHarnessRuleSeedRule{
	{Pattern: "deepseek-harness/", Name: "DSH"},
	{Pattern: "zcode/", Name: "ZCode"},
	{Pattern: "pi (", Name: "Pi"},
	{Pattern: "codex_cli_rs/", Name: "Codex"},
	{Pattern: "codex desktop/", Name: "Codex"},
	{Pattern: "claude-cli/", Name: "Claude Code"},
	{Pattern: "opencode/", Name: "OpenCode"},
}
