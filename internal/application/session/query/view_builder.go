// Package query Session 域查询侧视图构造辅助函数
//
// 把 dbmodel.Message/Tool 按给定 ID 顺序映射为 application 内部视图类型。
package query

import (
	"github.com/samber/lo"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// BuildOrderedMessages 按指定 ID 顺序构建消息视图列表
//
//   - 使用 lo.SliceToMap 建立 ID -> Message 映射
//
//   - 按 ids 顺序遍历，未在 messages 中的 ID 跳过（无占位）
//
//     @param ids []uint 有序 ID 列表
//     @param messages []*dbmodel.Message 消息列表
//     @return []*MessageView
//     @author centonhuang
//     @update 2026-04-23 11:00:00
func BuildOrderedMessages(ids []uint, messages []*dbmodel.Message) []*MessageView {
	messageMap := lo.SliceToMap(messages, func(m *dbmodel.Message) (uint, *dbmodel.Message) {
		return m.ID, m
	})
	items := make([]*MessageView, 0, len(ids))
	for _, id := range ids {
		msg, ok := messageMap[id]
		if !ok {
			continue
		}
		items = append(items, &MessageView{
			ID:        msg.ID,
			Model:     msg.Model,
			Message:   msg.Message,
			CreatedAt: msg.CreatedAt,
		})
	}
	return items
}

// BuildOrderedTools 按指定 ID 顺序构建工具视图列表
//
//	@param ids []uint 有序 ID 列表
//	@param tools []*dbmodel.Tool 工具列表
//	@return []*ToolView
//	@author centonhuang
//	@update 2026-04-23 11:00:00
func BuildOrderedTools(ids []uint, tools []*dbmodel.Tool) []*ToolView {
	toolMap := lo.SliceToMap(tools, func(t *dbmodel.Tool) (uint, *dbmodel.Tool) {
		return t.ID, t
	})
	items := make([]*ToolView, 0, len(ids))
	for _, id := range ids {
		tool, ok := toolMap[id]
		if !ok {
			continue
		}
		items = append(items, &ToolView{
			ID:        tool.ID,
			Tool:      tool.Tool,
			CreatedAt: tool.CreatedAt,
		})
	}
	return items
}
