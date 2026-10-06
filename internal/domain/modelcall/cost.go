package modelcall

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// CostTotal 按币种费用合计（微单位）
type CostTotal struct {
	Currency  string
	CostMicro int64
}

// CostPoint 费用趋势点（微单位；时间桶 × 币种）
type CostPoint struct {
	Time      time.Time
	Currency  string
	CostMicro int64
}

// CostDistributionPoint 成本分布行（每行 = 分组 × 币种）
type CostDistributionPoint struct {
	ID        string
	Name      string
	Currency  string
	CostMicro int64
}

// CostSummaryResult 成本合计结果（微单位）
type CostSummaryResult struct {
	Totals []*CostTotal
	Series []*CostPoint
}

// CostDistributionResult 成本分布结果（微单位）
type CostDistributionResult struct {
	GroupBy enum.CostGroupBy
	Items   []*CostDistributionPoint
}
