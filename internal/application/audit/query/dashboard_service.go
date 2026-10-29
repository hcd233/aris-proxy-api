package query

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// dashboardService port.DashboardService 实现
type dashboardService struct {
	repo      modelcall.DashboardRepository
	apiKeyIDs port.APIKeyIDLookup
	relations port.DashboardRelationLookup
	rules     port.HarnessFingerprinter
}

// NewDashboardService 构造 dashboard 查询服务
//
//	@param repo modelcall.DashboardRepository
//	@param apiKeyIDs port.APIKeyIDLookup user 视角解析名下 Key
//	@param relations port.DashboardRelationLookup Key → 名称/归属用户
//	@param rules port.HarnessFingerprinter 可为 nil（不带指纹，缓存仍按 TTL 过期）
//	@return port.DashboardService
//	@author centonhuang
//	@update 2026-10-28 10:00:00
func NewDashboardService(
	repo modelcall.DashboardRepository,
	apiKeyIDs port.APIKeyIDLookup,
	relations port.DashboardRelationLookup,
	rules port.HarnessFingerprinter,
) port.DashboardService {
	return &dashboardService{repo: repo, apiKeyIDs: apiKeyIDs, relations: relations, rules: rules}
}

// resolveScope 唯一的权限分流点：admin/demo → nil（全量）；user → 名下 Key（无 Key 时 empty=true）
func (s *dashboardService) resolveScope(ctx context.Context, scope port.DashboardScope) (ids []uint, isEmpty bool, err error) {
	switch scope.Permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		return nil, false, nil
	case enum.PermissionUser:
		keyIDs, lookupErr := s.apiKeyIDs.LookupIDsByUserID(ctx, scope.UserID)
		if lookupErr != nil {
			return nil, false, lookupErr
		}
		if len(keyIDs) == 0 {
			return nil, true, nil
		}
		return keyIDs, false, nil
	default:
		return nil, false, ierr.ErrUnauthorized
	}
}

// validateWindow 时间窗必须有效
func validateWindow(start, end time.Time) error {
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		return ierr.New(ierr.ErrValidation, "invalid dashboard time window")
	}
	return nil
}

// dashboardBuckets 校验并生成时间桶（统一 UTC，与 SQL 的 date_trunc(... AT TIME ZONE 'UTC') 对齐）
func dashboardBuckets(start, end time.Time, g enum.Granularity) ([]time.Time, error) {
	if err := validateWindow(start, end); err != nil {
		return nil, err
	}
	buckets := port.BuildBuckets(start.UTC(), end.UTC(), g, nil)
	if len(buckets) > constant.DashboardMaxBuckets {
		return nil, ierr.New(ierr.ErrValidation, "too many dashboard time buckets")
	}
	return buckets, nil
}

// previousWindow 上一等长周期 [start-(end-start), start)
func previousWindow(start, end time.Time) (prevStart, prevEnd time.Time) {
	return start.Add(-end.Sub(start)), start.Add(-time.Microsecond)
}

// fillStatsSeries 按桶补齐（缺失桶为零值统计，分位数保持 nil）
func fillStatsSeries(points []*modelcall.DashboardStatsPoint, buckets []time.Time) []*modelcall.DashboardStatsPoint {
	byUnix := lo.SliceToMap(points, func(p *modelcall.DashboardStatsPoint) (int64, *modelcall.DashboardStatsPoint) {
		return p.Time.Unix(), p
	})
	return lo.Map(buckets, func(b time.Time, _ int) *modelcall.DashboardStatsPoint {
		if p, ok := byUnix[b.Unix()]; ok {
			return &modelcall.DashboardStatsPoint{Time: b, DashboardStats: p.DashboardStats}
		}
		return &modelcall.DashboardStatsPoint{Time: b}
	})
}

func (s *dashboardService) Overview(ctx context.Context, scope port.DashboardScope, start, end time.Time, g enum.Granularity) (*port.DashboardOverview, error) {
	curBuckets, err := dashboardBuckets(start, end, g)
	if err != nil {
		return nil, err
	}
	prevStart, prevEnd := previousWindow(start, end)
	prevBuckets := port.BuildBuckets(prevStart.UTC(), prevEnd.UTC(), g, nil)

	ids, isEmpty, err := s.resolveScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	if isEmpty {
		return &port.DashboardOverview{
			Current:  port.DashboardPeriod{Summary: &modelcall.DashboardStats{}, Series: fillStatsSeries(nil, curBuckets)},
			Previous: port.DashboardPeriod{Summary: &modelcall.DashboardStats{}, Series: fillStatsSeries(nil, prevBuckets)},
		}, nil
	}

	var (
		curSummary, prevSummary *modelcall.DashboardStats
		curSeries, prevSeries   []*modelcall.DashboardStatsPoint
	)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() (e error) { curSummary, e = s.repo.QueryStats(egCtx, ids, start, end); return e })
	eg.Go(func() (e error) { prevSummary, e = s.repo.QueryStats(egCtx, ids, prevStart, prevEnd); return e })
	eg.Go(func() (e error) { curSeries, e = s.repo.QueryStatsSeries(egCtx, ids, start, end, g); return e })
	eg.Go(func() (e error) { prevSeries, e = s.repo.QueryStatsSeries(egCtx, ids, prevStart, prevEnd, g); return e })
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &port.DashboardOverview{
		Current:  port.DashboardPeriod{Summary: curSummary, Series: fillStatsSeries(curSeries, curBuckets)},
		Previous: port.DashboardPeriod{Summary: prevSummary, Series: fillStatsSeries(prevSeries, prevBuckets)},
	}, nil
}

func (s *dashboardService) ModelSeries(ctx context.Context, scope port.DashboardScope, start, end time.Time, g enum.Granularity) ([]*modelcall.DashboardModelPoint, error) {
	if _, err := dashboardBuckets(start, end, g); err != nil {
		return nil, err
	}
	ids, isEmpty, err := s.resolveScope(ctx, scope)
	if err != nil || isEmpty {
		return []*modelcall.DashboardModelPoint{}, err
	}
	return s.repo.QueryModelSeries(ctx, ids, start, end, g)
}

func (s *dashboardService) TopErrors(ctx context.Context, scope port.DashboardScope, start, end time.Time, limit int) ([]*modelcall.DashboardErrorRow, error) {
	if err := validateWindow(start, end); err != nil {
		return nil, err
	}
	ids, isEmpty, err := s.resolveScope(ctx, scope)
	if err != nil || isEmpty {
		return []*modelcall.DashboardErrorRow{}, err
	}
	return s.repo.QueryTopErrors(ctx, ids, start, end, limit)
}

func (s *dashboardService) EndpointHealth(ctx context.Context, scope port.DashboardScope, start, end time.Time, limit int) ([]*modelcall.DashboardEndpointRow, error) {
	if err := validateWindow(start, end); err != nil {
		return nil, err
	}
	ids, isEmpty, err := s.resolveScope(ctx, scope)
	if err != nil || isEmpty {
		return []*modelcall.DashboardEndpointRow{}, err
	}
	rows, err := s.repo.QueryEndpointHealth(ctx, ids, start, end, limit)
	if err != nil {
		return nil, err
	}
	if scope.Permission != enum.PermissionDemo {
		return rows, nil
	}
	// demo 脱敏规则与审计列表 buildAuditViews 一致：Endpoint 名 MaskSecret
	return lo.Map(rows, func(r *modelcall.DashboardEndpointRow, _ int) *modelcall.DashboardEndpointRow {
		masked := *r
		masked.Endpoint = commonutil.MaskSecret(r.Endpoint)
		return &masked
	}), nil
}

func (s *dashboardService) TopUsage(ctx context.Context, scope port.DashboardScope, start, end time.Time, dim enum.DashboardDimension, limit int) (*port.DashboardTop, error) {
	if err := validateWindow(start, end); err != nil {
		return nil, err
	}
	if dim == enum.DashboardDimensionUser && scope.Permission == enum.PermissionUser {
		return nil, ierr.ErrNoPermission
	}
	ids, isEmpty, err := s.resolveScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	if isEmpty {
		return &port.DashboardTop{Items: []*port.DashboardTopItem{}}, nil
	}

	queryDim := lo.Ternary(dim == enum.DashboardDimensionUser, enum.DashboardDimensionAPIKey, dim)
	fingerprint := ""
	if dim == enum.DashboardDimensionHarness {
		fingerprint = s.harnessFingerprint(ctx)
	}
	rows, err := s.repo.QueryGroupUsage(ctx, ids, start, end, queryDim, fingerprint)
	if err != nil {
		return nil, err
	}
	items, err := s.buildTopItems(ctx, dim, rows, scope.Permission == enum.PermissionDemo)
	if err != nil {
		return nil, err
	}
	return rankTop(items, limit), nil
}

// harnessFingerprint 取规则指纹；失败退化为空串（只影响缓存新鲜度，不影响正确性）
func (s *dashboardService) harnessFingerprint(ctx context.Context) string {
	if s.rules == nil {
		return ""
	}
	fingerprint, err := s.rules.Fingerprint(ctx)
	if err != nil {
		return ""
	}
	return fingerprint
}

// toTopItem 分组行 → 排行项（名称默认取分组键）
func toTopItem(r *modelcall.DashboardGroupRow) *port.DashboardTopItem {
	prompt := r.InputTokens + r.CacheReadTokens + r.CacheWriteTokens
	return &port.DashboardTopItem{
		Key:             r.Key,
		Name:            r.Key,
		Requests:        r.Requests,
		Tokens:          prompt + r.OutputTokens,
		CacheReadTokens: r.CacheReadTokens,
		PromptTokens:    prompt,
		CostMicro:       r.CostMicro,
	}
}

// buildTopItems 按维度回填名称；user 维度把同一用户名下的 Key 合并
func (s *dashboardService) buildTopItems(ctx context.Context, dim enum.DashboardDimension, rows []*modelcall.DashboardGroupRow, isDemo bool) ([]*port.DashboardTopItem, error) {
	if dim != enum.DashboardDimensionUser && dim != enum.DashboardDimensionAPIKey {
		return lo.Map(rows, func(r *modelcall.DashboardGroupRow, _ int) *port.DashboardTopItem { return toTopItem(r) }), nil
	}
	keyIDs := lo.FilterMap(rows, func(r *modelcall.DashboardGroupRow, _ int) (uint, bool) {
		id, err := strconv.ParseUint(r.Key, constant.DecimalBase10, constant.BitSize64)
		return uint(id), err == nil
	})
	relations, err := s.relations.BatchGetRelations(ctx, keyIDs)
	if err != nil {
		return nil, err
	}
	relationOf := func(key string) *modelcall.AuditRelation {
		id, parseErr := strconv.ParseUint(key, constant.DecimalBase10, constant.BitSize64)
		if parseErr != nil {
			return nil
		}
		return relations[uint(id)]
	}
	if dim == enum.DashboardDimensionAPIKey {
		return lo.Map(rows, func(r *modelcall.DashboardGroupRow, _ int) *port.DashboardTopItem {
			item := toTopItem(r)
			if rel := relationOf(r.Key); rel != nil && rel.APIKeyName != "" {
				item.Name = rel.APIKeyName
			}
			if isDemo {
				item.Name = commonutil.MaskSecret(item.Name)
			}
			return item
		}), nil
	}
	return mergeByOwner(rows, relationOf, isDemo), nil
}

// mergeByOwner user 维度：按 Key 归属用户合并（找不到归属的 Key 归入 "0"）
func mergeByOwner(rows []*modelcall.DashboardGroupRow, relationOf func(string) *modelcall.AuditRelation, isDemo bool) []*port.DashboardTopItem {
	byOwner := map[string]*port.DashboardTopItem{}
	order := []string{}
	for _, r := range rows {
		ownerKey, ownerName := "0", ""
		if rel := relationOf(r.Key); rel != nil {
			ownerKey, ownerName = strconv.FormatUint(uint64(rel.UserID), constant.DecimalBase10), rel.UserName
		}
		row := toTopItem(r)
		acc, ok := byOwner[ownerKey]
		if !ok {
			acc = &port.DashboardTopItem{Key: ownerKey, Name: lo.Ternary(isDemo, commonutil.MaskIdentity(ownerName), ownerName)}
			byOwner[ownerKey] = acc
			order = append(order, ownerKey)
		}
		acc.Requests += row.Requests
		acc.Tokens += row.Tokens
		acc.CacheReadTokens += row.CacheReadTokens
		acc.PromptTokens += row.PromptTokens
		acc.CostMicro += row.CostMicro
	}
	return lo.Map(order, func(k string, _ int) *port.DashboardTopItem { return byOwner[k] })
}

// rankTop 按费用、请求数降序取前 limit，其余请求合计
func rankTop(items []*port.DashboardTopItem, limit int) *port.DashboardTop {
	slices.SortStableFunc(items, func(a, b *port.DashboardTopItem) int {
		return cmp.Or(cmp.Compare(b.CostMicro, a.CostMicro), cmp.Compare(b.Requests, a.Requests))
	})
	top := &port.DashboardTop{Items: items[:min(limit, len(items))]}
	for i, item := range items {
		top.TotalCostMicro += item.CostMicro
		if i >= limit {
			top.OtherRequests += item.Requests
		}
	}
	return top
}

func (s *dashboardService) Heatmap(ctx context.Context, scope port.DashboardScope, start, end time.Time, location string) ([]*modelcall.DashboardHeatmapCell, error) {
	if err := validateWindow(start, end); err != nil {
		return nil, err
	}
	if err := validateLocation(location); err != nil {
		return nil, err
	}
	ids, isEmpty, err := s.resolveScope(ctx, scope)
	if err != nil || isEmpty {
		return []*modelcall.DashboardHeatmapCell{}, err
	}
	return s.repo.QueryHeatmap(ctx, ids, start, end, location)
}

// validateLocation 时区必须是 IANA 名（空串与 Local 会被 LoadLocation 接受但 PG 不认，显式拒绝）
func validateLocation(location string) error {
	if location == "" || location == constant.TimezoneLocal {
		return ierr.New(ierr.ErrValidation, "invalid timezone")
	}
	if _, err := time.LoadLocation(location); err != nil {
		return ierr.Wrap(ierr.ErrValidation, err, "invalid timezone")
	}
	return nil
}
