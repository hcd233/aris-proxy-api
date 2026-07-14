// Package trace agent 运行观测领域
package trace

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// Trace 一次 agent 运行（领域结构体）
type Trace struct {
	ID         uint
	Agent      string
	SessionID  string
	APIKeyName string
	UserID     uint
	Model      string
	CWD        string
	Source     string
	Status     string
	Metadata   map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TraceEvent 运行内单个事件（领域结构体）
type TraceEvent struct {
	ID        uint
	TraceID   uint
	SessionID string
	Event     string
	TurnID    string
	Payload   []byte
	CreatedAt time.Time
}

// TraceRepository Trace 聚合仓储接口
type TraceRepository interface {
	// UpsertBySessionID 按 session_id 幂等写入/更新 trace；回填 ID
	UpsertBySessionID(ctx context.Context, t *Trace) (*Trace, error)
	// FindBySessionID 按 session_id 查询；未找到返回 (nil, nil)
	FindBySessionID(ctx context.Context, sessionID string) (*Trace, error)
	// FindByID 按 ID 查询；未找到返回 (nil, nil)
	FindByID(ctx context.Context, id uint) (*Trace, error)
	// MarkDone 将 trace 标记为 done
	MarkDone(ctx context.Context, sessionID string) error
	// InsertEvent 插入一条事件
	InsertEvent(ctx context.Context, e *TraceEvent) error
	// PaginateByOwners 按 owner 名称列表分页（admin 传空切片表示不过滤）
	PaginateByOwners(ctx context.Context, owners []string, param model.CommonParam) ([]*Trace, *model.PageInfo, error)
	// CountEvents 统计某 trace 的事件数
	CountEvents(ctx context.Context, traceID uint) (int64, error)
	// ListEvents 按 trace_id 分页列出事件（按 id 升序即时间线）
	ListEvents(ctx context.Context, traceID uint, param model.CommonParam) ([]*TraceEvent, *model.PageInfo, error)
}
