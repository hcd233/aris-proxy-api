package transport

import (
	"github.com/bytedance/sonic"
	"github.com/samber/lo"
)

// upstreamRequestSummary 从上游请求体中提取的关键诊断字段。
// 避免在 Info 级别打印完整请求体，同时保留足够的诊断信息。
type upstreamRequestSummary struct {
	Model               string   `json:"model"`
	ToolNames           []string `json:"toolNames"`
	MessageCount        int      `json:"messageCount"`
	MaxTokens           int      `json:"maxTokens,omitempty"`
	MaxCompletionTokens int      `json:"maxCompletionTokens,omitempty"`
	Temperature         float64  `json:"temperature,omitempty"`
	Stream              bool     `json:"stream"`
}

type upstreamRequestBodyFields struct {
	Model               string              `json:"model"`
	MaxTokens           int                 `json:"max_tokens"`
	MaxCompletionTokens int                 `json:"max_completion_tokens"`
	Temperature         float64             `json:"temperature"`
	Stream              bool                `json:"stream"`
	Messages            []struct{}          `json:"messages"`
	Tools               []toolNameExtractor `json:"tools"`
}

// toolNameExtractor 同时兼容 OpenAI（tools[].function.name）和 Anthropic（tools[].name）的工具名称提取。
type toolNameExtractor struct {
	Name     string `json:"name"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

// parseUpstreamRequestSummary 从原始请求体中轻量解析关键诊断字段。
// 解析失败时返回零值 summary，不阻断日志记录。
func parseUpstreamRequestSummary(body []byte) upstreamRequestSummary {
	var raw upstreamRequestBodyFields
	if err := sonic.Unmarshal(body, &raw); err != nil {
		return upstreamRequestSummary{}
	}

	toolNames := lo.FilterMap(raw.Tools, func(t toolNameExtractor, _ int) (string, bool) {
		if t.Function.Name != "" {
			return t.Function.Name, true
		}
		if t.Name != "" {
			return t.Name, true
		}
		return "", false
	})

	return upstreamRequestSummary{
		Model:               raw.Model,
		ToolNames:           toolNames,
		MessageCount:        len(raw.Messages),
		MaxTokens:           raw.MaxTokens,
		MaxCompletionTokens: raw.MaxCompletionTokens,
		Temperature:         raw.Temperature,
		Stream:              raw.Stream,
	}
}
