package query

import (
	"sort"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// FillTrendSeries 把 SQL 返回的稀疏点集补齐为每个 model 拥有相同时间槽的折线数据。
//
// GROUP BY date_trunc 不会为没有调用的时间槽生成行；前端折线图需要这些槽位才能连续绘制。
// 缺失槽位以 count=0 填充。
func FillTrendSeries(points []*modelcall.ModelTrendPoint) []*dto.ModelTrendItem {
	modelOrder, byModel, buckets := indexSeries(points,
		func(p *modelcall.ModelTrendPoint) string { return p.Model },
		func(p *modelcall.ModelTrendPoint) time.Time { return p.Time },
		func(p *modelcall.ModelTrendPoint) int { return p.Count },
	)
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
func FillRateSeries(points []*modelcall.RequestRatePoint) []*dto.RequestRateItem {
	type slot struct{ total, success int }
	modelOrder, byModel, buckets := indexSeries(points,
		func(p *modelcall.RequestRatePoint) string { return p.Model },
		func(p *modelcall.RequestRatePoint) time.Time { return p.Time },
		func(p *modelcall.RequestRatePoint) slot { return slot{total: p.Total, success: p.Success} },
	)
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

// indexSeries 提取 (model, time) 索引：返回 model 出现顺序 + 嵌套 map + 全局排序后的 time 桶。
func indexSeries[P any, V any](
	points []P,
	modelOf func(P) string,
	timeOf func(P) time.Time,
	valueOf func(P) V,
) ([]string, map[string]map[time.Time]V, []time.Time) {
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
	buckets := make([]time.Time, 0, len(timeSet))
	for t := range timeSet {
		buckets = append(buckets, t)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Before(buckets[j]) })
	return modelOrder, byModel, buckets
}
