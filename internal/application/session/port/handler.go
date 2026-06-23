// Package port defines application-layer ports for session use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
)

// SessionSummaryView Session 列表单项视图（application 层只读投影）
type SessionSummaryView struct {
	ID           uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Summary      string
	Score        *int
	MessageCount int
	ToolCount    int
	Models       []string
}

// MessageView 消息视图
type MessageView struct {
	ID        uint
	Model     string
	Message   *vo.UnifiedMessage
	CreatedAt time.Time
}

// ToolView 工具视图
type ToolView struct {
	ID        uint
	Tool      *vo.UnifiedTool
	CreatedAt time.Time
}

// SessionDetailView Session 详情视图
type SessionDetailView struct {
	ID         uint
	APIKeyName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Metadata   map[string]string
	Score      *int
	ScoredAt   *time.Time
	MessageIDs []uint
	ToolIDs    []uint
	Messages   []*MessageView
	Tools      []*ToolView
}

// SessionMetaView session 元数据视图（含 IDs 数组，仅在 application 层内部使用）
type SessionMetaView struct {
	ID           uint
	APIKeyName   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Metadata     map[string]string
	Score        *int
	ScoredAt     *time.Time
	MessageIDs   []uint
	ToolIDs      []uint
	MessageCount int
	ToolCount    int
}

// ListSessionsByUserQuery 列出 session 查询参数
type ListSessionsByUserQuery struct {
	UserID    uint
	IsAdmin   bool
	Page      int
	PageSize  int
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
	Keyword   string
	Filter    string
}

// ListSessionsByUserHandler 列出 session handler 接口
type ListSessionsByUserHandler interface {
	Handle(ctx context.Context, q ListSessionsByUserQuery) ([]*SessionSummaryView, *model.PageInfo, error)
}

// GetSessionByUserQuery 获取 session 详情查询参数
type GetSessionByUserQuery struct {
	UserID             uint
	IsAdmin            bool
	SkipOwnershipCheck bool
	SessionID          uint
}

// GetSessionByUserHandler 获取 session 详情 handler 接口
type GetSessionByUserHandler interface {
	Handle(ctx context.Context, q GetSessionByUserQuery) (*SessionDetailView, error)
}

// GetSessionMetaByUserQuery 获取 session 元数据查询参数
type GetSessionMetaByUserQuery struct {
	UserID    uint
	IsAdmin   bool
	SessionID uint
}

// GetSessionMetaByUserHandler 元数据查询 handler 接口
type GetSessionMetaByUserHandler interface {
	Handle(ctx context.Context, q GetSessionMetaByUserQuery) (*SessionMetaView, error)
}

// ListSessionMessagesQuery 分页获取 session messages 查询参数
type ListSessionMessagesQuery struct {
	UserID    uint
	IsAdmin   bool
	SessionID uint
	Page      int
	PageSize  int
}

// ListSessionMessagesResult 分页结果
type ListSessionMessagesResult struct {
	Messages []*MessageView
	Total    int64
}

// ListSessionMessagesHandler 分页获取 messages handler 接口
type ListSessionMessagesHandler interface {
	Handle(ctx context.Context, q ListSessionMessagesQuery) (*ListSessionMessagesResult, error)
}

// ListSessionToolsQuery 分页获取 session tools 查询参数
type ListSessionToolsQuery struct {
	UserID    uint
	IsAdmin   bool
	SessionID uint
	Page      int
	PageSize  int
}

// ListSessionToolsResult 分页结果
type ListSessionToolsResult struct {
	Tools []*ToolView
	Total int64
}

// ListSessionToolsHandler 分页获取 tools handler 接口
type ListSessionToolsHandler interface {
	Handle(ctx context.Context, q ListSessionToolsQuery) (*ListSessionToolsResult, error)
}

// DeleteSessionCommand 删除 Session 命令
type DeleteSessionCommand struct {
	SessionIDs          []uint
	RequesterID         uint
	RequesterPermission enum.Permission
}

// DeleteSessionFailedItem 删除失败项
type DeleteSessionFailedItem struct {
	ID    uint
	Error string
}

// DeleteSessionResult 删除结果
type DeleteSessionResult struct {
	DeletedCount int
	Failures     []DeleteSessionFailedItem
}

// DeleteSessionHandler 删除命令处理器接口（支持单个和批量）
type DeleteSessionHandler interface {
	Handle(ctx context.Context, cmd DeleteSessionCommand) (*DeleteSessionResult, error)
}

// ScoreSessionCommand 评分命令
type ScoreSessionCommand struct {
	SessionID           uint
	Score               int
	RequesterID         uint
	RequesterPermission enum.Permission
}

// ScoreSessionHandler 评分命令处理器接口
type ScoreSessionHandler interface {
	Handle(ctx context.Context, cmd ScoreSessionCommand) (*time.Time, error)
}

// DeleteScoreSessionCommand 删除评分命令
type DeleteScoreSessionCommand struct {
	SessionID           uint
	RequesterID         uint
	RequesterPermission enum.Permission
}

// DeleteScoreSessionHandler 删除评分命令处理器接口
type DeleteScoreSessionHandler interface {
	Handle(ctx context.Context, cmd DeleteScoreSessionCommand) error
}

// ListSessionOptionQuery 会话筛选选项查询
type ListSessionOptionQuery struct {
	Field     string
	Keyword   string
	StartTime time.Time
	EndTime   time.Time
}

// ListSessionOptionHandler 会话筛选选项查询处理器
type ListSessionOptionHandler interface {
	Handle(ctx context.Context, q ListSessionOptionQuery) ([]string, error)
}

// CreateShareCommand 创建分享命令
type CreateShareCommand struct {
	RequesterID         uint
	RequesterPermission enum.Permission
	SessionID           uint
	ExpiresIn           string
	ExpiresAt           *int64
}

// CreateShareResult 创建分享结果
type CreateShareResult struct {
	ShareID   string
	ExpiresAt time.Time
}

// CreateShareHandler 创建分享命令处理器接口
type CreateShareHandler interface {
	Handle(ctx context.Context, cmd CreateShareCommand) (*CreateShareResult, error)
}
