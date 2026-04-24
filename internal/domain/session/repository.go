package session

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
)

// SessionRepository Session 聚合仓储接口
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
type SessionRepository interface {
	// Save 持久化聚合（首次 Save 后回填 ID）
	Save(ctx context.Context, session *aggregate.Session) error
	// FindByID 按 ID 查询；未找到返回 (nil, nil)
	FindByID(ctx context.Context, id uint) (*aggregate.Session, error)
	// Paginate 按 owner 分页查询（用于 List 接口）
	Paginate(ctx context.Context, owner string, param PageParam) ([]*aggregate.Session, *model.PageInfo, error)
	// Delete 软删除（标记 deleted_at）
	Delete(ctx context.Context, id uint) error
}

// PageParam 分页查询参数
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
type PageParam struct {
	Page     int
	PageSize int
}

// ==================== CQRS 读模型 ====================

// SessionSummaryProjection Session 列表只读投影
//
//	@author centonhuang
//	@update 2026-04-24 20:00:00
type SessionSummaryProjection struct {
	ID         uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Summary    string
	MessageIDs []uint
	ToolIDs    []uint
}

// MessageDetailProjection 消息详情只读投影
//
//	@author centonhuang
//	@update 2026-04-24 20:00:00
type MessageDetailProjection struct {
	ID        uint
	Model     string
	Message   *vo.UnifiedMessage
	CreatedAt time.Time
}

// ToolDetailProjection 工具详情只读投影
//
//	@author centonhuang
//	@update 2026-04-24 20:00:00
type ToolDetailProjection struct {
	ID        uint
	Tool      *vo.UnifiedTool
	CreatedAt time.Time
}

// SessionDetailProjection Session 详情只读投影
//
//	@author centonhuang
//	@update 2026-04-24 20:00:00
type SessionDetailProjection struct {
	ID         uint
	APIKeyName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Metadata   map[string]string
	MessageIDs []uint
	ToolIDs    []uint
	Messages   []*MessageDetailProjection
	Tools      []*ToolDetailProjection
}

// SessionReadRepository Session 只读查询仓储（CQRS 读模型）
//
// 与 SessionRepository 分离，避免写仓储受读模型查询的字段/投影需求污染。
//
//	@author centonhuang
//	@update 2026-04-24 20:00:00
type SessionReadRepository interface {
	// ListSessions 分页查询 Session 列表投影
	ListSessions(ctx context.Context, owner string, page, pageSize int) ([]*SessionSummaryProjection, *model.PageInfo, error)
	// GetSessionDetail 查询 Session 详情（含 Message/Tool 投影）
	GetSessionDetail(ctx context.Context, id uint, owner string) (*SessionDetailProjection, error)
}
