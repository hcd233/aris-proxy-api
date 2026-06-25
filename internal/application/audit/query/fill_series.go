package query

import (
	"sort"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// indexSeries 提取 (model, time) 索引：返回 model 出现顺序 + 嵌套 map + 全局 time 桶集合。
func indexSeries[P any, V any](
	points []P,
	modelOf func(P) string,
	timeOf func(P) time.Time,
	valueOf func(P) V,
) (modelOrder []string, byModel map[string]map[time.Time]V, timeSet map[time.Time]struct{}) {
	seenModel := make(map[string]struct{}, len(points))
	modelOrder = make([]string, 0, len(points))
	byModel = make(map[string]map[time.Time]V, len(points))
	timeSet = make(map[time.Time]struct{}, len(points))
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
		buckets := lo.Keys(fallback)
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
