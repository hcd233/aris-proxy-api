// Package conversation Conversation 域根（仓储接口）
package conversation

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/aggregate"
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
