package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// ListCronJobsReq 列出 CronJob 请求
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type ListCronJobsReq struct {
	Page      int       `query:"page" required:"true" minimum:"1"`
	PageSize  int       `query:"pageSize" required:"true" minimum:"1" maximum:"100"`
	Query     string    `query:"query" maxLength:"100"`
	Sort      enum.Sort `query:"sort" enum:"asc,desc"`
	SortField string    `query:"sortField" maxLength:"50"`
}

// ListCronJobsRsp 列出 CronJob 响应
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type ListCronJobsRsp struct {
	CommonRsp
	Jobs     []*CronJobItem  `json:"jobs,omitempty" doc:"CronJob 列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// CronJobItem CronJob 条目
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronJobItem struct {
	Name        string    `json:"name" doc:"任务名"`
	Type        string    `json:"type" doc:"任务类型: functional/core"`
	Spec        string    `json:"spec" doc:"cron 表达式"`
	Description string    `json:"description" doc:"任务描述"`
	Enabled     bool      `json:"enabled" doc:"是否启用"`
	CreatedAt   time.Time `json:"createdAt" doc:"创建时间"`
	UpdatedAt   time.Time `json:"updatedAt" doc:"更新时间"`
}

// UpdateCronJobReq 更新 CronJob 请求
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type UpdateCronJobReq struct {
	Name string                `query:"name" required:"true" maxLength:"100" doc:"任务名"`
	Body *UpdateCronJobReqBody `json:"body" doc:"请求体"`
}

// UpdateCronJobReqBody 更新 CronJob 请求体
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type UpdateCronJobReqBody struct {
	Enabled *bool   `json:"enabled,omitempty" doc:"是否启用"`
	Spec    *string `json:"spec,omitempty" doc:"cron 表达式，如 */5 * * * *"`
}

// UpdateCronJobRsp 更新 CronJob 响应
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type UpdateCronJobRsp struct {
	CommonRsp
}

// ListCronCallAuditsReq 列出 CronCallAudit 请求
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type ListCronCallAuditsReq struct {
	Page      int       `query:"page" required:"true" minimum:"1"`
	PageSize  int       `query:"pageSize" required:"true" minimum:"1" maximum:"100"`
	Query     string    `query:"query" maxLength:"100"`
	Sort      enum.Sort `query:"sort" enum:"asc,desc"`
	SortField string    `query:"sortField" maxLength:"50"`
	StartTime time.Time `query:"startTime" doc:"开始时间"`
	EndTime   time.Time `query:"endTime" doc:"结束时间"`
	Filter    string    `query:"filter" maxLength:"500" doc:"筛选表达式，格式: field:value field2:!value2"`
}

// ListCronCallAuditsRsp 列出 CronCallAudit 响应
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type ListCronCallAuditsRsp struct {
	CommonRsp
	Logs     []*CronCallAuditItem `json:"logs,omitempty" doc:"CronCallAudit 列表"`
	PageInfo *model.PageInfo      `json:"pageInfo,omitempty" doc:"分页信息"`
}

// CronCallAuditItem CronCallAudit 条目
//
//	@author centonhuang
//	@update 2026-06-18 10:00:00
type CronCallAuditItem struct {
	ID         uint           `json:"id" doc:"记录ID"`
	CronName   string         `json:"cronName" doc:"任务名"`
	TraceID    string         `json:"traceId" doc:"Trace ID"`
	StartedAt  time.Time      `json:"startedAt" doc:"开始时间"`
	EndedAt    time.Time      `json:"endedAt" doc:"结束时间"`
	DurationMs int64          `json:"durationMs" doc:"执行耗时(ms)"`
	Status     string         `json:"status" doc:"执行状态"`
	Message    string         `json:"message" doc:"附加信息"`
	Metadata   map[string]any `json:"metadata" doc:"执行元数据"`
	CreatedAt  time.Time      `json:"createdAt" doc:"创建时间"`
}

// CronCallAuditOptionListReq CronCallAudit 筛选项请求
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronCallAuditOptionListReq struct {
	Field     string    `query:"field" required:"true" enum:"type,status" doc:"筛选字段"`
	Keyword   string    `query:"keyword" maxLength:"100" doc:"关键词"`
	StartTime time.Time `query:"startTime" doc:"开始时间"`
	EndTime   time.Time `query:"endTime" doc:"结束时间"`
}

// CronCallAuditOptionListRsp CronCallAudit 筛选项响应
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronCallAuditOptionListRsp struct {
	CommonRsp
	Items []string `json:"items,omitempty" doc:"选项列表"`
}
