package vo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/encoder"
	"github.com/samber/lo"

	commonvo "github.com/hcd233/aris-proxy-api/internal/domain/common/vo"
)

// ToolSchemaMap 工具名 → 参数 Schema 的映射表，用于 schema-aware checksum 计算
//
//	@author centonhuang
//	@update 2026-04-22 14:15:00
type ToolSchemaMap map[string]*commonvo.JSONSchemaProperty

// ComputeMessageChecksum 计算统一消息校验和
//
// 对 UnifiedMessage 做规范化处理，确保语义相同但表示不同的消息产生相同的 checksum：
//
//   - 清除 ToolCalls 中的 ID（上游分配的标识符，不影响消息语义，
//     且同一条消息在流式和非流式路径中可能产生不同的 ID 格式）
//
//   - 保留 ToolCallID（工具结果消息中引用的调用 ID，用于区分不同的工具调用结果）
//
//   - 移除 ToolCall arguments 中等于 schema default 的非 required 字段（需提供 toolSchemas）
//
//   - 序列化规范化后的结构体，计算 SHA256
//
//     @param msg *UnifiedMessage
//     @param toolSchemas ToolSchemaMap 工具 Schema 映射表（可为 nil，nil 时退化为无 schema 模式）
//     @return string
//     @author centonhuang
//     @update 2026-04-22 14:15:00
func ComputeMessageChecksum(msg *UnifiedMessage, toolSchemas ToolSchemaMap) string {
	normalized := *msg

	if len(normalized.ToolCalls) > 0 {
		cleanedCalls := make([]*UnifiedToolCall, len(normalized.ToolCalls))
		for i, tc := range normalized.ToolCalls {
			var schema *commonvo.JSONSchemaProperty
			if toolSchemas != nil {
				schema = toolSchemas[tc.Name]
			}
			cleanedCalls[i] = &UnifiedToolCall{
				Name:      tc.Name,
				Arguments: normalizeArgumentsWithSchema(tc.Arguments, schema),
			}
		}
		normalized.ToolCalls = cleanedCalls
	}

	hash := sha256.Sum256(lo.Must1(encoder.Encode(normalized, encoder.SortMapKeys)))
	return hex.EncodeToString(hash[:])
}

// normalizeArgumentsWithSchema 根据 tool schema 规范化 arguments JSON 字符串
//
// 移除 arguments 中满足以下全部条件的字段：
//   - 不在 schema.Required 中
//   - schema.Properties[key] 存在 Default 值
//   - arguments 中该字段的值等于 Default 值
//
// schema 为 nil 时仅做键排序规范化，行为等价于旧的 normalizeJSONString。
//
// 容错策略（防止外部上游 tool_call arguments 异常 JSON 触发 panic）：
//
//   - args 解析失败 → 直接回退返回原始字符串（checksum 仍稳定，仅失去规范化能力）
//
//   - schema.Default 解析失败 → 跳过该字段的 default 比对（不影响其他字段）
//
//   - 最终 encode 失败 → 回退原始字符串
//
//     @param args string arguments JSON 字符串
//     @param schema *commonvo.JSONSchemaProperty 工具参数 schema（可为 nil）
//     @return string 规范化后的 JSON 字符串
//     @author centonhuang
//     @update 2026-04-23 11:00:00
func normalizeArgumentsWithSchema(args string, schema *commonvo.JSONSchemaProperty) string {
	var obj map[string]any
	if err := sonic.UnmarshalString(args, &obj); err != nil {
		return args
	}

	if schema != nil && schema.Properties != nil {
		requiredSet := lo.SliceToMap(schema.Required, func(r string) (string, bool) {
			return r, true
		})

		for key, val := range obj {
			if requiredSet[key] {
				continue
			}
			prop, hasProp := schema.Properties[key]
			if !hasProp || prop.Default == nil {
				continue
			}
			var defaultVal any
			if err := sonic.Unmarshal(prop.Default, &defaultVal); err != nil {
				continue
			}
			if jsonEqual(val, defaultVal) {
				delete(obj, key)
			}
		}
	}

	encoded, err := encoder.Encode(obj, encoder.SortMapKeys)
	if err != nil {
		return args
	}
	return string(encoded)
}

// jsonEqual 比较两个通过 JSON 反序列化得到的值是否语义相等
//
//	@param a any
//	@param b any
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 14:15:00
func jsonEqual(a, b any) bool {
	aBytes := lo.Must1(encoder.Encode(a, encoder.SortMapKeys))
	bBytes := lo.Must1(encoder.Encode(b, encoder.SortMapKeys))
	return bytes.Equal(aBytes, bBytes)
}

// toolChecksumWire is the JSON-shaped payload for stable tool checksum hashing.
type toolChecksumWire struct {
	Name       string                       `json:"name"`
	Parameters *commonvo.JSONSchemaProperty `json:"parameters"`
}

// ComputeToolChecksum 计算工具校验和，基于工具名和完整参数 Schema
//
// 使用 encoder.Encode + SortMapKeys 对 Name 和 Parameters 进行规范化序列化，
// 确保 map key 顺序稳定，完整捕获所有层级参数结构的差异。
//
//	@param tool *UnifiedTool
//	@return string
//	@author centonhuang
//	@update 2026-04-22 14:15:00
func ComputeToolChecksum(tool *UnifiedTool) string {
	data := toolChecksumWire{
		Name:       tool.Name,
		Parameters: tool.Parameters,
	}

	hash := sha256.Sum256(lo.Must1(encoder.Encode(data, encoder.SortMapKeys)))
	return hex.EncodeToString(hash[:])
}
