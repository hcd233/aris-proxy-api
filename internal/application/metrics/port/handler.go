// Package port 运行时指标应用层端口
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
package port

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// RuntimeMetricsService 运行时指标聚合查询服务
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type RuntimeMetricsService interface {
	RuntimeMetrics(ctx context.Context, rangeKey string, since int64) (dto.RuntimeSeries, int64, error)
}
