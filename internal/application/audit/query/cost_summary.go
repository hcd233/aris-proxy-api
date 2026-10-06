package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// CostSummaryRepository 成本合计/趋势查询的仓储依赖
type CostSummaryRepository interface {
	SumCostByCurrency(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time) ([]*modelcall.CostTotal, error)
	QueryCostSeries(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.CostPoint, error)
}

// CostSummaryQuery 成本合计查询（admin/demo 全量视角）
type CostSummaryQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

// CostSummaryByUserQuery 成本合计查询（user 视角，限定名下 key）
type CostSummaryByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

// CostSummaryHandler 成本合计处理器（admin/demo 全量视角）
type CostSummaryHandler interface {
	Handle(ctx context.Context, q CostSummaryQuery) (*modelcall.CostSummaryResult, error)
}

// CostSummaryByUserHandler 成本合计处理器（user 视角）
type CostSummaryByUserHandler interface {
	Handle(ctx context.Context, q CostSummaryByUserQuery) (*modelcall.CostSummaryResult, error)
}

type costSummaryHandler struct {
	repo CostSummaryRepository
}

type costSummaryByUserHandler struct {
	repo      CostSummaryRepository
	apiKeyIDs port.APIKeyIDLookup
}

// NewCostSummaryHandler 构造成本合计处理器（admin/demo 全量视角）
func NewCostSummaryHandler(repo CostSummaryRepository) CostSummaryHandler {
	return &costSummaryHandler{repo: repo}
}

// NewCostSummaryByUserHandler 构造成本合计处理器（user 视角）
func NewCostSummaryByUserHandler(repo CostSummaryRepository, apiKeyIDs port.APIKeyIDLookup) CostSummaryByUserHandler {
	return &costSummaryByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *costSummaryHandler) Handle(ctx context.Context, q CostSummaryQuery) (*modelcall.CostSummaryResult, error) {
	return fetchCostSummary(ctx, h.repo, nil, q.StartTime, q.EndTime, q.Granularity)
}

func (h *costSummaryByUserHandler) Handle(ctx context.Context, q CostSummaryByUserQuery) (*modelcall.CostSummaryResult, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	// 名下无 API Key：直接返回空，防止空列表在仓储层退化为全量查询（越权）
	if len(keyIDs) == 0 {
		return &modelcall.CostSummaryResult{Totals: []*modelcall.CostTotal{}, Series: []*modelcall.CostPoint{}}, nil
	}
	return fetchCostSummary(ctx, h.repo, keyIDs, q.StartTime, q.EndTime, q.Granularity)
}

// fetchCostSummary 查询费用合计与趋势（apiKeyIDs 为 nil 表示不过滤）
func fetchCostSummary(ctx context.Context, repo CostSummaryRepository, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) (*modelcall.CostSummaryResult, error) {
	totals, err := repo.SumCostByCurrency(ctx, apiKeyIDs, startTime, endTime)
	if err != nil {
		return nil, err
	}
	series, err := repo.QueryCostSeries(ctx, apiKeyIDs, startTime, endTime, granularity)
	if err != nil {
		return nil, err
	}
	return &modelcall.CostSummaryResult{Totals: totals, Series: series}, nil
}
