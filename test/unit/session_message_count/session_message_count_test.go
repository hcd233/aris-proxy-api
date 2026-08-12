// Package session_message_count 验证消息数区间桶生成逻辑（固定边界 + 动态上限）。
package session_message_count

import (
	"reflect"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/session/query"
)

func TestBuildMessageCountBuckets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		maxCount     int
		bucketCounts map[int]int64
		want         []string
	}{
		{
			name:         "max within 100, all buckets have data",
			maxCount:     82,
			bucketCounts: map[int]int64{0: 5, 1: 3, 2: 7},
			want:         []string{"0-10", "11-50", "51-82"},
		},
		{
			name:         "empty middle bucket is skipped",
			maxCount:     82,
			bucketCounts: map[int]int64{0: 5, 2: 7},
			want:         []string{"0-10", "51-82"},
		},
		{
			name:         "max below first edge truncates to single bucket",
			maxCount:     8,
			bucketCounts: map[int]int64{0: 5},
			want:         []string{"0-8"},
		},
		{
			name:         "max exactly on edge 10",
			maxCount:     10,
			bucketCounts: map[int]int64{0: 5},
			want:         []string{"0-10"},
		},
		{
			name:         "max beyond 500 uses dynamic last bucket",
			maxCount:     1200,
			bucketCounts: map[int]int64{0: 1, 1: 1, 2: 1, 3: 1, 4: 1, 5: 9},
			want:         []string{"0-10", "11-50", "51-100", "101-200", "201-500", "501-1200"},
		},
		{
			name:         "max 501 only last bucket has data",
			maxCount:     501,
			bucketCounts: map[int]int64{5: 2},
			want:         []string{"501-501"},
		},
		{
			name:         "no sessions returns nil",
			maxCount:     0,
			bucketCounts: map[int]int64{},
			want:         nil,
		},
		{
			name:         "max exactly on edge 50 truncates last bucket",
			maxCount:     50,
			bucketCounts: map[int]int64{0: 5, 1: 3},
			want:         []string{"0-10", "11-50"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := query.BuildMessageCountBuckets(tt.maxCount, tt.bucketCounts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildMessageCountBuckets(%d, %v) = %v, want %v", tt.maxCount, tt.bucketCounts, got, tt.want)
			}
		})
	}
}
