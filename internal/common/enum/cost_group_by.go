package enum

// CostGroupBy 成本分布分组维度
type CostGroupBy string

const (
	CostGroupByUser   CostGroupBy = "user"
	CostGroupByAPIKey CostGroupBy = "api_key"
	CostGroupByModel  CostGroupBy = "model"
)
