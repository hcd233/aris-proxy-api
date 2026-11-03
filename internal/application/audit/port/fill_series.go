package port

import (
	"slices"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
)

// BuildBuckets 生成完整时间桶序列；start/end 无效时退化为 fallback 集合中的桶。
//
// GROUP BY date_trunc 不会为没有调用的时间槽生成行；图表需要连续的槽位，由调用方按本序列补齐。
func BuildBuckets(start, end time.Time, granularity enum.Granularity, fallback map[time.Time]struct{}) []time.Time {
	if start.IsZero() || end.IsZero() || start.After(end) {
		buckets := lo.Keys(fallback)
		slices.SortFunc(buckets, func(a, b time.Time) int { return a.Compare(b) })
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

// fixedStepGranularities 非日历对齐的定宽粒度：桶边界按 Unix 纪元整除（UTC 下与整点 / 零点对齐），
// 与 SQL 侧 date_bin(..., 纪元原点) 一致。
var fixedStepGranularities = map[enum.Granularity]time.Duration{
	enum.GranularityFiveMinutes:   constant.DashboardFiveMinuteBucket,
	enum.GranularityTenMinutes:    constant.DashboardTenMinuteBucket,
	enum.GranularityThirtyMinutes: constant.DashboardThirtyMinuteBucket,
	enum.GranularityFourHours:     constant.DashboardFourHourBucket,
}

// fixedStepOf 返回定宽粒度的桶宽；minute/hour/day/week 走 date_trunc 日历截断，返回 false。
func fixedStepOf(granularity enum.Granularity) (time.Duration, bool) {
	step, ok := fixedStepGranularities[granularity]
	return step, ok
}

func bucketStep(granularity enum.Granularity) time.Duration {
	if step, ok := fixedStepOf(granularity); ok {
		return step
	}
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
	if step, ok := fixedStepOf(granularity); ok {
		// Time.Truncate 以零时刻（UTC 零点）为原点，桶宽都整除 24h，故与 Unix 纪元原点等价
		return t.Truncate(step)
	}
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
