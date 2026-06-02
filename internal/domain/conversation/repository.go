// Package conversation Conversation 域根（仓储接口）
package conversation

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// MessageRepository Message 聚合仓储接口
//
// 支持批量去重存储：BatchSaveDedup 接受一组 Message，按 Checksum 查找已
// 存在的条目，只持久化新内容；返回与输入顺序对齐的 ID 列表（含复用的 ID）。
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
type MessageRepository interface {
	// BatchSaveDedup 批量去重保存消息，返回与输入顺序对齐的 ID 列表
	BatchSaveDedup(ctx context.Context, messages []*aggregate.Message) ([]uint, error)
	// FindByIDs 按 ID 列表批量查询（用于 Session 详情页）
	FindByIDs(ctx context.Context, ids []uint) ([]*aggregate.Message, error)
}

// ThinkExtractMessage 推理内容提取任务的消息候选。
type ThinkExtractMessage struct {
	ID      uint
	Message *vo.UnifiedMessage
}

// ThinkExtractRepository 为定时任务暴露消息提取所需的最小仓储端口。
type ThinkExtractRepository interface {
	FindThinkExtractCandidates(ctx context.Context, afterID uint, startTime, endTime time.Time, limit int) ([]*ThinkExtractMessage, error)
	UpdateMessageContent(ctx context.Context, id uint, message *vo.UnifiedMessage) error
}

// ToolRepository Tool 聚合仓储接口
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
type ToolRepository interface {
	// BatchSaveDedup 批量去重保存工具
	BatchSaveDedup(ctx context.Context, tools []*aggregate.Tool) ([]uint, error)
	// FindByIDs 按 ID 列表批量查询
	FindByIDs(ctx context.Context, ids []uint) ([]*aggregate.Tool, error)
}
