package dashboard_service

import (
	"testing"
	"time"

	"github.com/hcd233/aris-mass/internal/application/audit/port"
	"github.com/hcd233/aris-mass/internal/common/enum"
)

// 定宽粒度的桶边界按 Unix 纪元整除（UTC 下与整点 / 零点对齐），步长恒定，与 SQL 侧 date_bin 纪元原点一致
func TestBuildBuckets_FixedStepGranularity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		granularity enum.Granularity
		start, end  time.Time
		step        time.Duration
		first       time.Time
		count       int
	}{
		{
			granularity: enum.GranularityTenMinutes,
			start:       time.Date(2026, 10, 27, 0, 7, 30, 0, time.UTC),
			end:         time.Date(2026, 10, 28, 0, 7, 30, 0, time.UTC),
			step:        10 * time.Minute,
			first:       time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC),
			count:       145,
		},
		{
			granularity: enum.GranularityFourHours,
			start:       time.Date(2026, 10, 21, 5, 0, 0, 0, time.UTC),
			end:         time.Date(2026, 10, 28, 5, 0, 0, 0, time.UTC),
			step:        4 * time.Hour,
			first:       time.Date(2026, 10, 21, 4, 0, 0, 0, time.UTC),
			count:       43,
		},
		{
			granularity: enum.GranularityFiveMinutes,
			start:       time.Date(2026, 10, 27, 23, 58, 0, 0, time.UTC),
			end:         time.Date(2026, 10, 28, 0, 3, 0, 0, time.UTC),
			step:        5 * time.Minute,
			first:       time.Date(2026, 10, 27, 23, 55, 0, 0, time.UTC),
			count:       2,
		},
		{
			granularity: enum.GranularityThirtyMinutes,
			start:       time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC),
			end:         time.Date(2026, 10, 27, 2, 0, 0, 0, time.UTC),
			step:        30 * time.Minute,
			first:       time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC),
			count:       5,
		},
	}
	for _, tc := range cases {
		t.Run(tc.granularity, func(t *testing.T) {
			t.Parallel()
			buckets := port.BuildBuckets(tc.start, tc.end, tc.granularity, nil)
			if len(buckets) != tc.count {
				t.Fatalf("bucket count = %d, want %d", len(buckets), tc.count)
			}
			if !buckets[0].Equal(tc.first) {
				t.Errorf("first bucket = %v, want %v", buckets[0], tc.first)
			}
			for i := 1; i < len(buckets); i++ {
				if got := buckets[i].Sub(buckets[i-1]); got != tc.step {
					t.Fatalf("step at %d = %v, want %v", i, got, tc.step)
				}
			}
		})
	}
}
