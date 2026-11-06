// Package port defines application-layer ports for dataset export use cases.
package port

import (
	"bufio"
	"context"
	"time"

	"github.com/hcd233/aris-mass/internal/common/enum"
)

// ExportParams 训练数据导出参数
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type ExportParams struct {
	Permission enum.Permission
	UserID     uint
	MinScore   int
	ModelIDs   []string
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

// ExportStream 一次流式导出
//
//	Total 在开始写之前即已确定（匹配的会话数）；Write 把 ShareGPT JSONL 按会话分批读取、逐行写入 w，
//	中途出错即返回，客户端以收到的行数少于 Total 判定导出不完整。
//
//	@author centonhuang
//	@update 2026-11-06 10:00:00
type ExportStream struct {
	Total int
	Write func(w *bufio.Writer) error
}

// ExportDatasetHandler 流式导出处理器（NDJSON）
//
//	筛选、鉴权与"无匹配会话"等错误在返回 ExportStream 之前给出，便于以普通 JSON 错误响应。
//
//	@author centonhuang
//	@update 2026-11-06 10:00:00
type ExportDatasetHandler interface {
	Handle(ctx context.Context, p ExportParams) (*ExportStream, error)
}

// FormatPreviewResult 单条会话格式预览结果
//
//	@author centonhuang
//	@update 2026-07-06 10:00:00
type FormatPreviewResult struct {
	SessionID    uint
	Offset       int
	TotalCount   int
	ShareGPTJSON string
}

// PreviewFormatDatasetHandler 单条会话格式预览处理器
//
//	@author centonhuang
//	@update 2026-07-06 10:00:00
type PreviewFormatDatasetHandler interface {
	Handle(ctx context.Context, p ExportParams, offset int) (*FormatPreviewResult, error)
}
