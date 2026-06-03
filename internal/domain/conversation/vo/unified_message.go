package vo

import (
	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// UnifiedContent 统一消息内容（替代 any），纯文本时仅使用 Text，多部分内容时使用 Parts
type UnifiedContent struct {
	Text  string                `json:"-"`
	Parts []*UnifiedContentPart `json:"-"`
}

func (c *UnifiedContent) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.Text = s
		return nil
	}
	return sonic.Unmarshal(data, &c.Parts)
}

func (c UnifiedContent) MarshalJSON() ([]byte, error) {
	if len(c.Parts) > 0 {
		return sonic.Marshal(c.Parts)
	}
	return sonic.Marshal(c.Text)
}

func (UnifiedContent) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{OneOf: []*huma.Schema{
		{Type: enum.JSONSchemaTypeString},
		{Type: enum.JSONSchemaTypeArray, Items: &huma.Schema{Type: enum.JSONSchemaTypeObject}},
	}}
}

// UnifiedContentPart 统一内容部分
type UnifiedContentPart struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	ImageDetail string `json:"image_detail,omitempty"`
	AudioData   string `json:"audio_data,omitempty"`
	AudioFormat string `json:"audio_format,omitempty"`
	FileData    string `json:"file_data,omitempty"`
	FileID      string `json:"file_id,omitempty"`
	Filename    string `json:"filename,omitempty"`
}

// UnifiedMessage 统一消息格式，用于跨 Provider 的消息存储
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
type UnifiedToolCall struct {
	ID        string `json:"id,omitempty" doc:"工具调用ID"`
	Name      string `json:"name" doc:"工具/函数名称"`
	Arguments string `json:"arguments" doc:"工具参数(JSON字符串)"`
}
