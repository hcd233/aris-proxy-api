package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// DashboardScope dashboard 查询的调用者视角
type DashboardScope struct {
	Permission enum.Permission
	UserID     uint
}

// DashboardPeriod 一个统计周期：总量 + 已补齐空桶的时间序列
type DashboardPeriod struct {
	Summary *modelcall.DashboardStats
	Series  []*modelcall.DashboardStatsPoint
}

// DashboardOverview 本期与上一等长周期
type DashboardOverview struct {
	Current  DashboardPeriod
	Previous DashboardPeriod
}

// DashboardTopItem Top 排行的一行（Name 已按视角脱敏）
type DashboardTopItem struct {
	Key             string
	Name            string
	Requests        int64
	Tokens          int64
	CacheReadTokens int64
	PromptTokens    int64
	CostMicro       int64
}

// DashboardTop Top 排行结果：前 N 名 + 其余请求合计 + 全部费用合计（算占比用）
type DashboardTop struct {
	Items          []*DashboardTopItem
	OtherRequests  int64
	TotalCostMicro int64
}

// DashboardRelationLookup 按 API Key 批量查归属（Key 名、用户）
type DashboardRelationLookup interface {
	BatchGetRelations(ctx context.Context, apiKeyIDs []uint) (map[uint]*modelcall.AuditRelation, error)
}

// HarnessFingerprinter harness 规则指纹（参与读缓存键，规则变更后分组立即刷新）
type HarnessFingerprinter interface {
	Fingerprint(ctx context.Context) (string, error)
}

// DashboardService dashboard 聚合查询：统一权限分流、上一周期、补桶、Top 聚合与 demo 脱敏
type DashboardService interface {
	Overview(ctx context.Context, s DashboardScope, start, end time.Time, g enum.Granularity) (*DashboardOverview, error)
	ModelSeries(ctx context.Context, s DashboardScope, start, end time.Time, g enum.Granularity) ([]*modelcall.DashboardModelPoint, error)
	TopErrors(ctx context.Context, s DashboardScope, start, end time.Time, limit int) ([]*modelcall.DashboardErrorRow, error)
	EndpointHealth(ctx context.Context, s DashboardScope, start, end time.Time, limit int) ([]*modelcall.DashboardEndpointRow, error)
	TopUsage(ctx context.Context, s DashboardScope, start, end time.Time, dim enum.DashboardDimension, limit int) (*DashboardTop, error)
	Heatmap(ctx context.Context, s DashboardScope, start, end time.Time, location string) ([]*modelcall.DashboardHeatmapCell, error)
}
