// Package port defines application-layer ports for trace use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// TraceSummaryView 列表项视图
type TraceSummaryView struct {
	ID         uint
	SessionID  string
	Agent      string
	APIKeyName string
	Model      string
	Source     string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TraceDetailView 详情视图
type TraceDetailView struct {
	ID         uint
	SessionID  string
	Agent      string
	APIKeyName string
	Model      string
	CWD        string
	Source     string
	Status     string
	Metadata   map[string]string
	EventCount int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
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
	HookEventName string
	SessionID     string
	Model         string
	CWD           string
	Source        string
	TurnID        string
	RawPayload    []byte
	APIKeyName    string
	UserID        uint
	Records       []ReportTraceRecord
}

// ReportTraceEventHandler 上报 handler 接口
type ReportTraceEventHandler interface {
	Handle(ctx context.Context, cmd ReportTraceEventCommand) error
}

// ListTracesQuery 列表查询
type ListTracesQuery struct {
	UserID   uint
	IsAdmin  bool
	Page     int
	PageSize int
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

// TraceConversationView Trace 对话投影视图。
type TraceConversationView struct {
	TraceID   uint
	SessionID string
	Turns     []*TraceConversationTurnView
}

// TraceConversationTurnView Trace turn 投影视图。
type TraceConversationTurnView struct {
	TurnID string
	Items  []*TraceConversationItemView
}

// TraceConversationItemView Trace 对话项投影视图。
type TraceConversationItemView struct {
	Kind      string
	Role      string
	Content   string
	ToolName  string
	CallID    string
	Arguments string
	Output    string
	Source    string
	RecordIDs []uint
}

// ListTraceConversationQuery Trace 对话查询。
type ListTraceConversationQuery struct {
	UserID  uint
	IsAdmin bool
	TraceID uint
}

// ListTraceConversationHandler Trace 对话查询 handler。
type ListTraceConversationHandler interface {
	Handle(ctx context.Context, q ListTraceConversationQuery) (*TraceConversationView, error)
}
