package util

import (
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// ToolSchemaMap 工具名 → 参数 Schema 的映射表，用于 schema-aware checksum 计算
//
// Deprecated: 请使用 internal/domain/conversation/vo.ToolSchemaMap
type ToolSchemaMap = vo.ToolSchemaMap

// ComputeMessageChecksum 计算统一消息校验和，委托到 domain/conversation/vo.ComputeMessageChecksum
//
//	@param msg *dto.UnifiedMessage
//	@param toolSchemas ToolSchemaMap
//	@return string
//	@author centonhuang
//	@update 2026-04-22 14:15:00
func ComputeMessageChecksum(msg *dto.UnifiedMessage, toolSchemas ToolSchemaMap) string {
	return vo.ComputeMessageChecksum(msg, toolSchemas)
}

// ComputeToolChecksum 计算工具校验和，委托到 domain/conversation/vo.ComputeToolChecksum
//
//	@param tool *dto.UnifiedTool
//	@return string
//	@author centonhuang
//	@update 2026-04-22 14:15:00
func ComputeToolChecksum(tool *dto.UnifiedTool) string {
	return vo.ComputeToolChecksum(tool)
}
