package dto

import (
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
)

// ==================== Response API Input Item Types ====================
//
// 参考 docs/openai/create_response.md 第 77-2896 行 InputItemList 定义。
// 28 种 input item 全部通过 `type` 字段区分，采用扁平结构体 + 按需子结构体方式。
// 相关 type 字面量常量见 internal/enum/openai_response.go。
//
//	@author centonhuang
//	@update 2026-04-17 17:00:00

// ==================== Content / Annotation / Logprob ====================

// ResponseInputContent Response API 消息内容块（联合 ResponseInputText/Image/File/
// ResponseOutputText/Refusal/SummaryText/ReasoningText 等形态）
type ResponseInputContent struct {
	Type string `json:"type" doc:"内容类型"`

	// input_text/output_text/reasoning_text/summary_text 共用
	Text string `json:"text,omitempty" doc:"文本内容"`

	// input_image/input_file 共用
	Detail string `json:"detail,omitempty" doc:"精度: low/high/auto/original"`

	// input_image 专用
	ImageURL string `json:"image_url,omitempty" doc:"图像 URL 或 Data URL"`

	// input_file 专用
	FileData string `json:"file_data,omitempty" doc:"文件内容(base64)"`
	FileURL  string `json:"file_url,omitempty" doc:"文件 URL"`
	Filename string `json:"filename,omitempty" doc:"文件名"`

	// input_image/input_file 共用
	FileID string `json:"file_id,omitempty" doc:"文件 ID"`

	// output_text 专用
	Annotations []*ResponseOutputTextAnnotation `json:"annotations,omitempty" doc:"引用标注列表"`
	Logprobs    []*ResponseOutputTextLogprob    `json:"logprobs,omitempty" doc:"token 对数概率"`

	// refusal 专用
	Refusal string `json:"refusal,omitempty" doc:"拒答原因"`
}

// ResponseOutputTextAnnotation 输出文本引用（file_citation/url_citation/
// container_file_citation/file_path 四类）
type ResponseOutputTextAnnotation struct {
	Type        string `json:"type" doc:"标注类型"`
	FileID      string `json:"file_id,omitempty" doc:"文件 ID"`
	Filename    string `json:"filename,omitempty" doc:"文件名"`
	Index       int    `json:"index,omitempty" doc:"索引位置"`
	EndIndex    int    `json:"end_index,omitempty" doc:"结束字符索引"`
	StartIndex  int    `json:"start_index,omitempty" doc:"起始字符索引"`
	Title       string `json:"title,omitempty" doc:"网页标题"`
	URL         string `json:"url,omitempty" doc:"网页 URL"`
	ContainerID string `json:"container_id,omitempty" doc:"容器 ID"`
}

// ResponseOutputTextLogprob 输出文本 token 对数概率
type ResponseOutputTextLogprob struct {
	Token       string                             `json:"token" doc:"token"`
	Bytes       []int                              `json:"bytes,omitempty" doc:"字节序列"`
	Logprob     float64                            `json:"logprob" doc:"对数概率"`
	TopLogprobs []*ResponseOutputTextTopLogprobRow `json:"top_logprobs,omitempty" doc:"候选 top logprobs"`
}

// ResponseOutputTextTopLogprobRow 候选 top logprob 行
type ResponseOutputTextTopLogprobRow struct {
	Token   string  `json:"token" doc:"token"`
	Bytes   []int   `json:"bytes,omitempty" doc:"字节序列"`
	Logprob float64 `json:"logprob" doc:"对数概率"`
}

// ==================== EasyInputMessage.content 的 string|array 联合 ====================

// ResponseInputMessageContent 消息内容，可以是字符串或 ResponseInputContent 数组
type ResponseInputMessageContent struct {
	Text  string                  `json:"-"`
	Parts []*ResponseInputContent `json:"-"`
}

// UnmarshalJSON 区分字符串与数组
func (c *ResponseInputMessageContent) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.Text = s
		return nil
	}
	return sonic.Unmarshal(data, &c.Parts)
}

// MarshalJSON Parts 优先，否则字符串
func (c ResponseInputMessageContent) MarshalJSON() ([]byte, error) {
	if len(c.Parts) > 0 {
		return sonic.Marshal(c.Parts)
	}
	return sonic.Marshal(c.Text)
}

// Schema 声明为 string 或 ResponseInputContent 数组
func (c ResponseInputMessageContent) Schema(r huma.Registry) *huma.Schema {
	contentSchema := r.Schema(reflect.TypeFor[ResponseInputContent](), true, "ResponseInputContent")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "array", Items: contentSchema},
		},
	}
}

// ==================== Computer Action ====================

// Response API computer action type 常量
// ResponseComputerAction Computer 动作（Click/DoubleClick/Drag/Keypress/Move/
// Screenshot/Scroll/Type/Wait 九种联合）
type ResponseComputerAction struct {
	Type string `json:"type" doc:"动作类型"`

	// click 专用: left/right/wheel/back/forward
	Button string `json:"button,omitempty" doc:"鼠标按键"`

	// click/double_click/move/scroll 共用
	X int `json:"x,omitempty" doc:"X 坐标"`
	Y int `json:"y,omitempty" doc:"Y 坐标"`

	// click/double_click/drag/move/scroll/keypress 共用
	Keys []string `json:"keys,omitempty" doc:"按键列表"`

	// drag 专用
	Path []*ResponseComputerActionPathPoint `json:"path,omitempty" doc:"拖拽路径"`

	// scroll 专用
	ScrollX int `json:"scroll_x,omitempty" doc:"水平滚动距离"`
	ScrollY int `json:"scroll_y,omitempty" doc:"垂直滚动距离"`

	// ComputerAction 的 "type" 分支专用
	Text string `json:"text,omitempty" doc:"输入文本"`
}

// ResponseComputerActionPathPoint Drag 动作路径点
type ResponseComputerActionPathPoint struct {
	X int `json:"x" doc:"X 坐标"`
	Y int `json:"y" doc:"Y 坐标"`
}

// ResponsePendingSafetyCheck ComputerCall 的待处理安全检查
type ResponsePendingSafetyCheck struct {
	ID      string `json:"id" doc:"安全检查 ID"`
	Code    string `json:"code,omitempty" doc:"安全检查类型"`
	Message string `json:"message,omitempty" doc:"安全检查详情"`
}

// ResponseComputerCallOutputScreenshot ComputerCallOutput 的截图
type ResponseComputerCallOutputScreenshot struct {
	Type     string `json:"type" doc:"固定为 computer_screenshot"`
	FileID   string `json:"file_id,omitempty" doc:"截图文件 ID"`
	ImageURL string `json:"image_url,omitempty" doc:"截图 URL"`
}

// ==================== Web Search Call Action ====================

// Response API web search action type 常量
// ResponseWebSearchAction WebSearchCall.action（Search/OpenPage/FindInPage 三选一）
type ResponseWebSearchAction struct {
	Type string `json:"type" doc:"动作类型: search/open_page/find_in_page"`

	// search 专用
	Query   string                           `json:"query,omitempty" doc:"[deprecated] 单条搜索查询"`
	Queries []string                         `json:"queries,omitempty" doc:"搜索查询列表"`
	Sources []*ResponseWebSearchActionSource `json:"sources,omitempty" doc:"搜索结果来源"`

	// open_page/find_in_page 共用
	URL string `json:"url,omitempty" doc:"URL"`

	// find_in_page 专用
	Pattern string `json:"pattern,omitempty" doc:"页内搜索模式"`
}

// ResponseWebSearchActionSource WebSearchAction 的来源 URL
type ResponseWebSearchActionSource struct {
	Type string `json:"type" doc:"固定为 url"`
	URL  string `json:"url" doc:"来源 URL"`
}

// ==================== LocalShell Call / Shell Call action & environment ====================

// ResponseLocalShellCallAction LocalShellCall.action (type=exec)
type ResponseLocalShellCallAction struct {
	Command          []string          `json:"command" doc:"命令参数数组"`
	Env              map[string]string `json:"env,omitempty" doc:"环境变量"`
	Type             string            `json:"type" doc:"固定为 exec"`
	TimeoutMS        *int64            `json:"timeout_ms,omitempty" doc:"超时毫秒"`
	User             string            `json:"user,omitempty" doc:"运行用户"`
	WorkingDirectory string            `json:"working_directory,omitempty" doc:"工作目录"`
}

// ResponseShellCallAction ShellCall.action
type ResponseShellCallAction struct {
	Commands        []string `json:"commands" doc:"有序命令列表"`
	MaxOutputLength *int64   `json:"max_output_length,omitempty" doc:"最大输出字符数"`
	TimeoutMS       *int64   `json:"timeout_ms,omitempty" doc:"超时毫秒"`
}

// Response API shell environment type 常量
// ResponseShellEnvironment Shell 或 ShellCall 的环境
// 联合 ContainerAuto / LocalEnvironment / ContainerReference
type ResponseShellEnvironment struct {
	Type string `json:"type" doc:"环境类型"`

	// container_auto 专用
	FileIDs       []string                        `json:"file_ids,omitempty" doc:"上传的文件 ID"`
	MemoryLimit   string                          `json:"memory_limit,omitempty" doc:"内存限制 1g/4g/16g/64g"`
	NetworkPolicy *ResponseContainerNetworkPolicy `json:"network_policy,omitempty" doc:"网络策略"`
	Skills        []*ResponseShellSkill           `json:"skills,omitempty" doc:"技能列表"`

	// container_reference 专用
	ContainerID string `json:"container_id,omitempty" doc:"容器 ID"`
}

// Response API container network policy type 常量
// ResponseContainerNetworkPolicy 容器网络策略（disabled/allowlist）
type ResponseContainerNetworkPolicy struct {
	Type           string                                  `json:"type" doc:"disabled/allowlist"`
	AllowedDomains []string                                `json:"allowed_domains,omitempty" doc:"允许的域名"`
	DomainSecrets  []*ResponseContainerNetworkDomainSecret `json:"domain_secrets,omitempty" doc:"域名作用域的密钥"`
}

// ResponseContainerNetworkDomainSecret 域名密钥
type ResponseContainerNetworkDomainSecret struct {
	Domain string `json:"domain" doc:"域名"`
	Name   string `json:"name" doc:"密钥名称"`
	Value  string `json:"value" doc:"密钥值"`
}

// Response API shell skill type 常量
// ResponseShellSkill ContainerAuto/LocalEnvironment 中的 skill（SkillReference/InlineSkill/LocalSkill）
type ResponseShellSkill struct {
	Type string `json:"type,omitempty" doc:"技能类型"`

	// skill_reference 专用
	SkillID string `json:"skill_id,omitempty" doc:"技能 ID"`
	Version string `json:"version,omitempty" doc:"技能版本"`

	// inline / local 共用
	Description string `json:"description,omitempty" doc:"技能描述"`
	Name        string `json:"name,omitempty" doc:"技能名称"`

	// inline 专用
	Source *ResponseInlineSkillSource `json:"source,omitempty" doc:"inline 技能源"`

	// local 专用
	Path string `json:"path,omitempty" doc:"本地目录路径"`
}

// ResponseInlineSkillSource Inline skill source (base64)
type ResponseInlineSkillSource struct {
	Data      string `json:"data" doc:"base64 zip 数据"`
	MediaType string `json:"media_type" doc:"固定 application/zip"`
	Type      string `json:"type" doc:"固定 base64"`
}

// ==================== Shell Call Output ====================

// Response API shell outcome type 常量
// ResponseShellCallOutputContent ShellCallOutput.output 单条
type ResponseShellCallOutputContent struct {
	Outcome *ResponseShellCallOutcome `json:"outcome" doc:"运行结果"`
	Stderr  string                    `json:"stderr" doc:"stderr 内容"`
	Stdout  string                    `json:"stdout" doc:"stdout 内容"`
}

// ResponseShellCallOutcome timeout/exit
type ResponseShellCallOutcome struct {
	Type     string `json:"type" doc:"timeout 或 exit"`
	ExitCode *int   `json:"exit_code,omitempty" doc:"退出码"`
}

// ==================== FileSearchCall Results ====================

// ResponseFileSearchCallResult FileSearchCall.results 单条
type ResponseFileSearchCallResult struct {
	Attributes map[string]*ResponseFileSearchResultAttribute `json:"attributes,omitempty" doc:"属性键值对"`
	FileID     string                                        `json:"file_id,omitempty" doc:"文件 ID"`
	Filename   string                                        `json:"filename,omitempty" doc:"文件名"`
	Score      *float64                                      `json:"score,omitempty" doc:"相关性分数 0-1"`
	Text       string                                        `json:"text,omitempty" doc:"文件内检索到的文本"`
}

// ResponseFileSearchResultAttribute FileSearchCall.results[].attributes 值
// 允许 string / number / boolean
type ResponseFileSearchResultAttribute struct {
	StringValue *string  `json:"-"`
	NumberValue *float64 `json:"-"`
	BoolValue   *bool    `json:"-"`
}

// UnmarshalJSON 区分 string/number/boolean
func (a *ResponseFileSearchResultAttribute) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		a.StringValue = &s
		return nil
	}
	var n float64
	if err := sonic.Unmarshal(data, &n); err == nil {
		a.NumberValue = &n
		return nil
	}
	var b bool
	if err := sonic.Unmarshal(data, &b); err == nil {
		a.BoolValue = &b
		return nil
	}
	return nil
}

// MarshalJSON 按优先级输出
func (a ResponseFileSearchResultAttribute) MarshalJSON() ([]byte, error) {
	if a.StringValue != nil {
		return sonic.Marshal(*a.StringValue)
	}
	if a.NumberValue != nil {
		return sonic.Marshal(*a.NumberValue)
	}
	if a.BoolValue != nil {
		return sonic.Marshal(*a.BoolValue)
	}
	return []byte("null"), nil
}

// Schema 接受 string/number/boolean
func (ResponseFileSearchResultAttribute) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "number"},
			{Type: "boolean"},
		},
	}
}

// ==================== FunctionCallOutput content ====================

// ResponseFunctionCallOutputContent FunctionCallOutput.output 既可为字符串也可
// 为 ResponseInputContent 数组
type ResponseFunctionCallOutputContent struct {
	Text  string                  `json:"-"`
	Parts []*ResponseInputContent `json:"-"`
}

// UnmarshalJSON 区分字符串和数组
func (c *ResponseFunctionCallOutputContent) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.Text = s
		return nil
	}
	return sonic.Unmarshal(data, &c.Parts)
}

// MarshalJSON Parts 优先
func (c ResponseFunctionCallOutputContent) MarshalJSON() ([]byte, error) {
	if len(c.Parts) > 0 {
		return sonic.Marshal(c.Parts)
	}
	return sonic.Marshal(c.Text)
}

// Schema 接受字符串或 ResponseInputContent 数组
func (ResponseFunctionCallOutputContent) Schema(r huma.Registry) *huma.Schema {
	itemSchema := r.Schema(reflect.TypeFor[ResponseInputContent](), true, "ResponseInputContent")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "array", Items: itemSchema},
		},
	}
}

// ==================== ApplyPatchCall Operation ====================

// Response API apply_patch operation type 常量见 enum.ResponseApplyPatchOpType*。

// ResponseApplyPatchOperation ApplyPatchCall.operation
type ResponseApplyPatchOperation struct {
	Type string `json:"type" doc:"操作类型"`
	Path string `json:"path" doc:"文件路径"`
	Diff string `json:"diff,omitempty" doc:"unified diff 内容"`
}

// ==================== Reasoning summary/content ====================

// ResponseReasoningSummary Reasoning.summary 元素（type=summary_text）
type ResponseReasoningSummary struct {
	Text string `json:"text" doc:"推理摘要文本"`
	Type string `json:"type" doc:"固定 summary_text"`
}

// ResponseReasoningTextContent Reasoning.content 元素（type=reasoning_text）
type ResponseReasoningTextContent struct {
	Text string `json:"text" doc:"推理文本"`
	Type string `json:"type" doc:"固定 reasoning_text"`
}

// ==================== McpListTools ====================

// ResponseMcpListToolsEntry McpListTools.tools 元素
type ResponseMcpListToolsEntry struct {
	InputSchema *JSONSchemaProperty `json:"input_schema" doc:"工具输入 JSON Schema"`
	Name        string              `json:"name" doc:"工具名称"`
	Annotations *JSONSchemaProperty `json:"annotations,omitempty" doc:"工具附加注解"`
	Description string              `json:"description,omitempty" doc:"工具描述"`
}

// ==================== ToolSearchCall / ToolSearchOutput ====================

// ResponseToolSearchCallArguments ToolSearchCall.arguments 为任意 JSON
// 结构化为 JSONSchemaProperty 以兼容 schema 声明
type ResponseToolSearchCallArguments = JSONSchemaProperty

// ==================== Unified Input Item ====================

// ResponseInputItem Response API 输入项联合（28 种 + 其嵌套的 EasyInputMessage 形态）
//
// 由于 Response API input 允许 EasyInputMessage（不带 type 字段）与 26 种带
// 带 type 字段的 item 混用，本结构体通过可选的 type + role 区分：
//   - 当 type 为空或 "message" 时，按 EasyInputMessage / Message / OutputMessage 处理
//   - 其他 type 常量对应各自 item 形态
//
// 各子类型字段按需合并到此结构体，非当前分支字段使用 omitempty。
type ResponseInputItem struct {
	Type   string `json:"type,omitempty" doc:"item 类型，省略时按 message 处理"`
	ID     string `json:"id,omitempty" doc:"item ID"`
	Status string `json:"status,omitempty" doc:"item 状态: in_progress/completed/incomplete/..."`

	// ---------- Message / EasyInputMessage / OutputMessage ----------
	Role    string                       `json:"role,omitempty" doc:"消息角色: user/assistant/system/developer"`
	Phase   string                       `json:"phase,omitempty" doc:"消息阶段: commentary/final_answer"`
	Content *ResponseInputMessageContent `json:"content,omitempty" doc:"消息内容：字符串或 content 数组"`

	// ---------- FileSearchCall ----------
	Queries []string                        `json:"queries,omitempty" doc:"FileSearchCall 查询列表"`
	Results []*ResponseFileSearchCallResult `json:"results,omitempty" doc:"FileSearchCall 结果"`

	// ---------- ComputerCall / ComputerCallOutput ----------
	CallID                   string                        `json:"call_id,omitempty" doc:"调用 ID"`
	PendingSafetyChecks      []*ResponsePendingSafetyCheck `json:"pending_safety_checks,omitempty" doc:"待处理安全检查"`
	AcknowledgedSafetyChecks []*ResponsePendingSafetyCheck `json:"acknowledged_safety_checks,omitempty" doc:"已确认安全检查"`
	Action                   *ResponseInputItemAction      `json:"action,omitempty" doc:"动作（ComputerCall/WebSearchCall/LocalShellCall/ShellCall 等）"`
	Actions                  []*ResponseComputerAction     `json:"actions,omitempty" doc:"扁平化批量动作（computer_use）"`
	Output                   *ResponseInputItemOutput      `json:"output,omitempty" doc:"输出（ComputerCallOutput/FunctionCallOutput/CustomToolCallOutput/LocalShellCallOutput 等）"`

	// ---------- FunctionCall / CustomToolCall ----------
	Arguments string `json:"arguments,omitempty" doc:"函数参数 JSON 字符串（FunctionCall/McpApprovalRequest/McpCall）或 ToolSearchCall 任意 JSON"`
	Name      string `json:"name,omitempty" doc:"函数/工具/命名空间名称"`
	Namespace string `json:"namespace,omitempty" doc:"工具命名空间"`
	Input     string `json:"input,omitempty" doc:"自定义工具调用输入"`

	// ---------- ToolSearchCall ----------
	Execution string `json:"execution,omitempty" doc:"执行端: server/client (ToolSearchCall/ToolSearchOutput)"`

	// ---------- ToolSearchOutput ----------
	Tools []*ResponseTool `json:"tools,omitempty" doc:"ToolSearchOutput 返回的工具定义 / McpListTools 条目通过 McpTools"`

	// ---------- McpListTools ----------
	McpTools    []*ResponseMcpListToolsEntry `json:"mcp_tools,omitempty" doc:"McpListTools 返回的工具列表（内部字段，序列化使用 MarshalJSON）"`
	ServerLabel string                       `json:"server_label,omitempty" doc:"MCP 服务器标签"`
	Error       string                       `json:"error,omitempty" doc:"MCP 调用错误信息"`

	// ---------- McpApprovalRequest / McpApprovalResponse / McpCall ----------
	ApprovalRequestID string `json:"approval_request_id,omitempty" doc:"MCP 审批请求 ID"`
	Approve           *bool  `json:"approve,omitempty" doc:"MCP 审批结果"`
	Reason            string `json:"reason,omitempty" doc:"MCP 审批原因"`

	// ---------- Reasoning ----------
	Summary          []*ResponseReasoningSummary     `json:"summary,omitempty" doc:"Reasoning 摘要内容"`
	ReasoningContent []*ResponseReasoningTextContent `json:"-"` // 内部保留，序列化时借由自定义 MarshalJSON 输出为 content 字段

	// ---------- Compaction / Reasoning ----------
	EncryptedContent string `json:"encrypted_content,omitempty" doc:"加密内容（Compaction/Reasoning）"`

	// ---------- ImageGenerationCall ----------
	Result string `json:"result,omitempty" doc:"ImageGenerationCall 生成的 base64 图像"`

	// ---------- CodeInterpreterCall ----------
	Code        string                               `json:"code,omitempty" doc:"代码内容"`
	ContainerID string                               `json:"container_id,omitempty" doc:"容器 ID (CodeInterpreterCall)"`
	Outputs     []*ResponseCodeInterpreterCallOutput `json:"outputs,omitempty" doc:"代码执行产物"`

	// ---------- ShellCall ----------
	Environment *ResponseShellEnvironment `json:"environment,omitempty" doc:"Shell/ShellCall 运行环境"`

	// ---------- ShellCallOutput ----------
	MaxOutputLength *int64 `json:"max_output_length,omitempty" doc:"ShellCallOutput 最大输出字符数"`

	// ---------- ApplyPatchCall ----------
	Operation *ResponseApplyPatchOperation `json:"operation,omitempty" doc:"apply_patch 操作指令"`
}

// ResponseCodeInterpreterCallOutput CodeInterpreterCall 输出 (logs|image)
type ResponseCodeInterpreterCallOutput struct {
	Type string `json:"type" doc:"logs 或 image"`
	Logs string `json:"logs,omitempty" doc:"日志内容（type=logs）"`
	URL  string `json:"url,omitempty" doc:"图像 URL（type=image）"`
}

// ==================== ResponseInputItem action/output 联合 ====================

// ResponseInputItemAction ComputerCall / WebSearchCall / LocalShellCall / ShellCall
// 中 action 字段的联合类型。通过 type 区分具体形态。
type ResponseInputItemAction struct {
	// 公共
	Type string `json:"type" doc:"动作类型"`

	// ComputerAction
	Button  string                             `json:"button,omitempty"`
	X       int                                `json:"x,omitempty"`
	Y       int                                `json:"y,omitempty"`
	Keys    []string                           `json:"keys,omitempty"`
	Path    []*ResponseComputerActionPathPoint `json:"path,omitempty"`
	ScrollX int                                `json:"scroll_x,omitempty"`
	ScrollY int                                `json:"scroll_y,omitempty"`
	Text    string                             `json:"text,omitempty"`

	// WebSearchAction
	Query   string                           `json:"query,omitempty"`
	Queries []string                         `json:"queries,omitempty"`
	Sources []*ResponseWebSearchActionSource `json:"sources,omitempty"`
	URL     string                           `json:"url,omitempty"`
	Pattern string                           `json:"pattern,omitempty"`

	// LocalShellCallAction (type=exec)
	Command          []string          `json:"command,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	TimeoutMS        *int64            `json:"timeout_ms,omitempty"`
	User             string            `json:"user,omitempty"`
	WorkingDirectory string            `json:"working_directory,omitempty"`

	// ShellCallAction
	Commands        []string `json:"commands,omitempty"`
	MaxOutputLength *int64   `json:"max_output_length,omitempty"`
}

// ResponseInputItemOutput ComputerCallOutput / FunctionCallOutput /
// CustomToolCallOutput / LocalShellCallOutput / McpCall / ShellCallOutput 中
// output 字段的联合
type ResponseInputItemOutput struct {
	// ComputerCallOutput: ResponseComputerToolCallOutputScreenshot
	Screenshot *ResponseComputerCallOutputScreenshot `json:"-"`
	// FunctionCallOutput / CustomToolCallOutput: string | content list
	FunctionOutput *ResponseFunctionCallOutputContent `json:"-"`
	// LocalShellCallOutput: 纯字符串
	Text string `json:"-"`
	// ShellCallOutput: 数组
	ShellOutputs []*ResponseShellCallOutputContent `json:"-"`
}

// UnmarshalJSON 依次尝试 string / screenshot 对象 / content 数组
func (o *ResponseInputItemOutput) UnmarshalJSON(data []byte) error {
	// 尝试字符串
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		o.Text = s
		return nil
	}
	// 尝试数组（ShellCallOutput / FunctionCallOutput content list）
	if len(data) > 0 && data[0] == '[' {
		var shellItems []*ResponseShellCallOutputContent
		if err := sonic.Unmarshal(data, &shellItems); err == nil && len(shellItems) > 0 && shellItems[0].Outcome != nil {
			o.ShellOutputs = shellItems
			return nil
		}
		// 回退为 function call output 的内容数组
		o.FunctionOutput = &ResponseFunctionCallOutputContent{}
		return sonic.Unmarshal(data, &o.FunctionOutput.Parts)
	}
	// 尝试 computer screenshot 对象
	o.Screenshot = &ResponseComputerCallOutputScreenshot{}
	return sonic.Unmarshal(data, o.Screenshot)
}

// MarshalJSON 按已设置分支输出
func (o ResponseInputItemOutput) MarshalJSON() ([]byte, error) {
	switch {
	case o.Screenshot != nil:
		return sonic.Marshal(o.Screenshot)
	case o.FunctionOutput != nil:
		return sonic.Marshal(o.FunctionOutput)
	case o.ShellOutputs != nil:
		return sonic.Marshal(o.ShellOutputs)
	default:
		return sonic.Marshal(o.Text)
	}
}

// Schema 接受 string / 对象 / 数组
func (ResponseInputItemOutput) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "object", AdditionalProperties: true},
			{Type: "array", Items: &huma.Schema{Type: "object", AdditionalProperties: true}},
		},
	}
}

// ==================== ResponseInput union (string | array of ResponseInputItem) ====================

// ResponseInput Response API 顶层 input 字段（string 或 ResponseInputItem 数组）
type ResponseInput struct {
	Text  string               `json:"-"`
	Items []*ResponseInputItem `json:"-"`
}

// UnmarshalJSON 区分字符串与数组
func (r *ResponseInput) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		r.Text = s
		return nil
	}
	return sonic.Unmarshal(data, &r.Items)
}

// MarshalJSON Items 优先，否则输出字符串
func (r ResponseInput) MarshalJSON() ([]byte, error) {
	if r.Items != nil {
		return sonic.Marshal(r.Items)
	}
	return sonic.Marshal(r.Text)
}

// Schema string 或 ResponseInputItem 数组
func (ResponseInput) Schema(reg huma.Registry) *huma.Schema {
	itemSchema := reg.Schema(reflect.TypeFor[ResponseInputItem](), true, "ResponseInputItem")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "array", Items: itemSchema},
		},
	}
}
