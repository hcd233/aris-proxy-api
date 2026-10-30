package port

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// ratio 安全除法（分母为 0 时为 0）
func ratio(num, den int64) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

// toDashboardStats 统计块 → DTO（Other 失败数 = 总数 − 成功 − 已归类失败，下限 0）
func toDashboardStats(s *modelcall.DashboardStats) *dto.DashboardStatsItem {
	if s == nil {
		s = &modelcall.DashboardStats{}
	}
	return &dto.DashboardStatsItem{
		Requests: s.Requests,
		Success:  s.Success,
		Failures: &dto.DashboardFailures{
			RateLimit:  s.RateLimitFailures,
			Server:     s.ServerFailures,
			Connection: s.ConnectionFailures,
			Other:      max(0, s.Requests-s.Success-s.RateLimitFailures-s.ServerFailures-s.ConnectionFailures),
		},
		Tokens: &dto.DashboardTokens{
			Input:      s.InputTokens,
			Output:     s.OutputTokens,
			CacheRead:  s.CacheReadTokens,
			CacheWrite: s.CacheWriteTokens,
		},
		Cost:                  dto.PriceDisplayFromMicro(s.CostMicro),
		PricedSuccess:         s.PricedSuccess,
		Latency:               &dto.DashboardLatency{P50: s.LatencyP50Ms, P95: s.LatencyP95Ms, P99: s.LatencyP99Ms},
		OutputTokensPerSecond: s.OutputTokensPerSecond,
		AvgStreamDurationMs:   s.AvgStreamDurationMs,
	}
}

func toDashboardPeriod(p DashboardPeriod) *dto.DashboardPeriodItem {
	return &dto.DashboardPeriodItem{
		Summary: toDashboardStats(p.Summary),
		Series: lo.Map(p.Series, func(pt *modelcall.DashboardStatsPoint, _ int) *dto.DashboardStatsPointItem {
			return &dto.DashboardStatsPointItem{Time: pt.Time, DashboardStatsItem: *toDashboardStats(&pt.DashboardStats)}
		}),
	}
}

// ToDashboardOverviewDTO 总览 → DTO
func ToDashboardOverviewDTO(ov *DashboardOverview) *dto.DashboardOverview {
	return &dto.DashboardOverview{
		Currency: enum.CurrencyUSD,
		Current:  toDashboardPeriod(ov.Current),
		Previous: toDashboardPeriod(ov.Previous),
		Uptime:   toDashboardUptimeDTO(ov.Uptime),
	}
}

// toDashboardUptimeDTO 可用性状态条 → DTO（Other 失败数 = 总数 − 成功 − 已归类失败，下限 0）
func toDashboardUptimeDTO(points []*modelcall.DashboardUptimePoint) []*dto.DashboardUptimeBucketItem {
	return lo.Map(points, func(p *modelcall.DashboardUptimePoint, _ int) *dto.DashboardUptimeBucketItem {
		return &dto.DashboardUptimeBucketItem{
			Start:    p.Time,
			End:      p.End,
			Requests: p.Requests,
			Success:  p.Success,
			Failures: &dto.DashboardFailures{
				RateLimit:  p.RateLimitFailures,
				Server:     p.ServerFailures,
				Connection: p.ConnectionFailures,
				Other:      max(0, p.Requests-p.Success-p.RateLimitFailures-p.ServerFailures-p.ConnectionFailures),
			},
		}
	})
}

// ToDashboardModelSeriesDTO 模型序列 → DTO
func ToDashboardModelSeriesDTO(points []*modelcall.DashboardModelPoint) []*dto.DashboardModelPointItem {
	return lo.Map(points, func(p *modelcall.DashboardModelPoint, _ int) *dto.DashboardModelPointItem {
		return &dto.DashboardModelPointItem{
			ModelID:  p.ModelID,
			Time:     p.Time,
			Requests: p.Requests,
			Tokens:   p.Tokens,
			Cost:     dto.PriceDisplayFromMicro(p.CostMicro),
		}
	})
}

// ToDashboardErrorsDTO 热门错误 → DTO
func ToDashboardErrorsDTO(rows []*modelcall.DashboardErrorRow) []*dto.DashboardErrorItem {
	return lo.Map(rows, func(r *modelcall.DashboardErrorRow, _ int) *dto.DashboardErrorItem {
		return &dto.DashboardErrorItem{StatusCode: r.StatusCode, Message: r.Message, Count: r.Count}
	})
}

// ToDashboardEndpointsDTO Endpoint 健康度 → DTO（错误率 = 失败 / 请求）
func ToDashboardEndpointsDTO(rows []*modelcall.DashboardEndpointRow) []*dto.DashboardEndpointItem {
	return lo.Map(rows, func(r *modelcall.DashboardEndpointRow, _ int) *dto.DashboardEndpointItem {
		return &dto.DashboardEndpointItem{
			Endpoint:              r.Endpoint,
			Requests:              r.Requests,
			ErrorRate:             ratio(r.Failures, r.Requests),
			P95LatencyMs:          r.LatencyP95Ms,
			OutputTokensPerSecond: r.OutputTokensPerSecond,
			Cost:                  dto.PriceDisplayFromMicro(r.CostMicro),
		}
	})
}

// ToDashboardTopDTO 排行 → DTO（命中率分母为 0 时省略；占比分母为 0 时为 0）
func ToDashboardTopDTO(top *DashboardTop) *dto.DashboardTop {
	items := lo.Map(top.Items, func(it *DashboardTopItem, _ int) *dto.DashboardTopItem {
		var hitRate *float64
		if it.PromptTokens > 0 {
			hitRate = lo.ToPtr(ratio(it.CacheReadTokens, it.PromptTokens))
		}
		return &dto.DashboardTopItem{
			Key:          it.Key,
			Name:         it.Name,
			Requests:     it.Requests,
			Tokens:       it.Tokens,
			CacheHitRate: hitRate,
			Cost:         dto.PriceDisplayFromMicro(it.CostMicro),
			CostShare:    ratio(it.CostMicro, top.TotalCostMicro),
		}
	})
	return &dto.DashboardTop{
		Items:         items,
		OtherRequests: top.OtherRequests,
	}
}

// ToDashboardHeatmapDTO 热力图 → DTO（PG ISODOW 1=周一 … 7=周日 → 0=周一 … 6=周日）
func ToDashboardHeatmapDTO(cells []*modelcall.DashboardHeatmapCell) []*dto.DashboardHeatmapCell {
	return lo.Map(cells, func(c *modelcall.DashboardHeatmapCell, _ int) *dto.DashboardHeatmapCell {
		return &dto.DashboardHeatmapCell{Weekday: c.Weekday - 1, Hour: c.Hour, Count: c.Count}
	})
}
