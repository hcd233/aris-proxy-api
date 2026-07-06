// Package dto Dataset export DTOs
package dto

import (
	"time"
)

// DatasetPreviewReq 数据集统计预览请求（GET query 参数）
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type DatasetPreviewReq struct {
	MinScore  int       `query:"minScore" minimum:"0" maximum:"5" doc:"最低评分阈值(0-5, 0=不筛选)"`
	Models    []string  `query:"models" doc:"模型列表筛选(逗号分隔)"`
	StartTime time.Time `query:"startTime" doc:"起始时间"`
	EndTime   time.Time `query:"endTime" doc:"结束时间"`
}

// DatasetPreviewRsp 数据集统计预览响应
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type DatasetPreviewRsp struct {
	CommonRsp
	TotalSessions     int            `json:"totalSessions,omitempty" doc:"匹配会话总数"`
	ScoreDistribution map[int]int    `json:"scoreDistribution,omitempty" doc:"评分分布(分值→数量)"`
	ModelDistribution map[string]int `json:"modelDistribution,omitempty" doc:"模型分布(模型名→数量)"`
}

// DatasetExportReq 数据集流式导出请求（GET query 参数）
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
type DatasetExportReq struct {
	MinScore  int       `query:"minScore" minimum:"0" maximum:"5" doc:"最低评分阈值(0-5, 0=不筛选)"`
	Models    []string  `query:"models" doc:"模型列表筛选(逗号分隔)"`
	StartTime time.Time `query:"startTime" doc:"起始时间"`
	EndTime   time.Time `query:"endTime" doc:"结束时间"`
}

// DatasetFormatPreviewReq 单条会话格式预览请求（GET query 参数）
//
//	@author centonhuang
//	@update 2026-07-06 10:00:00
type DatasetFormatPreviewReq struct {
	MinScore  int       `query:"minScore" minimum:"0" maximum:"5" doc:"最低评分阈值(0-5, 0=不筛选)"`
	Models    []string  `query:"models" doc:"模型列表筛选(逗号分隔)"`
	StartTime time.Time `query:"startTime" doc:"起始时间"`
	EndTime   time.Time `query:"endTime" doc:"结束时间"`
	Offset    int       `query:"offset" minimum:"0" doc:"会话索引偏移量"`
}

// DatasetFormatPreviewRsp 单条会话格式预览响应
//
//	@author centonhuang
//	@update 2026-07-06 10:00:00
type DatasetFormatPreviewRsp struct {
	CommonRsp
	SessionID    uint   `json:"sessionId,omitempty" doc:"会话 ID"`
	Offset       int    `json:"offset,omitempty" doc:"当前偏移量"`
	TotalCount   int    `json:"totalCount,omitempty" doc:"匹配会话总数"`
	ShareGPTJSON string `json:"sharegptJson,omitempty" doc:"ShareGPT 格式的一行 JSON"`
}
