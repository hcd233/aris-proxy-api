package model

// AgentHarnessRule agent harness 识别规则：UA 前缀 → harness 展示名。
//
// Pattern 存规范化后的小写前缀，匹配语义为 `lower(ua)` 前缀命中，
// 按 (sort, id) 升序取第一条；不做软删除（唯一索引 + 软删除会挡住重建同名规则）。
type AgentHarnessRule struct {
	BaseModel
	Pattern string `json:"pattern" gorm:"column:pattern;not null;uniqueIndex:idx_harness_rules_pattern;comment:UA前缀(小写)"`
	Name    string `json:"name" gorm:"column:name;not null;index:idx_harness_rules_name;comment:harness展示名"`
	Sort    int    `json:"sort" gorm:"column:sort;not null;default:0;comment:排序(升序优先)"`
}
