// Package port defines application-layer ports for audit use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// AuditLogView 审计日志只读投影
type AuditLogView struct {
	ID                       uint
	CreatedAt                time.Time
	ModelID                  string
	UpstreamProtocol         string
	APIProtocol              string
	Endpoint                 string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	FirstTokenLatencyMs      int64
	StreamDurationMs         int64
	UserAgent                string
	UpstreamStatusCode       int
	ErrorMessage             string
	TraceID                  string
	APIKeyName               string
	UserName                 string
	UserEmail                string
	CostMicro                *int64
	PricingCurrency          enum.Currency
}

// ListAuditLogsParams 列表查询的通用参数（不带权限相关字段）
type ListAuditLogsParams struct {
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
	Filter    string
}

// AuditService 统一按当前 JWT 权限派发审计查询，handler 只负责 DTO 映射
type AuditService interface {
	ListLogs(ctx context.Context, permission enum.Permission, userID uint, q ListAuditLogsParams) ([]*AuditLogView, *model.PageInfo, error)
	ListAuditOption(ctx context.Context, permission enum.Permission, userID uint, field, keyword string, startTime, endTime time.Time) ([]string, error)
	ModelTrend(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.ModelTrendPoint, error)
	RequestRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.RequestRatePoint, error)
	TokenThroughput(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error)
	TokenRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenRateItem, error)
	ModelUsage(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.ModelUsageItem, error)
	FirstTokenLatency(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.FirstTokenLatencyItem, error)
	CostSummary(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) (*CostSummaryView, error)
	CostDistribution(ctx context.Context, permission enum.Permission, userID uint, groupBy string, startTime, endTime time.Time, limit int) (*CostDistributionView, error)
}

// CostSummaryView 费用合计与趋势视图（展示单位，按币种分组）
type CostSummaryView struct {
	Totals []*dto.AuditCostTotalItem
	Series []*dto.AuditCostSeriesPoint
}

// CostDistributionView 费用分布视图（展示单位，每行 = 分组 × 币种）
type CostDistributionView struct {
	GroupBy enum.CostGroupBy
	Items   []*dto.AuditCostDistributionItem
}
