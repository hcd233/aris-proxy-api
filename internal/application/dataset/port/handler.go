// Package port defines application-layer ports for dataset export use cases.
package port

import (
	"context"
	"io"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// ExportParams 训练数据导出参数
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type ExportParams struct {
	Permission enum.Permission
	UserID     uint
	MinScore   *int
	Models     []string
	StartTime  time.Time
	EndTime    time.Time
}

// PreviewResult 统计预览结果
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type PreviewResult struct {
	TotalSessions     int            `json:"totalSessions"`
	ScoreDistribution map[int]int    `json:"scoreDistribution"`
	ModelDistribution map[string]int `json:"modelDistribution"`
}

// PreviewDatasetHandler 统计预览处理器
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type PreviewDatasetHandler interface {
	Handle(ctx context.Context, p ExportParams) (*PreviewResult, error)
}

// ExportDatasetHandler 流式导出处理器
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type ExportDatasetHandler interface {
	Handle(ctx context.Context, p ExportParams, w io.Writer) error
}
