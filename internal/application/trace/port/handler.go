// Package port defines application-layer ports for trace use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// TraceSummaryView 列表项视图
type TraceSummaryView struct {
	ID            uint
	SessionID     string
	ParentTraceID uint
	Agent         string
	APIKeyName    string
	Model         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TraceDetailView 详情视图
type TraceDetailView struct {
	ID            uint
	SessionID     string
	ParentTraceID uint
	Agent         string
	APIKeyName    string
	Model         string
	CWD           string
	Metadata      map[string]string
	EventCount    int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TraceEventView 事件视图
type TraceEventView struct {
	ID             uint
	Source         string
	RecordType     string
	Event          string
	TurnID         string
	CallID         string
	TranscriptLine *int64
	ClientSequence int64
	DedupKey       string
	Payload        []byte
	CreatedAt      time.Time
}

// ReportTraceRecord 单条原始 Trace 记录。
type ReportTraceRecord struct {
	Source         string
	RecordType     string
	HookEventName  string
	Event          string
	TurnID         string
	CallID         string
	TranscriptLine *int64
	ClientSequence int64
	DedupKey       string
	Payload        []byte
}

// ReportTraceEventCommand 上报事件命令
type ReportTraceEventCommand struct {
	SessionID       string
	ParentSessionID string
	Agent           string
	Model           string
	CWD             string
	AgentID         string
	AgentType       string
	APIKeyName      string
	Records         []ReportTraceRecord
}

// ReportTraceRecordResult 单条上报处理结果。
type ReportTraceRecordResult struct {
	DedupKey string
	Status   string
	Message  string
}

// ReportTraceEventHandler 上报 handler 接口
type ReportTraceEventHandler interface {
	Handle(ctx context.Context, cmd ReportTraceEventCommand) ([]ReportTraceRecordResult, error)
}

// ListTracesQuery 列表查询
type ListTracesQuery struct {
	UserID   uint
	IsAdmin  bool
	Page     int
	PageSize int
	Query    string
}

// ListTracesHandler 列表 handler 接口
type ListTracesHandler interface {
	Handle(ctx context.Context, q ListTracesQuery) ([]*TraceSummaryView, *model.PageInfo, error)
}

// GetTraceQuery 详情查询
type GetTraceQuery struct {
	UserID  uint
	IsAdmin bool
	TraceID uint
}

// GetTraceHandler 详情 handler 接口
type GetTraceHandler interface {
	Handle(ctx context.Context, q GetTraceQuery) (*TraceDetailView, error)
}

// ListTraceEventsQuery 事件时间线查询
type ListTraceEventsQuery struct {
	UserID   uint
	IsAdmin  bool
	TraceID  uint
	Page     int
	PageSize int
}

// ListTraceEventsHandler 事件时间线 handler 接口
type ListTraceEventsHandler interface {
	Handle(ctx context.Context, q ListTraceEventsQuery) ([]*TraceEventView, *model.PageInfo, error)
}

// DeleteTraceCommand 删除 Trace 命令
type DeleteTraceCommand struct {
	UserID  uint
	IsAdmin bool
	IDs     []uint
}

// DeleteTraceFailedItem 删除失败项
type DeleteTraceFailedItem struct {
	ID    uint
	Error string
}

// DeleteTraceResult 删除结果
type DeleteTraceResult struct {
	DeletedCount int
	Failures     []DeleteTraceFailedItem
}

// DeleteTraceHandler 删除命令处理器接口（支持单个和批量）
type DeleteTraceHandler interface {
	Handle(ctx context.Context, cmd DeleteTraceCommand) (*DeleteTraceResult, error)
}
