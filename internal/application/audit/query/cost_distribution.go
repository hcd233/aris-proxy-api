package query

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// CostDistributionRepository 成本分布查询的仓储依赖
type CostDistributionRepository interface {
	QueryCostDistribution(ctx context.Context, apiKeyIDs []uint, groupBy enum.CostGroupBy, startTime, endTime time.Time) ([]*modelcall.CostDistributionPoint, error)
	BatchGetRelations(ctx context.Context, apiKeyIDs []uint) (map[uint]*modelcall.AuditRelation, error)
}

// CostDistributionQuery 成本分布查询（admin/demo 全量视角）
type CostDistributionQuery struct {
	GroupBy   enum.CostGroupBy
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

// CostDistributionByUserQuery 成本分布查询（user 视角；group_by=user 拒绝）
type CostDistributionByUserQuery struct {
	UserID    uint
	GroupBy   enum.CostGroupBy
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

// CostDistributionHandler 成本分布处理器（admin/demo 全量视角）
type CostDistributionHandler interface {
	Handle(ctx context.Context, q CostDistributionQuery) (*modelcall.CostDistributionResult, error)
}

// CostDistributionByUserHandler 成本分布处理器（user 视角）
type CostDistributionByUserHandler interface {
	Handle(ctx context.Context, q CostDistributionByUserQuery) (*modelcall.CostDistributionResult, error)
}

type costDistributionHandler struct {
	repo CostDistributionRepository
}

type costDistributionByUserHandler struct {
	repo      CostDistributionRepository
	apiKeyIDs port.APIKeyIDLookup
}

// NewCostDistributionHandler 构造成本分布处理器（admin/demo 全量视角）
func NewCostDistributionHandler(repo CostDistributionRepository) CostDistributionHandler {
	return &costDistributionHandler{repo: repo}
}

// NewCostDistributionByUserHandler 构造成本分布处理器（user 视角）
func NewCostDistributionByUserHandler(repo CostDistributionRepository, apiKeyIDs port.APIKeyIDLookup) CostDistributionByUserHandler {
	return &costDistributionByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

func (h *costDistributionHandler) Handle(ctx context.Context, q CostDistributionQuery) (*modelcall.CostDistributionResult, error) {
	points, err := h.repo.QueryCostDistribution(ctx, nil, q.GroupBy, q.StartTime, q.EndTime)
	if err != nil {
		return nil, err
	}
	return enrichCostDistribution(ctx, h.repo, q.GroupBy, points, q.Limit)
}

func (h *costDistributionByUserHandler) Handle(ctx context.Context, q CostDistributionByUserQuery) (*modelcall.CostDistributionResult, error) {
	if q.GroupBy == enum.CostGroupByUser {
		return nil, ierr.New(ierr.ErrNoPermission, "cost distribution group by user is admin only")
	}
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	// 名下无 API Key：直接返回空，防止空列表在仓储层退化为全量查询（越权）
	if len(keyIDs) == 0 {
		return &modelcall.CostDistributionResult{GroupBy: q.GroupBy, Items: []*modelcall.CostDistributionPoint{}}, nil
	}
	points, err := h.repo.QueryCostDistribution(ctx, keyIDs, q.GroupBy, q.StartTime, q.EndTime)
	if err != nil {
		return nil, err
	}
	return enrichCostDistribution(ctx, h.repo, q.GroupBy, points, q.Limit)
}

// enrichCostDistribution 补全展示名并按币种裁剪 top-N：
//   - model 组：Name=ID
//   - api_key 组：Name=Key 名（BatchGetRelations）
//   - user 组：仓储按 api_key 分组，这里经关系表归并到用户（ID=用户ID，Name=用户名）
func enrichCostDistribution(ctx context.Context, repo CostDistributionRepository, groupBy enum.CostGroupBy, points []*modelcall.CostDistributionPoint, limit int) (*modelcall.CostDistributionResult, error) {
	if groupBy == enum.CostGroupByModel {
		items := lo.Map(points, func(p *modelcall.CostDistributionPoint, _ int) *modelcall.CostDistributionPoint {
			return &modelcall.CostDistributionPoint{ID: p.ID, Name: p.ID, Currency: p.Currency, CostMicro: p.CostMicro}
		})
		return &modelcall.CostDistributionResult{GroupBy: groupBy, Items: trimCostPerCurrency(items, limit)}, nil
	}

	keyIDs := make([]uint, 0, len(points))
	for _, p := range points {
		if id, err := strconv.ParseUint(p.ID, 10, constant.PricingParseBitSize); err == nil {
			keyIDs = append(keyIDs, uint(id))
		}
	}
	relations, err := repo.BatchGetRelations(ctx, lo.Uniq(keyIDs))
	if err != nil {
		return nil, err
	}

	if groupBy == enum.CostGroupByAPIKey {
		items := lo.Map(points, func(p *modelcall.CostDistributionPoint, _ int) *modelcall.CostDistributionPoint {
			name := ""
			if id, err := strconv.ParseUint(p.ID, 10, constant.PricingParseBitSize); err == nil {
				if rel, ok := relations[uint(id)]; ok {
					name = rel.APIKeyName
				}
			}
			return &modelcall.CostDistributionPoint{ID: p.ID, Name: name, Currency: p.Currency, CostMicro: p.CostMicro}
		})
		return &modelcall.CostDistributionResult{GroupBy: groupBy, Items: trimCostPerCurrency(items, limit)}, nil
	}

	// groupBy == user：按归属用户归并
	merged := map[string]*modelcall.CostDistributionPoint{}
	for _, p := range points {
		id, err := strconv.ParseUint(p.ID, 10, constant.PricingParseBitSize)
		if err != nil {
			continue
		}
		rel, ok := relations[uint(id)]
		if !ok {
			continue
		}
		uid := strconv.FormatUint(uint64(rel.UserID), 10)
		key := uid + "|" + p.Currency
		if cur, ok := merged[key]; ok {
			cur.CostMicro += p.CostMicro
			continue
		}
		merged[key] = &modelcall.CostDistributionPoint{ID: uid, Name: rel.UserName, Currency: p.Currency, CostMicro: p.CostMicro}
	}
	items := lo.Values(merged)
	slices.SortFunc(items, func(a, b *modelcall.CostDistributionPoint) int {
		if a.Currency != b.Currency {
			return cmp.Compare(a.Currency, b.Currency)
		}
		return cmp.Compare(b.CostMicro, a.CostMicro)
	})
	return &modelcall.CostDistributionResult{GroupBy: groupBy, Items: trimCostPerCurrency(items, limit)}, nil
}

// trimCostPerCurrency 每币种保留费用最高的 limit 条（输入需按币种、费用降序排好）。
// ponytail: top-N 在应用层裁剪而非 SQL 窗口函数，升级路径：ROW_NUMBER() OVER (PARTITION BY currency) 下沉仓储。
func trimCostPerCurrency(items []*modelcall.CostDistributionPoint, limit int) []*modelcall.CostDistributionPoint {
	if limit <= 0 {
		limit = constant.CostDistributionDefaultLimit
	}
	counts := map[string]int{}
	out := make([]*modelcall.CostDistributionPoint, 0, len(items))
	for _, it := range items {
		if counts[it.Currency] >= limit {
			continue
		}
		counts[it.Currency]++
		out = append(out, it)
	}
	return out
}
