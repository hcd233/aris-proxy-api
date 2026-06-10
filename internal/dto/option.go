// Package dto Option List DTO
package dto

// AuditOptionListReq 审计筛选选项请求
//
//	@author centonhuang
//	@update 2026-06-09 10:00:00
type AuditOptionListReq struct {
	Field   string `query:"field" required:"true" enum:"user,model,status" doc:"筛选字段"`
	Keyword string `query:"keyword" maxLength:"100" doc:"搜索关键词"`
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
//	@update 2026-06-09 10:00:00
type SessionOptionListReq struct {
	Field   string `query:"field" required:"true" enum:"score" doc:"筛选字段"`
	Keyword string `query:"keyword" maxLength:"100" doc:"搜索关键词"`
}

// SessionOptionListRsp 会话筛选选项响应
//
//	@author centonhuang
//	@update 2026-06-09 10:00:00
type SessionOptionListRsp struct {
	CommonRsp
	Items []OptionItem `json:"items" doc:"选项列表"`
}

// OptionItem 选项项
//
//	@author centonhuang
//	@update 2026-06-09 10:00:00
type OptionItem struct {
	Value string `json:"value" doc:"值"`
	Label string `json:"label" doc:"显示标签"`
}
