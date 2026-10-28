// Package port defines application-layer ports for audit use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// AuditLogView 审计日志只读投影
type AuditLogView struct {
	ID                       uint
	CreatedAt                time.Time
	ModelID                  string
	UpstreamProtocol         string
	APIProtocol              string
	Endpoint                 string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	FirstTokenLatencyMs      int64
	StreamDurationMs         int64
	UserAgent                string
	UpstreamStatusCode       int
	ErrorMessage             string
	TraceID                  string
	APIKeyName               string
	UserName                 string
	UserEmail                string
	CostMicro                *int64
	PricingCurrency          enum.Currency
}

// ListAuditLogsParams 列表查询的通用参数（不带权限相关字段）
type ListAuditLogsParams struct {
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
	Filter    string
}

// AuditService 统一按当前 JWT 权限派发审计查询，handler 只负责 DTO 映射
type AuditService interface {
	ListLogs(ctx context.Context, permission enum.Permission, userID uint, q ListAuditLogsParams) ([]*AuditLogView, *model.PageInfo, error)
	ListAuditOption(ctx context.Context, permission enum.Permission, userID uint, field, keyword string, startTime, endTime time.Time) ([]string, error)
}
