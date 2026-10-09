package openai

import (
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// ==================== Decision API (/decisions) ====================
//
// 参考 docs/openai/create_decisions.md。
// 请求完整建模；响应只建模审计/存储所需子集（model/answers/usage），
// 响应体本身由 usecase 原样透传（与 /responses unary 同范式）。

// OpenAICreateDecisionRequest 创建 Decision 请求
type OpenAICreateDecisionRequest struct {
	Body *OpenAICreateDecisionReq `json:"body" doc:"请求体"`
}

// OpenAICreateDecisionReq 创建 Decision 请求体
type OpenAICreateDecisionReq struct {
	Model            string              `json:"model" required:"true" doc:"模型别名"`
	Input            DecisionInput       `json:"input" required:"true" doc:"待评估的文本或 user 消息数组"`
	Questions        []*DecisionQuestion `json:"questions" required:"true" minItems:"1" doc:"问题列表"`
	SafetyIdentifier *string             `json:"safety_identifier,omitempty" doc:"调用方提供的终端用户标识"`
}

// DecisionInput input 联合类型：string | []DecisionInputMessage
type DecisionInput struct {
	Text     *string                 `json:"-"`
	Messages []*DecisionInputMessage `json:"-"`
}

// UnmarshalJSON 先按字符串解析，失败则按消息数组解析
func (d *DecisionInput) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		d.Text = &s
		return nil
	}
	return sonic.Unmarshal(data, &d.Messages)
}

// MarshalJSON 按已填充的分支输出
func (d DecisionInput) MarshalJSON() ([]byte, error) {
	if d.Text != nil {
		return sonic.Marshal(*d.Text)
	}
	if d.Messages != nil {
		return sonic.Marshal(d.Messages)
	}
	return []byte(constant.NullJSONLiteral), nil
}

// Schema 声明为 string 或 DecisionInputMessage 数组
func (DecisionInput) Schema(r huma.Registry) *huma.Schema {
	msgSchema := r.Schema(reflect.TypeFor[DecisionInputMessage](), true, "DecisionInputMessage")
	return &huma.Schema{OneOf: []*huma.Schema{
		{Type: enum.JSONSchemaTypeString},
		{Type: enum.JSONSchemaTypeArray, Items: msgSchema},
	}}
}

// DecisionInputMessage 待评估的 user 消息（仅支持 role=user）
type DecisionInputMessage struct {
	Role    string                      `json:"role" doc:"固定 user"`
	Type    *string                     `json:"type,omitempty" doc:"固定 message"`
	Content ResponseInputMessageContent `json:"content" doc:"文本或文本+图片 parts"`
}

// DecisionQuestion 决策问题（predicate / choice / score）
type DecisionQuestion struct {
	Type         string                `json:"type" doc:"predicate/choice/score"`
	Name         *string               `json:"name,omitempty" doc:"问题名(可选)"`
	Instructions string                `json:"instructions" required:"true" doc:"问题指令"`
	Choices      []*DecisionChoice     `json:"choices,omitempty" doc:"type=choice 时的 2~255 个选项"`
	Levels       []*DecisionScoreLevel `json:"levels,omitempty" doc:"type=score 时的有序等级"`
}

// DecisionChoice choice 型问题的选项
type DecisionChoice struct {
	Value       DecisionChoiceValue `json:"value" doc:"选项值(string|bool)"`
	Description *string             `json:"description,omitempty" doc:"选项描述"`
}

// DecisionChoiceValue choice.value 联合类型：string | bool
//
// 同文本的 string 与 bool 是不同选项，故必须区分。
type DecisionChoiceValue struct {
	StringValue  *string `json:"-"`
	BooleanValue *bool   `json:"-"`
}

// UnmarshalJSON 先按 bool 解析（否则 "true" 会被当成字符串），失败再按 string
func (v *DecisionChoiceValue) UnmarshalJSON(data []byte) error {
	var b bool
	if err := sonic.Unmarshal(data, &b); err == nil {
		v.BooleanValue = &b
		return nil
	}
	var s string
	if err := sonic.Unmarshal(data, &s); err != nil {
		return err
	}
	v.StringValue = &s
	return nil
}

// MarshalJSON 按已填充的分支输出
func (v DecisionChoiceValue) MarshalJSON() ([]byte, error) {
	if v.BooleanValue != nil {
		return sonic.Marshal(*v.BooleanValue)
	}
	if v.StringValue != nil {
		return sonic.Marshal(*v.StringValue)
	}
	return []byte(constant.NullJSONLiteral), nil
}

// Schema 声明为 boolean 或 string
func (DecisionChoiceValue) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{OneOf: []*huma.Schema{
		{Type: enum.JSONSchemaTypeBoolean},
		{Type: enum.JSONSchemaTypeString},
	}}
}

// DecisionScoreLevel score 型问题的有序等级
type DecisionScoreLevel struct {
	Label       string  `json:"label" doc:"等级标签"`
	Description *string `json:"description,omitempty" doc:"等级描述"`
}

// ==================== Decision 响应（审计/存储子集） ====================

// OpenAIDecisionRsp Decision 响应子集
type OpenAIDecisionRsp struct {
	Model   string                 `json:"model" doc:"上游返回的模型名"`
	Answers sonic.NoCopyRawMessage `json:"answers" doc:"答案数组(原样保留)"`
	Usage   *OpenAIDecisionUsage   `json:"usage,omitempty" doc:"使用量"`
}

// OpenAIDecisionUsage Decision 使用量
type OpenAIDecisionUsage struct {
	InputTokens         int                          `json:"input_tokens" doc:"输入 token"`
	OutputTokens        int                          `json:"output_tokens" doc:"输出 token"`
	TotalTokens         int                          `json:"total_tokens" doc:"总 token"`
	InputTokensDetails  *DecisionInputTokensDetails  `json:"input_tokens_details,omitempty" doc:"输入明细"`
	OutputTokensDetails *DecisionOutputTokensDetails `json:"output_tokens_details,omitempty" doc:"输出明细"`
}

// DecisionInputTokensDetails 输入明细（两维均为 input_tokens 的子集）
type DecisionInputTokensDetails struct {
	CachedTokens     int `json:"cached_tokens" doc:"缓存命中 token"`
	CacheWriteTokens int `json:"cache_write_tokens" doc:"缓存写入 token"`
}

// DecisionOutputTokensDetails 输出明细
type DecisionOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens" doc:"推理 token"`
}

// InputOutputTokens 返回 input_tokens + output_tokens（原始口径，供限流上报）
func (u *OpenAIDecisionUsage) InputOutputTokens() int64 {
	if u == nil {
		return 0
	}
	return int64(u.InputTokens + u.OutputTokens)
}
