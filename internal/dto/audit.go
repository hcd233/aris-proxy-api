package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// ListAuditLogsReq 审计日志列表请求
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type ListAuditLogsReq struct {
	Page      int       `query:"page" required:"true" minimum:"1"`
	PageSize  int       `query:"pageSize" required:"true" minimum:"1" maximum:"100"`
	Query     string    `query:"query" maxLength:"100"`
	Sort      enum.Sort `query:"sort" enum:"asc,desc"`
	SortField string    `query:"sortField" maxLength:"50"`
	StartTime time.Time `query:"startTime"`
	EndTime   time.Time `query:"endTime"`
}

// ListAuditLogsRsp 审计日志列表响应
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type ListAuditLogsRsp struct {
	CommonRsp
	Logs     []*AuditLogItem `json:"logs,omitempty" doc:"审计日志列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// AuditLogItem 审计日志条目
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type AuditLogItem struct {
	ID                        uint      `json:"id" doc:"记录ID"`
	CreatedAt                 time.Time `json:"createdAt" doc:"创建时间"`
	Model                     string    `json:"model" doc:"模型名"`
	UpstreamProvider          string    `json:"upstreamProvider" doc:"上游提供商"`
	APIProvider               string    `json:"apiProvider" doc:"接口协议"`
	InputTokens               int       `json:"inputTokens" doc:"输入token数"`
	OutputTokens              int       `json:"outputTokens" doc:"输出token数"`
	CacheCreationInputTokens  int       `json:"cacheCreationInputTokens" doc:"缓存写入token数"`
	CacheReadInputTokens      int       `json:"cacheReadInputTokens" doc:"缓存命中token数"`
	FirstTokenLatencyMs       int64     `json:"firstTokenLatencyMs" doc:"首token延迟(ms)"`
	StreamDurationMs          int64     `json:"streamDurationMs" doc:"流式持续时间(ms)"`
	UserAgent                 string    `json:"userAgent" doc:"User-Agent"`
	UpstreamStatusCode        int       `json:"upstreamStatusCode" doc:"上游状态码"`
	ErrorMessage              string    `json:"errorMessage" doc:"错误信息"`
	TraceID                   string    `json:"traceId" doc:"Trace ID"`
}
