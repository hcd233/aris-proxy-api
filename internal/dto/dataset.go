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
	MinScore  int       `query:"minScore" minimum:"0" maximum:"5" default:"0" doc:"最低评分阈值(1-5)，0 表示不限制"`
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
	MinScore  int       `query:"minScore" minimum:"0" maximum:"5" default:"0" doc:"最低评分阈值(1-5)，0 表示不限制"`
	Models    []string  `query:"models" doc:"模型列表筛选(逗号分隔)"`
	StartTime time.Time `query:"startTime" doc:"起始时间"`
	EndTime   time.Time `query:"endTime" doc:"结束时间"`
}
