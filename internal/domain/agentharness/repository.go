package agentharness

import "context"

// RuleRepository agent harness 规则的仓储端口。
//
// 规则量级是个位数到几十行，读取不做缓存：每次直查小表，
// 换来多副本部署下「改规则立即全量生效」的一致性。
type RuleRepository interface {
	// List 返回全部规则，按 (sort, id) 升序。
	List(ctx context.Context) ([]Rule, error)
	// Get 按 ID 查询；未找到返回 (nil, nil)。
	Get(ctx context.Context, id uint) (*Rule, error)
	// Create 规范化并落库一条规则，返回带 ID 的规则。
	Create(ctx context.Context, rule Rule) (*Rule, error)
	// Update 规范化并覆盖一条规则。
	Update(ctx context.Context, rule Rule) error
	// Delete 硬删除一条规则。
	Delete(ctx context.Context, id uint) error
	// Fingerprint 返回规则集指纹（条数 + 最近更新时间），供读缓存键使用。
	Fingerprint(ctx context.Context) (string, error)
}
