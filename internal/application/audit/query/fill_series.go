package query

import (
	"sort"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// FillTrendSeries 把 SQL 返回的稀疏点集补齐为每个 model 拥有相同时间槽的折线数据。
//
// GROUP BY date_trunc 不会为没有调用的时间槽生成行；前端折线图需要这些槽位才能连续绘制。
// 缺失槽位以 count=0 填充。start/end 非零时，按请求区间补齐完整时间轴。
func FillTrendSeries(points []*modelcall.ModelTrendPoint, start, end time.Time, granularity enum.Granularity) []*dto.ModelTrendItem {
	modelOrder, byModel, timeSet := indexSeries(points,
		func(p *modelcall.ModelTrendPoint) string { return p.Model },
		func(p *modelcall.ModelTrendPoint) time.Time { return p.Time },
		func(p *modelcall.ModelTrendPoint) int { return p.Count },
	)
	buckets := buildBuckets(start, end, granularity, timeSet)
	items := make([]*dto.ModelTrendItem, 0, len(modelOrder))
	for _, m := range modelOrder {
		pts := make([]*dto.TrendPoint, 0, len(buckets))
		for _, t := range buckets {
			pts = append(pts, &dto.TrendPoint{Time: t, Count: byModel[m][t]})
		}
		items = append(items, &dto.ModelTrendItem{Model: m, Points: pts})
	}
	return items
}

// FillRateSeries 同 FillTrendSeries，并计算 successRate 与 failed。
func FillRateSeries(points []*modelcall.RequestRatePoint, start, end time.Time, granularity enum.Granularity) []*dto.RequestRateItem {
	type slot struct{ total, success int }
	modelOrder, byModel, timeSet := indexSeries(points,
		func(p *modelcall.RequestRatePoint) string { return p.Model },
		func(p *modelcall.RequestRatePoint) time.Time { return p.Time },
		func(p *modelcall.RequestRatePoint) slot { return slot{total: p.Total, success: p.Success} },
	)
	buckets := buildBuckets(start, end, granularity, timeSet)
	items := make([]*dto.RequestRateItem, 0, len(modelOrder))
	for _, m := range modelOrder {
		pts := make([]*dto.RatePoint, 0, len(buckets))
		for _, t := range buckets {
			s := byModel[m][t]
			var rate float64
			if s.total > 0 {
				rate = float64(s.success) / float64(s.total)
			}
			pts = append(pts, &dto.RatePoint{
				Time:        t,
				Total:       s.total,
				Success:     s.success,
				Failed:      s.total - s.success,
				SuccessRate: rate,
			})
		}
		items = append(items, &dto.RequestRateItem{Model: m, Points: pts})
	}
	return items
}

// indexSeries 提取 (model, time) 索引：返回 model 出现顺序 + 嵌套 map + 全局 time 桶集合。
func indexSeries[P any, V any](
	points []P,
	modelOf func(P) string,
	timeOf func(P) time.Time,
	valueOf func(P) V,
) ([]string, map[string]map[time.Time]V, map[time.Time]struct{}) {
	seenModel := make(map[string]struct{}, len(points))
	modelOrder := make([]string, 0, len(points))
	byModel := make(map[string]map[time.Time]V, len(points))
	timeSet := make(map[time.Time]struct{}, len(points))
	for _, p := range points {
		m := modelOf(p)
		t := timeOf(p)
		if _, ok := seenModel[m]; !ok {
			seenModel[m] = struct{}{}
			modelOrder = append(modelOrder, m)
			byModel[m] = make(map[time.Time]V)
		}
		byModel[m][t] = valueOf(p)
		timeSet[t] = struct{}{}
	}
	return modelOrder, byModel, timeSet
}

func buildBuckets(start, end time.Time, granularity enum.Granularity, fallback map[time.Time]struct{}) []time.Time {
	if start.IsZero() || end.IsZero() || start.After(end) {
		buckets := make([]time.Time, 0, len(fallback))
		for t := range fallback {
			buckets = append(buckets, t)
		}
		sort.Slice(buckets, func(i, j int) bool { return buckets[i].Before(buckets[j]) })
		return buckets
	}

	step := bucketStep(granularity)
	endBucket := truncateBucket(end, granularity)
	buckets := make([]time.Time, 0)
	for t := truncateBucket(start, granularity); !t.After(endBucket); t = t.Add(step) {
		buckets = append(buckets, t)
	}
	return buckets
}

func bucketStep(granularity enum.Granularity) time.Duration {
	switch granularity {
	case enum.GranularityMinute:
		return time.Minute
	case enum.GranularityHour:
		return time.Hour
	case enum.GranularityWeek:
		return constant.DurationWeek
	default:
		return constant.DurationDay
	}
}

func truncateBucket(t time.Time, granularity enum.Granularity) time.Time {
	switch granularity {
	case enum.GranularityMinute:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
	case enum.GranularityHour:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case enum.GranularityWeek:
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		daysSinceMonday := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -daysSinceMonday)
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

type throughputSlot struct {
	inputTokens         int
	outputTokens        int
	cacheCreationTokens int
	cacheReadTokens     int
	outputTokensPerSec  float64
}

func FillTokenThroughputSeries(points []*modelcall.TokenThroughputPoint, start, end time.Time, granularity enum.Granularity) []*dto.TokenThroughputItem {
	modelOrder, byModel, timeSet := indexSeries(points,
		func(p *modelcall.TokenThroughputPoint) string { return p.Model },
		func(p *modelcall.TokenThroughputPoint) time.Time { return p.Time },
		func(p *modelcall.TokenThroughputPoint) throughputSlot {
			return throughputSlot{
				inputTokens:         p.InputTokens,
				outputTokens:        p.OutputTokens,
				cacheCreationTokens: p.CacheCreationTokens,
				cacheReadTokens:     p.CacheReadTokens,
				outputTokensPerSec:  p.OutputTokensPerSecond,
			}
		},
	)
	buckets := buildBuckets(start, end, granularity, timeSet)
	items := make([]*dto.TokenThroughputItem, 0, len(modelOrder))
	for _, m := range modelOrder {
		pts := make([]*dto.TokenThroughputPoint, 0, len(buckets))
		for _, t := range buckets {
			s, ok := byModel[m][t]
			tp := &dto.TokenThroughputPoint{Time: t}
			if ok {
				tp.InputTokens = s.inputTokens
				tp.OutputTokens = s.outputTokens
				tp.CacheCreationTokens = s.cacheCreationTokens
				tp.CacheReadTokens = s.cacheReadTokens
				tp.OutputTokensPerSecond = s.outputTokensPerSec
			}
			pts = append(pts, tp)
		}
		items = append(items, &dto.TokenThroughputItem{Model: m, Points: pts})
	}
	return items
}
