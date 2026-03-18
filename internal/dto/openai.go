package dto

import (
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// ==================== Union Type Structs for Strong Typing ====================

// MessageContent 消息内容（字符串或内容部分数组的联合类型）
//
//	用于 ChatCompletionMessageParam.Content 和 ChatCompletionPredictionContent.Content
//	当传入 JSON 是字符串时，存入 Text；当传入 JSON 是数组时，存入 Parts
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type MessageContent struct {
	Text  string                       `json:"-"`
	Parts []*ChatCompletionContentPart `json:"-"`
}

// UnmarshalJSON 自定义反序列化：区分字符串和数组
func (c *MessageContent) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.Text = s
		return nil
	}
	return sonic.Unmarshal(data, &c.Parts)
}

// MarshalJSON 自定义序列化：Parts 优先，否则输出字符串
func (c MessageContent) MarshalJSON() ([]byte, error) {
	if len(c.Parts) > 0 {
		return sonic.Marshal(c.Parts)
	}
	return sonic.Marshal(c.Text)
}

// Schema 实现 huma.SchemaProvider 接口，告诉 Huma 此类型接受字符串或 ContentPart 数组
func (c MessageContent) Schema(r huma.Registry) *huma.Schema {
	contentPartSchema := r.Schema(reflect.TypeFor[ChatCompletionContentPart](), true, "ChatCompletionContentPart")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "array", Items: contentPartSchema},
		},
	}
}

// ChatCompletionContentPart 内容部分联合结构体（按 Type 区分）
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type ChatCompletionContentPart struct {
	Type       string                           `json:"type"`
	Text       string                           `json:"text,omitempty"`
	Refusal    string                           `json:"refusal,omitempty"`
	ImageURL   *ChatCompletionImageURL          `json:"image_url,omitempty"`
	InputAudio *ChatCompletionInputAudioContent `json:"input_audio,omitempty"`
	File       *ChatCompletionFileContent       `json:"file,omitempty"`
}

// StopSequence 停止序列（字符串或字符串数组的联合类型）
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type StopSequence struct {
	Text  string   `json:"-"`
	Texts []string `json:"-"`
}

// UnmarshalJSON 自定义反序列化：区分字符串和字符串数组
func (s *StopSequence) UnmarshalJSON(data []byte) error {
	var text string
	if err := sonic.Unmarshal(data, &text); err == nil {
		s.Text = text
		return nil
	}
	return sonic.Unmarshal(data, &s.Texts)
}

// MarshalJSON 自定义序列化：Texts 优先，否则输出字符串
func (s StopSequence) MarshalJSON() ([]byte, error) {
	if len(s.Texts) > 0 {
		return sonic.Marshal(s.Texts)
	}
	return sonic.Marshal(s.Text)
}

// Schema 实现 huma.SchemaProvider 接口，告诉 Huma 此类型接受字符串或字符串数组
func (s StopSequence) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "array", Items: &huma.Schema{Type: "string"}},
		},
	}
}

// ChatCompletionToolChoiceParam 工具选择参数（字符串或对象的联合类型）
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type ChatCompletionToolChoiceParam struct {
	Mode  string                    `json:"-"` // "none" / "auto" / "required"
	Named *ChatCompletionToolChoice `json:"-"` // 具体的工具选择对象
}

// UnmarshalJSON 自定义反序列化：区分字符串模式和对象
func (tc *ChatCompletionToolChoiceParam) UnmarshalJSON(data []byte) error {
	var mode string
	if err := sonic.Unmarshal(data, &mode); err == nil {
		tc.Mode = mode
		return nil
	}
	tc.Named = &ChatCompletionToolChoice{}
	return sonic.Unmarshal(data, tc.Named)
}

// MarshalJSON 自定义序列化：Named 优先，否则输出字符串模式
func (tc ChatCompletionToolChoiceParam) MarshalJSON() ([]byte, error) {
	if tc.Named != nil {
		return sonic.Marshal(tc.Named)
	}
	return sonic.Marshal(tc.Mode)
}

// Schema 实现 huma.SchemaProvider 接口，告诉 Huma 此类型接受字符串或工具选择对象
func (tc ChatCompletionToolChoiceParam) Schema(r huma.Registry) *huma.Schema {
	toolChoiceSchema := r.Schema(reflect.TypeFor[ChatCompletionToolChoice](), true, "ChatCompletionToolChoice")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string", Enum: []any{"none", "auto", "required"}},
			toolChoiceSchema,
		},
	}
}

// VoiceParam 声音参数（字符串或对象 {id} 的联合类型）
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type VoiceParam struct {
	Name     string `json:"-"` // 内置声音名称如 "alloy"
	CustomID string `json:"-"` // 自定义声音 ID
}

// UnmarshalJSON 自定义反序列化：区分字符串和 {id} 对象
func (v *VoiceParam) UnmarshalJSON(data []byte) error {
	var name string
	if err := sonic.Unmarshal(data, &name); err == nil {
		v.Name = name
		return nil
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := sonic.Unmarshal(data, &obj); err != nil {
		return err
	}
	v.CustomID = obj.ID
	return nil
}

// MarshalJSON 自定义序列化：CustomID 优先输出对象，否则输出字符串
func (v VoiceParam) MarshalJSON() ([]byte, error) {
	if v.CustomID != "" {
		return sonic.Marshal(map[string]string{"id": v.CustomID})
	}
	return sonic.Marshal(v.Name)
}

// Schema 实现 huma.SchemaProvider 接口，告诉 Huma 此类型接受字符串或 {id} 对象
func (v VoiceParam) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{
				Type: "object",
				Properties: map[string]*huma.Schema{
					"id": {Type: "string"},
				},
				Required: []string{"id"},
			},
		},
	}
}

// ==================== Chat Completion Request DTOs ====================

// ChatCompletionReq Chat Completions请求
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionReq struct {
	Messages             []*ChatCompletionMessageParam    `json:"messages" doc:"对话消息列表"`
	Model                string                           `json:"model" doc:"模型ID"`
	Audio                *ChatCompletionAudioParam        `json:"audio,omitempty" doc:"音频输出参数"`
	FrequencyPenalty     *float64                         `json:"frequency_penalty,omitempty" doc:"频率惩罚(-2.0到2.0)"`
	LogitBias            map[string]int                   `json:"logit_bias,omitempty" doc:"token偏差映射"`
	Logprobs             *bool                            `json:"logprobs,omitempty" doc:"是否返回log概率"`
	MaxCompletionTokens  *int                             `json:"max_completion_tokens,omitempty" doc:"最大完成token数（包含推理token）"`
	MaxTokens            *int                             `json:"max_tokens,omitempty" doc:"最大token数（已废弃）"`
	Metadata             map[string]string                `json:"metadata,omitempty" doc:"元数据(最多16个键值对)"`
	Modalities           []*enum.ModalityType             `json:"modalities,omitempty" doc:"输出模态类型"`
	N                    *int                             `json:"n,omitempty" doc:"生成选择数量"`
	ParallelToolCalls    *bool                            `json:"parallel_tool_calls,omitempty" doc:"是否启用并行工具调用"`
	Prediction           *ChatCompletionPredictionContent `json:"prediction,omitempty" doc:"预测输出内容"`
	PresencePenalty      *float64                         `json:"presence_penalty,omitempty" doc:"存在惩罚(-2.0到2.0)"`
	PromptCacheKey       string                           `json:"prompt_cache_key,omitempty" doc:"提示缓存键"`
	PromptCacheRetention enum.PromptCacheRetention        `json:"prompt_cache_retention,omitempty" doc:"提示缓存保留策略"`
	ReasoningEffort      enum.ReasoningEffort             `json:"reasoning_effort,omitempty" doc:"推理努力级别"`
	ResponseFormat       *ResponseFormat                  `json:"response_format,omitempty" doc:"响应格式"`
	SafetyIdentifier     string                           `json:"safety_identifier,omitempty" doc:"安全标识符"`
	Seed                 *int                             `json:"seed,omitempty" doc:"随机种子"`
	ServiceTier          enum.ServiceTier                 `json:"service_tier,omitempty" doc:"服务层级"`
	Stop                 *StopSequence                    `json:"stop,omitempty" doc:"停止序列(字符串或字符串数组)"`
	Store                *bool                            `json:"store,omitempty" doc:"是否存储输出"`
	Stream               *bool                            `json:"stream,omitempty" doc:"是否流式响应"`
	StreamOptions        *ChatCompletionStreamOptions     `json:"stream_options,omitempty" doc:"流式选项"`
	Temperature          *float64                         `json:"temperature,omitempty" doc:"采样温度(0-2)"`
	ToolChoice           *ChatCompletionToolChoiceParam   `json:"tool_choice,omitempty" doc:"工具选择(字符串或对象)"`
	Tools                []ChatCompletionTool             `json:"tools,omitempty" doc:"可用工具列表"`
	TopLogprobs          *int                             `json:"top_logprobs,omitempty" doc:"返回的最可能token数量(0-20)"`
	TopP                 *float64                         `json:"top_p,omitempty" doc:"核采样概率质量"`
	User                 string                           `json:"user,omitempty" doc:"用户标识符(已废弃，使用safety_identifier或prompt_cache_key)"`
	Verbosity            enum.Verbosity                   `json:"verbosity,omitempty" doc:"响应详细程度"`
	WebSearchOptions     *WebSearchOptions                `json:"web_search_options,omitempty" doc:"网页搜索选项"`
}

// ChatCompletionMessageParam 聊天完成消息参数接口
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionMessageParam struct {
	Role             enum.Role       `json:"role" doc:"消息角色"`
	Content          *MessageContent `json:"content,omitempty" doc:"消息内容(字符串或数组)"`
	ReasoningContent string          `json:"reasoning_content,omitempty" doc:"推理内容"`
	Name             string          `json:"name,omitempty" doc:"参与者名称"`

	// 开发者/系统消息特有

	// 用户消息特有

	// 助手消息特有
	Audio     *ChatCompletionAudioReference    `json:"audio,omitempty" doc:"音频响应数据"`
	ToolCalls []*ChatCompletionMessageToolCall `json:"tool_calls,omitempty" doc:"工具调用列表"`
	Refusal   string                           `json:"refusal,omitempty" doc:"拒绝消息"`

	// 工具消息特有
	ToolCallID string `json:"tool_call_id,omitempty" doc:"工具调用ID"`

	// 额外字段
	Annotations []*MessageAnnotation `json:"annotations,omitempty" doc:"消息注解"`
}

// ChatCompletionAudioReference 音频引用
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionAudioReference struct {
	ID string `json:"id" doc:"音频响应的唯一标识符"`
}

// ChatCompletionImageURL 图片URL信息
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionImageURL struct {
	URL    string           `json:"url" doc:"图片URL或base64编码数据"`
	Detail enum.ImageDetail `json:"detail,omitempty" doc:"细节级别: auto/low/high"`
}

// ChatCompletionInputAudioContent 音频输入内容
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionInputAudioContent struct {
	Data   string                `json:"data" doc:"base64编码的音频数据"`
	Format enum.InputAudioFormat `json:"format" doc:"音频格式: wav/mp3"`
}

// ChatCompletionFileContent 文件内容
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionFileContent struct {
	FileData string `json:"file_data,omitempty" doc:"base64编码的文件数据"`
	FileID   string `json:"file_id,omitempty" doc:"上传文件的ID"`
	Filename string `json:"filename,omitempty" doc:"文件名"`
}

// ChatCompletionAudioParam 音频输出参数
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionAudioParam struct {
	Format enum.AudioFormat `json:"format" doc:"输出音频格式"`
	Voice  *VoiceParam      `json:"voice" doc:"声音(字符串或对象{id})"`
}

// ChatCompletionPredictionContent 预测输出内容
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionPredictionContent struct {
	Type    string          `json:"type" doc:"类型: content"`
	Content *MessageContent `json:"content" doc:"内容(字符串或数组)"`
}

// ResponseFormat 响应格式
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ResponseFormat struct {
	Type       enum.ResponseFormatType `json:"type" doc:"响应格式类型: text/json_object/json_schema"`
	JSONSchema *JSONSchemaFormat       `json:"json_schema,omitempty" doc:"JSON Schema格式配置"`
}

// JSONSchemaFormat JSON Schema格式配置
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type JSONSchemaFormat struct {
	Name        string         `json:"name" doc:"响应格式名称"`
	Description string         `json:"description,omitempty" doc:"响应格式描述"`
	Schema      map[string]any `json:"schema,omitempty" doc:"JSON Schema对象"`
	Strict      *bool          `json:"strict,omitempty" doc:"是否启用严格模式"`
}

// ChatCompletionStreamOptions 流式选项
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionStreamOptions struct {
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty" doc:"是否包含混淆数据"`
	IncludeUsage       *bool `json:"include_usage,omitempty" doc:"是否包含使用量统计"`
}

// ChatCompletionTool 聊天完成工具
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionTool struct {
	Type     enum.ToolType         `json:"type" doc:"工具类型: function/custom"`
	Function *FunctionDefinition   `json:"function,omitempty" doc:"函数定义"`
	Custom   *CustomToolDefinition `json:"custom,omitempty" doc:"自定义工具定义"`
}

// FunctionDefinition 函数定义
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type FunctionDefinition struct {
	Name        string              `json:"name" doc:"函数名称"`
	Description string              `json:"description,omitempty" doc:"函数描述"`
	Parameters  *JSONSchemaProperty `json:"parameters,omitempty" doc:"参数JSON Schema"`
	Strict      *bool               `json:"strict,omitempty" doc:"是否启用严格模式"`
}

// CustomToolDefinition 自定义工具定义
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type CustomToolDefinition struct {
	Name        string            `json:"name" doc:"自定义工具名称"`
	Description string            `json:"description,omitempty" doc:"自定义工具描述"`
	Format      *CustomToolFormat `json:"format,omitempty" doc:"输入格式"`
}

// CustomToolFormat 自定义工具格式
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type CustomToolFormat struct {
	Type    string          `json:"type" doc:"格式类型: text/grammar"`
	Grammar *GrammarContent `json:"grammar,omitempty" doc:"语法定义(当type=grammar时)"`
}

// GrammarContent 语法内容
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type GrammarContent struct {
	Definition string             `json:"definition" doc:"语法定义"`
	Syntax     enum.GrammarSyntax `json:"syntax" doc:"语法类型: lark/regex"`
}

// ChatCompletionToolChoice 工具选择（具体结构）
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionToolChoice struct {
	Type         string              `json:"type" doc:"工具类型: function/custom/allowed_tools"`
	Function     *ToolChoiceFunction `json:"function,omitempty" doc:"函数工具选择"`
	Custom       *ToolChoiceCustom   `json:"custom,omitempty" doc:"自定义工具选择"`
	AllowedTools *AllowedToolsConfig `json:"allowed_tools,omitempty" doc:"允许的工具配置"`
}

// ToolChoiceFunction 函数工具选择
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ToolChoiceFunction struct {
	Name string `json:"name" doc:"函数名称"`
}

// ToolChoiceCustom 自定义工具选择
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ToolChoiceCustom struct {
	Name string `json:"name" doc:"自定义工具名称"`
}

// AllowedToolsConfig 允许的工具配置
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type AllowedToolsConfig struct {
	Mode  string           `json:"mode" doc:"模式: auto/required"`
	Tools []map[string]any `json:"tools" doc:"允许的工具定义列表"`
}

// WebSearchOptions 网页搜索选项
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type WebSearchOptions struct {
	SearchContextSize enum.SearchContextSize `json:"search_context_size,omitempty" doc:"搜索上下文大小"`
	UserLocation      *UserLocation          `json:"user_location,omitempty" doc:"用户位置信息"`
}

// UserLocation 用户位置
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type UserLocation struct {
	Type        string               `json:"type" doc:"位置类型: approximate"`
	Approximate *ApproximateLocation `json:"approximate,omitempty" doc:"近似位置"`
}

// ApproximateLocation 近似位置
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ApproximateLocation struct {
	City     string `json:"city,omitempty" doc:"城市"`
	Country  string `json:"country,omitempty" doc:"国家(ISO 3166-1两位代码)"`
	Region   string `json:"region,omitempty" doc:"地区/州"`
	Timezone string `json:"timezone,omitempty" doc:"时区(IANA格式)"`
}

// ==================== Chat Completion Response DTOs ====================

// ChatCompletion Chat Completions响应
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletion struct {
	ID                string                  `json:"id" doc:"唯一标识符"`
	Choices           []*ChatCompletionChoice `json:"choices" doc:"完成选择列表"`
	Created           int64                   `json:"created" doc:"创建时间戳(Unix秒)"`
	Model             string                  `json:"model" doc:"使用的模型"`
	Object            string                  `json:"object" doc:"对象类型: chat.completion"`
	ServiceTier       enum.ServiceTier        `json:"service_tier,omitempty" doc:"服务层级"`
	SystemFingerprint string                  `json:"system_fingerprint,omitempty" doc:"系统指纹"`
	Usage             *CompletionUsage        `json:"usage,omitempty" doc:"使用量统计"`
}

// ChatCompletionChoice 完成选择
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionChoice struct {
	FinishReason enum.FinishReason           `json:"finish_reason" doc:"完成原因"`
	Index        int                         `json:"index" doc:"选择索引"`
	Logprobs     *Logprobs                   `json:"logprobs,omitempty" doc:"Log概率信息"`
	Message      *ChatCompletionMessageParam `json:"message" doc:"消息内容"`
}

// MessageAnnotation 消息注释
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type MessageAnnotation struct {
	Type        string       `json:"type" doc:"注释类型: url_citation"`
	URLCitation *URLCitation `json:"url_citation,omitempty" doc:"URL引用"`
}

// URLCitation URL引用
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type URLCitation struct {
	EndIndex   int    `json:"end_index" doc:"URL引用结束字符索引"`
	StartIndex int    `json:"start_index" doc:"URL引用开始字符索引"`
	Title      string `json:"title" doc:"网页资源标题"`
	URL        string `json:"url" doc:"网页资源URL"`
}

// ChatCompletionAudio 聊天完成音频响应
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionAudio struct {
	ID         string `json:"id" doc:"音频响应唯一标识符"`
	Data       string `json:"data" doc:"base64编码的音频数据"`
	ExpiresAt  int64  `json:"expires_at" doc:"过期时间戳(Unix秒)"`
	Transcript string `json:"transcript" doc:"音频转录文本"`
}

// ChatCompletionMessageToolCall 聊天完成消息工具调用
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionMessageToolCall struct {
	Index    int                                    `json:"index,omitempty" doc:"工具调用索引(流式delta中使用)"`
	ID       string                                 `json:"id,omitempty" doc:"工具调用ID"`
	Type     enum.ToolType                          `json:"type,omitempty" doc:"工具类型: function/custom"`
	Function *ChatCompletionMessageFunctionToolCall `json:"function,omitempty" doc:"函数工具调用"`
	Custom   *ChatCompletionMessageCustomToolCall   `json:"custom,omitempty" doc:"自定义工具调用"`
}

// ChatCompletionMessageFunctionToolCall 函数工具调用
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionMessageFunctionToolCall struct {
	Arguments string `json:"arguments" doc:"函数参数(JSON格式)"`
	Name      string `json:"name" doc:"函数名称"`
}

// ChatCompletionMessageCustomToolCall 自定义工具调用
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionMessageCustomToolCall struct {
	Input string `json:"input" doc:"自定义工具输入"`
	Name  string `json:"name" doc:"自定义工具名称"`
}

// Logprobs Log概率信息
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type Logprobs struct {
	Content []*ChatCompletionTokenLogprob `json:"content,omitempty" doc:"消息内容token的log概率"`
	Refusal []*ChatCompletionTokenLogprob `json:"refusal,omitempty" doc:"拒绝消息token的log概率"`
}

// ChatCompletionTokenLogprob Token Log概率
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionTokenLogprob struct {
	Token       string             `json:"token" doc:"token"`
	Bytes       []int              `json:"bytes,omitempty" doc:"UTF-8字节表示"`
	Logprob     float64            `json:"logprob" doc:"log概率"`
	TopLogprobs []*TopTokenLogprob `json:"top_logprobs,omitempty" doc:"最可能的token及其概率"`
}

// TopTokenLogprob 最可能的Token Log概率
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type TopTokenLogprob struct {
	Token   string  `json:"token" doc:"token"`
	Bytes   []int   `json:"bytes,omitempty" doc:"UTF-8字节表示"`
	Logprob float64 `json:"logprob" doc:"log概率"`
}

// CompletionUsage 完成使用量统计
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type CompletionUsage struct {
	CompletionTokens        int                      `json:"completion_tokens" doc:"生成的token数"`
	PromptTokens            int                      `json:"prompt_tokens" doc:"提示的token数"`
	TotalTokens             int                      `json:"total_tokens" doc:"总token数"`
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details,omitempty" doc:"完成token详细信息"`
	PromptTokensDetails     *PromptTokensDetails     `json:"prompt_tokens_details,omitempty" doc:"提示token详细信息"`
}

// CompletionTokensDetails 完成Token详细信息
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type CompletionTokensDetails struct {
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty" doc:"接受的预测token数"`
	AudioTokens              int `json:"audio_tokens,omitempty" doc:"音频token数"`
	ReasoningTokens          int `json:"reasoning_tokens,omitempty" doc:"推理token数"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty" doc:"拒绝的预测token数"`
}

// PromptTokensDetails 提示Token详细信息
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type PromptTokensDetails struct {
	AudioTokens  int `json:"audio_tokens,omitempty" doc:"音频token数"`
	CachedTokens int `json:"cached_tokens,omitempty" doc:"缓存token数"`
}

// ==================== Request/Response Wrapper DTOs ====================

// ChatCompletionRequest Chat Completions请求包装
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionRequest struct {
	Body *ChatCompletionReq `json:"body" doc:"请求体"`
}

// ListModelsResponseBody 模型列表响应体
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ListModelsRsp struct {
	Object string         `json:"object" doc:"对象类型: list"`
	Data   []*OpenAIModel `json:"data" doc:"模型列表"`
}

// OpenAIModel OpenAI模型
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type OpenAIModel struct {
	ID      string `json:"id" doc:"模型ID"`
	Created int64  `json:"created" doc:"创建时间戳"`
	Object  string `json:"object" doc:"对象类型: model"`
	OwnedBy string `json:"owned_by" doc:"所有者"`
}

// ==================== OpenAI Error DTOs ====================

// OpenAIError OpenAI错误响应
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type OpenAIError struct {
	Message string `json:"message" doc:"错误消息"`
	Type    string `json:"type" doc:"错误类型"`
	Code    string `json:"code" doc:"错误代码"`
}

// OpenAIErrorResponse OpenAI错误响应包装
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type OpenAIErrorResponse struct {
	Error *OpenAIError `json:"error" doc:"错误信息"`
}

// ==================== Chat Completion Chunk (Streaming) DTOs ====================

// ChatCompletionChunk 聊天完成流式块
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionChunk struct {
	ID                string                       `json:"id" doc:"唯一标识符"`
	Choices           []*ChatCompletionChunkChoice `json:"choices" doc:"选择列表"`
	Created           int64                        `json:"created" doc:"创建时间戳(Unix秒)"`
	Model             string                       `json:"model" doc:"使用的模型"`
	Object            string                       `json:"object" doc:"对象类型: chat.completion.chunk"`
	ServiceTier       enum.ServiceTier             `json:"service_tier,omitempty" doc:"服务层级"`
	SystemFingerprint string                       `json:"system_fingerprint,omitempty" doc:"系统指纹"`
	Usage             *CompletionUsage             `json:"usage,omitempty" doc:"使用量统计"`
}

// ChatCompletionChunkChoice 流式选择
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionChunkChoice struct {
	Delta        *ChatCompletionChunkDelta `json:"delta" doc:"增量内容"`
	FinishReason enum.FinishReason         `json:"finish_reason,omitempty" doc:"完成原因"`
	Index        int                       `json:"index" doc:"选择索引"`
	Logprobs     *Logprobs                 `json:"logprobs,omitempty" doc:"Log概率信息"`
}

// ChatCompletionChunkDelta 流式增量内容
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type ChatCompletionChunkDelta struct {
	Content          string                           `json:"content,omitempty" doc:"内容增量"`
	ReasoningContent string                           `json:"reasoning_content,omitempty" doc:"推理内容增量"`
	Refusal          string                           `json:"refusal,omitempty" doc:"拒绝消息增量"`
	Role             enum.Role                        `json:"role,omitempty" doc:"角色"`
	ToolCalls        []*ChatCompletionMessageToolCall `json:"tool_calls,omitempty" doc:"工具调用增量"`
}
