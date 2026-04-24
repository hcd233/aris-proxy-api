// Package vo Conversation 域值对象
package vo

import (
	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// ==================== Unified Content Types ====================

// UnifiedContent 统一消息内容（替代 any），纯文本时仅使用 Text，多部分内容时使用 Parts
//
//	@author centonhuang
//	@update 2026-04-22 14:10:00
type UnifiedContent struct {
	Text  string                `json:"-"`
	Parts []*UnifiedContentPart `json:"-"`
}

// UnmarshalJSON 自定义反序列化：兼容旧数据（string / array / object）
//
//	@receiver c *UnifiedContent
//	@param data []byte
//	@return error
//	@author centonhuang
//	@update 2026-04-22 14:10:00
func (c *UnifiedContent) UnmarshalJSON(data []byte) error {
	// 1. 尝试作为字符串
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.Text = s
		return nil
	}
	// 2. 尝试作为 Parts 数组
	return sonic.Unmarshal(data, &c.Parts)
}

// MarshalJSON 自定义序列化：Parts 优先，否则输出字符串
//
//	@receiver c UnifiedContent
//	@return []byte
//	@return error
//	@author centonhuang
//	@update 2026-04-22 14:10:00
func (c UnifiedContent) MarshalJSON() ([]byte, error) {
	if len(c.Parts) > 0 {
		return sonic.Marshal(c.Parts)
	}
	return sonic.Marshal(c.Text)
}

// Schema 实现 huma.SchemaProvider：UnifiedContent 是 string|array 联合类型
//
//	@receiver UnifiedContent
//	@param r huma.Registry
//	@return *huma.Schema
//	@author centonhuang
//	@update 2026-04-22 14:10:00
func (UnifiedContent) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{OneOf: []*huma.Schema{
		{Type: "string"},
		{Type: "array", Items: &huma.Schema{Type: "object"}},
	}}
}

// UnifiedContentPart 统一内容部分
//
//	@author centonhuang
//	@update 2026-04-22 14:10:00
type UnifiedContentPart struct {
	Type        string `json:"type"`                   // text/image_url/input_audio/file/refusal
	Text        string `json:"text,omitempty"`         // type=text 或 type=refusal
	ImageURL    string `json:"image_url,omitempty"`    // type=image_url: URL 或 base64
	ImageDetail string `json:"image_detail,omitempty"` // type=image_url: 细节级别
	AudioData   string `json:"audio_data,omitempty"`   // type=input_audio
	AudioFormat string `json:"audio_format,omitempty"` // type=input_audio
	FileData    string `json:"file_data,omitempty"`    // type=file
	FileID      string `json:"file_id,omitempty"`      // type=file
	Filename    string `json:"filename,omitempty"`     // type=file
}

// ==================== Unified Message ====================

// UnifiedMessage 统一消息格式，用于跨 Provider 的消息存储
//
//	@author centonhuang
//	@update 2026-04-22 14:10:00
type UnifiedMessage struct {
	Role             enum.Role          `json:"role" doc:"消息角色"`
	Content          *UnifiedContent    `json:"content,omitempty" doc:"消息内容"`
	ReasoningContent string             `json:"reasoning_content,omitempty" doc:"推理/思考内容"`
	Name             string             `json:"name,omitempty" doc:"参与者名称"`
	ToolCalls        []*UnifiedToolCall `json:"tool_calls,omitempty" doc:"工具调用列表"`
	ToolCallID       string             `json:"tool_call_id,omitempty" doc:"工具调用ID(工具结果消息)"`
	Refusal          string             `json:"refusal,omitempty" doc:"拒绝消息"`
}

// UnifiedToolCall 统一工具调用
//
//	@author centonhuang
//	@update 2026-04-22 14:10:00
type UnifiedToolCall struct {
	ID        string `json:"id,omitempty" doc:"工具调用ID"`
	Name      string `json:"name" doc:"工具/函数名称"`
	Arguments string `json:"arguments" doc:"工具参数(JSON字符串)"`
}
