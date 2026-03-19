package util

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/encoder"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// ComputeMessageChecksum 计算统一消息校验和
//
// 对 UnifiedMessage 做规范化处理，确保语义相同但表示不同的消息产生相同的 checksum：
//
//   - 清除 ToolCalls 中的 ID（上游分配的标识符，不影响消息语义，
//     且同一条消息在流式和非流式路径中可能产生不同的 ID 格式）
//
//   - 清除 ToolCallID（工具结果消息中引用的调用 ID，同理不影响语义）
//
//   - 序列化规范化后的结构体，计算 SHA256
//
//     @param msg *dto.UnifiedMessage
//     @return string
//     @author centonhuang
//     @update 2026-03-18 10:00:00
func ComputeMessageChecksum(msg *dto.UnifiedMessage) string {
	// 深拷贝以避免修改原始消息
	normalized := *msg

	// 清除易变的标识符字段
	normalized.ToolCallID = ""

	if len(normalized.ToolCalls) > 0 {
		cleanedCalls := make([]*dto.UnifiedToolCall, len(normalized.ToolCalls))
		for i, tc := range normalized.ToolCalls {
			cleanedCalls[i] = &dto.UnifiedToolCall{
				Name:      tc.Name,
				Arguments: normalizeJSONString(tc.Arguments),
			}
		}
		normalized.ToolCalls = cleanedCalls
	}

	hash := sha256.Sum256(lo.Must1(encoder.Encode(normalized, encoder.SortMapKeys)))
	return hex.EncodeToString(hash[:])
}

// normalizeJSONString 将 JSON 字符串反序列化后重新序列化为紧凑格式（键排序），消除键顺序和空格等格式差异
//
//	@param s string JSON字符串
//	@return string 规范化后的JSON字符串，如果解析失败则返回原始字符串
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func normalizeJSONString(s string) string {
	var obj map[string]any
	lo.Must0(sonic.UnmarshalString(s, &obj))
	return string(lo.Must1(encoder.Encode(obj, encoder.SortMapKeys)))
}

// ComputeToolChecksum 计算工具校验和，基于工具名和完整参数 Schema
//
// 使用 encoder.Encode + SortMapKeys 对 Name 和 Parameters 进行规范化序列化，
// 确保 map key 顺序稳定，完整捕获所有层级参数结构的差异。
//
//	@param tool *UnifiedTool
//	@return string
//	@author centonhuang
//	@update 2026-03-19 10:00:00
func ComputeToolChecksum(tool *dto.UnifiedTool) string {
	data := struct {
		Name       string                  `json:"name"`
		Parameters *dto.JSONSchemaProperty `json:"parameters"`
	}{
		Name:       tool.Name,
		Parameters: tool.Parameters,
	}

	hash := sha256.Sum256(lo.Must1(encoder.Encode(data, encoder.SortMapKeys)))
	return hex.EncodeToString(hash[:])
}
