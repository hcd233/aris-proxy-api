package model

// AgentHarnessRule agent harness 识别规则：UA 前缀 → harness 展示名。
//
// Pattern 存规范化后的小写前缀，匹配语义为 `lower(ua)` 前缀命中，
// 多条规则同时命中时取最长前缀，因此规则之间没有先后关系；
// 不做软删除（唯一索引 + 软删除会挡住重建同名规则）。
//
// 历史上有一列 sort 记录手工优先级，已废弃：模型不再读写它，
// 库里的旧列保留不动——回滚到旧版本时旧代码仍需要它。
type AgentHarnessRule struct {
	BaseModel
	Pattern string `json:"pattern" gorm:"column:pattern;not null;uniqueIndex:idx_harness_rules_pattern;comment:UA前缀(小写)"`
	Name    string `json:"name" gorm:"column:name;not null;index:idx_harness_rules_name;comment:harness展示名"`
}
