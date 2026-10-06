package modelcall

// ModelCostPoint 模型成本排行行（微单位；行 = 模型 × 币种）。
// 仅统计带四维费用拆分的调用（cost_micro 拆分列为 NULL 的存量行不计入）。
type ModelCostPoint struct {
	ModelID                string
	Currency               string
	InputCostMicro         int64
	OutputCostMicro        int64
	CacheCreationCostMicro int64
	CacheReadCostMicro     int64
}

// TotalCostMicro 四维合计（微单位）
func (p *ModelCostPoint) TotalCostMicro() int64 {
	return p.InputCostMicro + p.OutputCostMicro + p.CacheCreationCostMicro + p.CacheReadCostMicro
}
