// Package dto Option List DTO
package dto

import "time"

// AuditOptionListReq 审计筛选选项请求
//
//	@author centonhuang
//	@update 2026-06-10 12:00:00
type AuditOptionListReq struct {
	Field     string    `query:"field" required:"true" enum:"user,model,status" doc:"筛选字段"`
	Keyword   string    `query:"keyword" maxLength:"100" doc:"搜索关键词"`
	StartTime time.Time `query:"startTime" doc:"筛选起始时间"`
	EndTime   time.Time `query:"endTime" doc:"筛选结束时间"`
}

// AuditOptionListRsp 审计筛选选项响应
//
//	@author centonhuang
//	@update 2026-06-09 10:00:00
type AuditOptionListRsp struct {
	CommonRsp
	Items []string `json:"items" doc:"选项列表"`
}

// SessionOptionListReq 会话筛选选项请求
//
//	@author centonhuang
//	@update 2026-06-16 14:00:00
type SessionOptionListReq struct {
	Field     string    `query:"field" required:"true" enum:"score,model" doc:"筛选字段"`
	Keyword   string    `query:"keyword" maxLength:"100" doc:"搜索关键词"`
	StartTime time.Time `query:"startTime" doc:"筛选起始时间"`
	EndTime   time.Time `query:"endTime" doc:"筛选结束时间"`
}

// SessionOptionListRsp 会话筛选选项响应
//
//	@author centonhuang
//	@update 2026-06-10 12:00:00
type SessionOptionListRsp struct {
	CommonRsp
	Items []string `json:"items" doc:"选项列表"`
}
