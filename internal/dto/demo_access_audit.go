package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// ListDemoAccessAuditsReq 列出 Demo 访问审计请求（admin only）
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type ListDemoAccessAuditsReq struct {
	Page      int       `query:"page" required:"true" minimum:"1"`
	PageSize  int       `query:"pageSize" required:"true" minimum:"1" maximum:"500"`
	Query     string    `query:"query" maxLength:"100"`
	Sort      enum.Sort `query:"sort" enum:"asc,desc"`
	SortField string    `query:"sortField" maxLength:"50"`
	StartTime time.Time `query:"startTime"`
	EndTime   time.Time `query:"endTime"`
	Filter    string    `query:"filter" maxLength:"500" doc:"筛选表达式，格式: field:value field2:!value2"`
}

// ListDemoAccessAuditsRsp 列出 Demo 访问审计响应
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type ListDemoAccessAuditsRsp struct {
	CommonRsp
	Logs     []*DemoAccessAuditItem `json:"logs,omitempty" doc:"Demo 访问审计列表"`
	PageInfo *model.PageInfo        `json:"pageInfo,omitempty" doc:"分页信息"`
}

// DemoAccessAuditItem Demo 访问审计条目
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAuditItem struct {
	ID        uint      `json:"id" doc:"记录ID"`
	Action    string    `json:"action" doc:"动作:login/login_denied/module_access/module_denied"`
	Module    string    `json:"module" doc:"demo 模块名，login 类为空串"`
	Path      string    `json:"path" doc:"请求路径"`
	IP        string    `json:"ip" doc:"客户端 IP"`
	UserAgent string    `json:"userAgent" doc:"User-Agent"`
	Reason    string    `json:"reason" doc:"拒绝原因:login_disabled/no_demo_user/module_closed"`
	CreatedAt time.Time `json:"createdAt" doc:"创建时间"`
}

// DemoAccessAuditOptionListReq Demo 访问审计筛选项请求
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAuditOptionListReq struct {
	Field     string    `query:"field" required:"true" enum:"action,module" doc:"筛选字段"`
	Keyword   string    `query:"keyword" maxLength:"100" doc:"关键词"`
	StartTime time.Time `query:"startTime"`
	EndTime   time.Time `query:"endTime"`
}

// DemoAccessAuditOptionListRsp Demo 访问审计筛选项响应
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAuditOptionListRsp struct {
	CommonRsp
	Items []string `json:"items,omitempty" doc:"选项列表"`
}
