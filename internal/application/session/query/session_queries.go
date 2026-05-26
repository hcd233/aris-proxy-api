// Package query Session 域只读查询处理器
//
// 遵循 CQRS 读模型原则：通过 SessionReadRepository 接口获取只读投影，
// 并在 application 层映射为内部视图类型，保持层次隔离。
package query

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// SessionSummaryView Session 列表单项视图（application 层只读投影）
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type SessionSummaryView struct {
	ID         uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Summary    string
	MessageIDs []uint
	ToolIDs    []uint
}

// MessageView 消息视图
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type MessageView struct {
	ID        uint
	Model     string
	Message   *vo.UnifiedMessage
	CreatedAt time.Time
}

// ToolView 工具视图
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type ToolView struct {
	ID        uint
	Tool      *vo.UnifiedTool
	CreatedAt time.Time
}

// SessionDetailView Session 详情视图
//
//	@author centonhuang
//	@update 2026-04-23 11:00:00
type SessionDetailView struct {
	ID         uint
	APIKeyName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Metadata   map[string]string
	MessageIDs []uint
	ToolIDs    []uint
	Messages   []*MessageView
	Tools      []*ToolView
}


