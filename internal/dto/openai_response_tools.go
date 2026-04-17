package dto

import (
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// ==================== Response API Tools ====================
//
// 参考 docs/openai/create_response.md 第 3547-4282 行 tools 数组元素定义。
// 14 种 tool 通过 `type` 字段区分。采用自定义 Marshal/Unmarshal 的联合类型方式：
// ResponseTool 只暴露 Type 与 14 个子结构体指针字段（JSON 侧无对应 tag），
// 由 MarshalJSON/UnmarshalJSON 根据 Type 路由到具体子结构体。
//
//	@author centonhuang
//	@update 2026-04-17 17:00:00

// Response API tool type 字面量常量见 enum.ResponseToolType*。

// ==================== FileSearch 相关 ====================

// Response API file search filter type 字面量常量见 enum.ResponseFileSearchFilterType*。

// ResponseFileSearchFilter FileSearch.filters 联合：ComparisonFilter / CompoundFilter
type ResponseFileSearchFilter struct {
	Type    string                         `json:"type" doc:"过滤类型"`
	Key     string                         `json:"key,omitempty" doc:"比较字段"`
	Value   *ResponseFileSearchFilterValue `json:"value,omitempty" doc:"比较值"`
	Filters []*ResponseFileSearchFilter    `json:"filters,omitempty" doc:"子过滤器列表"`
}

// ResponseFileSearchFilterValue 比较值：string | number | boolean | array<string|number>
type ResponseFileSearchFilterValue struct {
	StringValue *string   `json:"-"`
	NumberValue *float64  `json:"-"`
	BoolValue   *bool     `json:"-"`
	Strings     []string  `json:"-"`
	Numbers     []float64 `json:"-"`
}

// UnmarshalJSON 依次尝试 string/number/bool/array
func (v *ResponseFileSearchFilterValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		v.StringValue = &s
		return nil
	}
	var f float64
	if err := sonic.Unmarshal(data, &f); err == nil {
		v.NumberValue = &f
		return nil
	}
	var b bool
	if err := sonic.Unmarshal(data, &b); err == nil {
		v.BoolValue = &b
		return nil
	}
	var ss []string
	if err := sonic.Unmarshal(data, &ss); err == nil {
		v.Strings = ss
		return nil
	}
	return sonic.Unmarshal(data, &v.Numbers)
}

// MarshalJSON 按已设置分支输出
func (v ResponseFileSearchFilterValue) MarshalJSON() ([]byte, error) {
	switch {
	case v.StringValue != nil:
		return sonic.Marshal(*v.StringValue)
	case v.NumberValue != nil:
		return sonic.Marshal(*v.NumberValue)
	case v.BoolValue != nil:
		return sonic.Marshal(*v.BoolValue)
	case v.Strings != nil:
		return sonic.Marshal(v.Strings)
	case v.Numbers != nil:
		return sonic.Marshal(v.Numbers)
	}
	return []byte("null"), nil
}

// Schema 接受 string/number/boolean/数组
func (ResponseFileSearchFilterValue) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "number"},
			{Type: "boolean"},
			{Type: "array", Items: &huma.Schema{OneOf: []*huma.Schema{{Type: "string"}, {Type: "number"}}}},
		},
	}
}

// ResponseFileSearchRankingOptions FileSearch.ranking_options
type ResponseFileSearchRankingOptions struct {
	HybridSearch   *ResponseFileSearchHybridSearch `json:"hybrid_search,omitempty" doc:"混合检索权重"`
	Ranker         string                          `json:"ranker,omitempty" doc:"ranker: auto/default-2024-11-15"`
	ScoreThreshold *float64                        `json:"score_threshold,omitempty" doc:"分数阈值"`
}

// ResponseFileSearchHybridSearch 混合检索权重
type ResponseFileSearchHybridSearch struct {
	EmbeddingWeight float64 `json:"embedding_weight" doc:"embedding 权重"`
	TextWeight      float64 `json:"text_weight" doc:"文本权重"`
}

// ==================== WebSearch 用户位置 ====================

// ResponseWebSearchFilters WebSearch 的 filters
type ResponseWebSearchFilters struct {
	AllowedDomains []string `json:"allowed_domains,omitempty" doc:"允许的域名"`
}

// ResponseWebSearchUserLocation 用户位置
type ResponseWebSearchUserLocation struct {
	Type     string `json:"type,omitempty" doc:"固定 approximate"`
	City     string `json:"city,omitempty" doc:"城市"`
	Country  string `json:"country,omitempty" doc:"国家 ISO 代码"`
	Region   string `json:"region,omitempty" doc:"地区"`
	Timezone string `json:"timezone,omitempty" doc:"IANA 时区"`
}

// ==================== MCP 相关 ====================

// ResponseMcpAllowedTools Mcp.allowed_tools: array<string> | McpToolFilter
type ResponseMcpAllowedTools struct {
	Names  []string               `json:"-"`
	Filter *ResponseMcpToolFilter `json:"-"`
}

// UnmarshalJSON 数组或对象
func (a *ResponseMcpAllowedTools) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		return sonic.Unmarshal(data, &a.Names)
	}
	a.Filter = &ResponseMcpToolFilter{}
	return sonic.Unmarshal(data, a.Filter)
}

// MarshalJSON 按分支输出
func (a ResponseMcpAllowedTools) MarshalJSON() ([]byte, error) {
	if a.Filter != nil {
		return sonic.Marshal(a.Filter)
	}
	return sonic.Marshal(a.Names)
}

// Schema 接受数组或对象
func (ResponseMcpAllowedTools) Schema(reg huma.Registry) *huma.Schema {
	filterSchema := reg.Schema(reflect.TypeFor[ResponseMcpToolFilter](), true, "ResponseMcpToolFilter")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "array", Items: &huma.Schema{Type: "string"}},
			filterSchema,
		},
	}
}

// ResponseMcpToolFilter Mcp allowed_tools filter
type ResponseMcpToolFilter struct {
	ReadOnly  *bool    `json:"read_only,omitempty" doc:"是否只读"`
	ToolNames []string `json:"tool_names,omitempty" doc:"工具名称列表"`
}

// ResponseMcpRequireApproval Mcp.require_approval: filter object | "always" | "never"
type ResponseMcpRequireApproval struct {
	Mode   string                         `json:"-"`
	Filter *ResponseMcpToolApprovalFilter `json:"-"`
}

// UnmarshalJSON 字符串或对象
func (r *ResponseMcpRequireApproval) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		r.Mode = s
		return nil
	}
	r.Filter = &ResponseMcpToolApprovalFilter{}
	return sonic.Unmarshal(data, r.Filter)
}

// MarshalJSON 按分支输出
func (r ResponseMcpRequireApproval) MarshalJSON() ([]byte, error) {
	if r.Filter != nil {
		return sonic.Marshal(r.Filter)
	}
	return sonic.Marshal(r.Mode)
}

// Schema 接受字符串或对象
func (ResponseMcpRequireApproval) Schema(reg huma.Registry) *huma.Schema {
	filterSchema := reg.Schema(reflect.TypeFor[ResponseMcpToolApprovalFilter](), true, "ResponseMcpToolApprovalFilter")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string", Enum: []any{"always", "never"}},
			filterSchema,
		},
	}
}

// ResponseMcpToolApprovalFilter Mcp require_approval 过滤器
type ResponseMcpToolApprovalFilter struct {
	Always *ResponseMcpToolFilter `json:"always,omitempty" doc:"需要审批的工具过滤"`
	Never  *ResponseMcpToolFilter `json:"never,omitempty" doc:"不需要审批的工具过滤"`
}

// ==================== CodeInterpreter Container ====================

// ResponseCodeInterpreterContainer CodeInterpreter.container: string | CodeInterpreterToolAuto
type ResponseCodeInterpreterContainer struct {
	ID   string                                `json:"-"`
	Auto *ResponseCodeInterpreterContainerAuto `json:"-"`
}

// UnmarshalJSON 字符串或对象
func (c *ResponseCodeInterpreterContainer) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.ID = s
		return nil
	}
	c.Auto = &ResponseCodeInterpreterContainerAuto{}
	return sonic.Unmarshal(data, c.Auto)
}

// MarshalJSON 按分支输出
func (c ResponseCodeInterpreterContainer) MarshalJSON() ([]byte, error) {
	if c.Auto != nil {
		return sonic.Marshal(c.Auto)
	}
	return sonic.Marshal(c.ID)
}

// Schema 字符串或对象
func (ResponseCodeInterpreterContainer) Schema(reg huma.Registry) *huma.Schema {
	autoSchema := reg.Schema(reflect.TypeFor[ResponseCodeInterpreterContainerAuto](), true, "ResponseCodeInterpreterContainerAuto")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			autoSchema,
		},
	}
}

// ResponseCodeInterpreterContainerAuto CodeInterpreterToolAuto
type ResponseCodeInterpreterContainerAuto struct {
	Type          string                          `json:"type" doc:"固定 auto"`
	FileIDs       []string                        `json:"file_ids,omitempty" doc:"文件 ID 列表"`
	MemoryLimit   string                          `json:"memory_limit,omitempty" doc:"内存限制 1g/4g/16g/64g"`
	NetworkPolicy *ResponseContainerNetworkPolicy `json:"network_policy,omitempty" doc:"网络策略"`
}

// ==================== ImageGeneration Input Image Mask ====================

// ResponseImageGenerationMask ImageGeneration.input_image_mask
type ResponseImageGenerationMask struct {
	FileID   string `json:"file_id,omitempty" doc:"掩码文件 ID"`
	ImageURL string `json:"image_url,omitempty" doc:"base64 掩码图像"`
}

// ==================== Custom tool format ====================

// Response API custom tool format type 常量
// ResponseCustomToolFormat Custom.format (Text/Grammar)
type ResponseCustomToolFormat struct {
	Type       string `json:"type" doc:"text 或 grammar"`
	Definition string `json:"definition,omitempty" doc:"grammar 定义（type=grammar）"`
	Syntax     string `json:"syntax,omitempty" doc:"lark 或 regex (type=grammar)"`
}

// ==================== 14 种 tool 的独立定义 ====================

// ResponseToolFunction Function 工具
type ResponseToolFunction struct {
	Type         string              `json:"type" doc:"固定 function"`
	Name         string              `json:"name" doc:"函数名称"`
	Parameters   *JSONSchemaProperty `json:"parameters" doc:"函数参数 JSON schema"`
	Strict       bool                `json:"strict" doc:"是否严格校验参数"`
	DeferLoading *bool               `json:"defer_loading,omitempty" doc:"是否延迟加载"`
	Description  string              `json:"description,omitempty" doc:"函数描述"`
}

// ResponseToolFileSearch FileSearch 工具
type ResponseToolFileSearch struct {
	Type           string                            `json:"type" doc:"固定 file_search"`
	VectorStoreIDs []string                          `json:"vector_store_ids" doc:"向量库 ID 列表"`
	Filters        *ResponseFileSearchFilter         `json:"filters,omitempty" doc:"过滤器"`
	MaxNumResults  *int                              `json:"max_num_results,omitempty" doc:"最多返回结果数"`
	RankingOptions *ResponseFileSearchRankingOptions `json:"ranking_options,omitempty" doc:"排序选项"`
}

// ResponseToolComputer Computer 工具
type ResponseToolComputer struct {
	Type string `json:"type" doc:"固定 computer"`
}

// ResponseToolComputerUsePreview ComputerUsePreview 工具
type ResponseToolComputerUsePreview struct {
	DisplayHeight int    `json:"display_height" doc:"显示器高度"`
	DisplayWidth  int    `json:"display_width" doc:"显示器宽度"`
	Environment   string `json:"environment" doc:"环境: windows/mac/linux/ubuntu/browser"`
	Type          string `json:"type" doc:"固定 computer_use_preview"`
}

// ResponseToolWebSearch WebSearch 工具
type ResponseToolWebSearch struct {
	Type              string                         `json:"type" doc:"web_search 或 web_search_2025_08_26"`
	Filters           *ResponseWebSearchFilters      `json:"filters,omitempty" doc:"WebSearch 过滤器"`
	SearchContextSize string                         `json:"search_context_size,omitempty" doc:"low/medium/high"`
	UserLocation      *ResponseWebSearchUserLocation `json:"user_location,omitempty" doc:"用户位置"`
}

// ResponseToolMcp Mcp 工具
type ResponseToolMcp struct {
	Type              string                      `json:"type" doc:"固定 mcp"`
	ServerLabel       string                      `json:"server_label" doc:"MCP 服务器标签"`
	AllowedTools      *ResponseMcpAllowedTools    `json:"allowed_tools,omitempty" doc:"允许的工具"`
	Authorization     string                      `json:"authorization,omitempty" doc:"OAuth token"`
	ConnectorID       string                      `json:"connector_id,omitempty" doc:"连接器 ID"`
	DeferLoading      *bool                       `json:"defer_loading,omitempty" doc:"是否延迟加载"`
	Headers           map[string]string           `json:"headers,omitempty" doc:"HTTP 头"`
	RequireApproval   *ResponseMcpRequireApproval `json:"require_approval,omitempty" doc:"审批配置"`
	ServerDescription string                      `json:"server_description,omitempty" doc:"服务器描述"`
	ServerURL         string                      `json:"server_url,omitempty" doc:"服务器 URL"`
}

// ResponseToolCodeInterpreter CodeInterpreter 工具
type ResponseToolCodeInterpreter struct {
	Type      string                            `json:"type" doc:"固定 code_interpreter"`
	Container *ResponseCodeInterpreterContainer `json:"container" doc:"容器 ID 或 CodeInterpreterToolAuto 对象"`
}

// ResponseToolImageGeneration ImageGeneration 工具
type ResponseToolImageGeneration struct {
	Type              string                       `json:"type" doc:"固定 image_generation"`
	Action            string                       `json:"action,omitempty" doc:"generate/edit/auto"`
	Background        string                       `json:"background,omitempty" doc:"transparent/opaque/auto"`
	InputFidelity     string                       `json:"input_fidelity,omitempty" doc:"high/low"`
	InputImageMask    *ResponseImageGenerationMask `json:"input_image_mask,omitempty" doc:"掩码图像"`
	Model             string                       `json:"model,omitempty" doc:"图像模型"`
	Moderation        string                       `json:"moderation,omitempty" doc:"auto/low"`
	OutputCompression *int                         `json:"output_compression,omitempty" doc:"压缩度"`
	OutputFormat      string                       `json:"output_format,omitempty" doc:"png/webp/jpeg"`
	PartialImages     *int                         `json:"partial_images,omitempty" doc:"部分图像数量"`
	Quality           string                       `json:"quality,omitempty" doc:"low/medium/high/auto"`
	Size              string                       `json:"size,omitempty" doc:"图像尺寸"`
}

// ResponseToolLocalShell LocalShell 工具
type ResponseToolLocalShell struct {
	Type string `json:"type" doc:"固定 local_shell"`
}

// ResponseToolShell Shell 工具
type ResponseToolShell struct {
	Type        string                    `json:"type" doc:"固定 shell"`
	Environment *ResponseShellEnvironment `json:"environment,omitempty" doc:"Shell 运行环境"`
}

// ResponseToolCustom Custom 工具
type ResponseToolCustom struct {
	Type         string                    `json:"type" doc:"固定 custom"`
	Name         string                    `json:"name" doc:"自定义工具名称"`
	DeferLoading *bool                     `json:"defer_loading,omitempty" doc:"是否延迟加载"`
	Description  string                    `json:"description,omitempty" doc:"工具描述"`
	Format       *ResponseCustomToolFormat `json:"format,omitempty" doc:"输入格式"`
}

// ResponseToolNamespace Namespace 工具
type ResponseToolNamespace struct {
	Type        string                   `json:"type" doc:"固定 namespace"`
	Description string                   `json:"description" doc:"命名空间描述"`
	Name        string                   `json:"name" doc:"命名空间名称"`
	Tools       []*ResponseNamespaceTool `json:"tools" doc:"命名空间内的工具列表"`
}

// ResponseNamespaceTool Namespace.tools: Function | Custom
type ResponseNamespaceTool struct {
	Name         string                    `json:"name" doc:"工具名称"`
	Type         string                    `json:"type" doc:"function 或 custom"`
	DeferLoading *bool                     `json:"defer_loading,omitempty" doc:"是否延迟加载"`
	Description  string                    `json:"description,omitempty" doc:"工具描述"`
	Parameters   *JSONSchemaProperty       `json:"parameters,omitempty" doc:"函数参数 JSON schema"`
	Strict       *bool                     `json:"strict,omitempty" doc:"严格模式"`
	Format       *ResponseCustomToolFormat `json:"format,omitempty" doc:"自定义工具输入格式"`
}

// ResponseToolToolSearch ToolSearch 工具
type ResponseToolToolSearch struct {
	Type        string              `json:"type" doc:"固定 tool_search"`
	Description string              `json:"description,omitempty" doc:"工具描述"`
	Execution   string              `json:"execution,omitempty" doc:"server 或 client"`
	Parameters  *JSONSchemaProperty `json:"parameters,omitempty" doc:"参数 JSON schema"`
}

// ResponseToolWebSearchPreview WebSearchPreview 工具
type ResponseToolWebSearchPreview struct {
	Type               string                         `json:"type" doc:"web_search_preview 或 web_search_preview_2025_03_11"`
	SearchContentTypes []string                       `json:"search_content_types,omitempty" doc:"text 或 image"`
	SearchContextSize  string                         `json:"search_context_size,omitempty" doc:"low/medium/high"`
	UserLocation       *ResponseWebSearchUserLocation `json:"user_location,omitempty" doc:"用户位置"`
}

// ResponseToolApplyPatch ApplyPatch 工具
type ResponseToolApplyPatch struct {
	Type string `json:"type" doc:"固定 apply_patch"`
}

// ==================== 统一 ResponseTool 联合 ====================

// ResponseTool Response API tools 数组元素联合类型。根据 type 字段分派到具体子结构体。
type ResponseTool struct {
	Type string `json:"-"` // 通过具体 union 字段读取

	Function           *ResponseToolFunction           `json:"-"`
	FileSearch         *ResponseToolFileSearch         `json:"-"`
	Computer           *ResponseToolComputer           `json:"-"`
	ComputerUsePreview *ResponseToolComputerUsePreview `json:"-"`
	WebSearch          *ResponseToolWebSearch          `json:"-"`
	Mcp                *ResponseToolMcp                `json:"-"`
	CodeInterpreter    *ResponseToolCodeInterpreter    `json:"-"`
	ImageGeneration    *ResponseToolImageGeneration    `json:"-"`
	LocalShell         *ResponseToolLocalShell         `json:"-"`
	Shell              *ResponseToolShell              `json:"-"`
	Custom             *ResponseToolCustom             `json:"-"`
	Namespace          *ResponseToolNamespace          `json:"-"`
	ToolSearch         *ResponseToolToolSearch         `json:"-"`
	WebSearchPreview   *ResponseToolWebSearchPreview   `json:"-"`
	ApplyPatch         *ResponseToolApplyPatch         `json:"-"`
}

// responseToolTypeProbe 用于从 tool JSON 中仅提取 type 字段
type responseToolTypeProbe struct {
	Type string `json:"type"`
}

// UnmarshalJSON 按 type 分派
func (t *ResponseTool) UnmarshalJSON(data []byte) error {
	var probe responseToolTypeProbe
	if err := sonic.Unmarshal(data, &probe); err != nil {
		return err
	}
	t.Type = probe.Type
	switch probe.Type {
	case enum.ResponseToolTypeFunction:
		t.Function = &ResponseToolFunction{}
		return sonic.Unmarshal(data, t.Function)
	case enum.ResponseToolTypeFileSearch:
		t.FileSearch = &ResponseToolFileSearch{}
		return sonic.Unmarshal(data, t.FileSearch)
	case enum.ResponseToolTypeComputer:
		t.Computer = &ResponseToolComputer{}
		return sonic.Unmarshal(data, t.Computer)
	case enum.ResponseToolTypeComputerUsePreview:
		t.ComputerUsePreview = &ResponseToolComputerUsePreview{}
		return sonic.Unmarshal(data, t.ComputerUsePreview)
	case enum.ResponseToolTypeWebSearch, enum.ResponseToolTypeWebSearch20250826:
		t.WebSearch = &ResponseToolWebSearch{}
		return sonic.Unmarshal(data, t.WebSearch)
	case enum.ResponseToolTypeMcp:
		t.Mcp = &ResponseToolMcp{}
		return sonic.Unmarshal(data, t.Mcp)
	case enum.ResponseToolTypeCodeInterpreter:
		t.CodeInterpreter = &ResponseToolCodeInterpreter{}
		return sonic.Unmarshal(data, t.CodeInterpreter)
	case enum.ResponseToolTypeImageGeneration:
		t.ImageGeneration = &ResponseToolImageGeneration{}
		return sonic.Unmarshal(data, t.ImageGeneration)
	case enum.ResponseToolTypeLocalShell:
		t.LocalShell = &ResponseToolLocalShell{}
		return sonic.Unmarshal(data, t.LocalShell)
	case enum.ResponseToolTypeShell:
		t.Shell = &ResponseToolShell{}
		return sonic.Unmarshal(data, t.Shell)
	case enum.ResponseToolTypeCustom:
		t.Custom = &ResponseToolCustom{}
		return sonic.Unmarshal(data, t.Custom)
	case enum.ResponseToolTypeNamespace:
		t.Namespace = &ResponseToolNamespace{}
		return sonic.Unmarshal(data, t.Namespace)
	case enum.ResponseToolTypeToolSearch:
		t.ToolSearch = &ResponseToolToolSearch{}
		return sonic.Unmarshal(data, t.ToolSearch)
	case enum.ResponseToolTypeWebSearchPreview, enum.ResponseToolTypeWebSearchPreview311:
		t.WebSearchPreview = &ResponseToolWebSearchPreview{}
		return sonic.Unmarshal(data, t.WebSearchPreview)
	case enum.ResponseToolTypeApplyPatch:
		t.ApplyPatch = &ResponseToolApplyPatch{}
		return sonic.Unmarshal(data, t.ApplyPatch)
	default:
		// 未知 type 同样接受，保存为 Function 分派失败时的通用容器；这里以 Function 的宽结构承载
		t.Function = &ResponseToolFunction{}
		return sonic.Unmarshal(data, t.Function)
	}
}

// MarshalJSON 按已设置分支输出
func (t ResponseTool) MarshalJSON() ([]byte, error) {
	switch {
	case t.Function != nil:
		return sonic.Marshal(t.Function)
	case t.FileSearch != nil:
		return sonic.Marshal(t.FileSearch)
	case t.Computer != nil:
		return sonic.Marshal(t.Computer)
	case t.ComputerUsePreview != nil:
		return sonic.Marshal(t.ComputerUsePreview)
	case t.WebSearch != nil:
		return sonic.Marshal(t.WebSearch)
	case t.Mcp != nil:
		return sonic.Marshal(t.Mcp)
	case t.CodeInterpreter != nil:
		return sonic.Marshal(t.CodeInterpreter)
	case t.ImageGeneration != nil:
		return sonic.Marshal(t.ImageGeneration)
	case t.LocalShell != nil:
		return sonic.Marshal(t.LocalShell)
	case t.Shell != nil:
		return sonic.Marshal(t.Shell)
	case t.Custom != nil:
		return sonic.Marshal(t.Custom)
	case t.Namespace != nil:
		return sonic.Marshal(t.Namespace)
	case t.ToolSearch != nil:
		return sonic.Marshal(t.ToolSearch)
	case t.WebSearchPreview != nil:
		return sonic.Marshal(t.WebSearchPreview)
	case t.ApplyPatch != nil:
		return sonic.Marshal(t.ApplyPatch)
	}
	return []byte("null"), nil
}

// Schema 声明为 14 种 tool 的 oneOf
func (ResponseTool) Schema(reg huma.Registry) *huma.Schema {
	return &huma.Schema{
		OneOf: []*huma.Schema{
			reg.Schema(reflect.TypeFor[ResponseToolFunction](), true, "ResponseToolFunction"),
			reg.Schema(reflect.TypeFor[ResponseToolFileSearch](), true, "ResponseToolFileSearch"),
			reg.Schema(reflect.TypeFor[ResponseToolComputer](), true, "ResponseToolComputer"),
			reg.Schema(reflect.TypeFor[ResponseToolComputerUsePreview](), true, "ResponseToolComputerUsePreview"),
			reg.Schema(reflect.TypeFor[ResponseToolWebSearch](), true, "ResponseToolWebSearch"),
			reg.Schema(reflect.TypeFor[ResponseToolMcp](), true, "ResponseToolMcp"),
			reg.Schema(reflect.TypeFor[ResponseToolCodeInterpreter](), true, "ResponseToolCodeInterpreter"),
			reg.Schema(reflect.TypeFor[ResponseToolImageGeneration](), true, "ResponseToolImageGeneration"),
			reg.Schema(reflect.TypeFor[ResponseToolLocalShell](), true, "ResponseToolLocalShell"),
			reg.Schema(reflect.TypeFor[ResponseToolShell](), true, "ResponseToolShell"),
			reg.Schema(reflect.TypeFor[ResponseToolCustom](), true, "ResponseToolCustom"),
			reg.Schema(reflect.TypeFor[ResponseToolNamespace](), true, "ResponseToolNamespace"),
			reg.Schema(reflect.TypeFor[ResponseToolToolSearch](), true, "ResponseToolToolSearch"),
			reg.Schema(reflect.TypeFor[ResponseToolWebSearchPreview](), true, "ResponseToolWebSearchPreview"),
			reg.Schema(reflect.TypeFor[ResponseToolApplyPatch](), true, "ResponseToolApplyPatch"),
		},
	}
}

// ==================== tool_choice 联合 ====================

// Response API tool_choice option 与 type 常量见 enum.ResponseToolChoiceOption* 与
// enum.ResponseToolChoiceType*。

// ResponseToolChoiceParam Response API tool_choice（string 或 对象）
type ResponseToolChoiceParam struct {
	Mode   string                    `json:"-"`
	Object *ResponseToolChoiceObject `json:"-"`
}

// UnmarshalJSON 字符串或对象
func (t *ResponseToolChoiceParam) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		t.Mode = s
		return nil
	}
	t.Object = &ResponseToolChoiceObject{}
	return sonic.Unmarshal(data, t.Object)
}

// MarshalJSON 按分支输出
func (t ResponseToolChoiceParam) MarshalJSON() ([]byte, error) {
	if t.Object != nil {
		return sonic.Marshal(t.Object)
	}
	return sonic.Marshal(t.Mode)
}

// Schema 字符串或对象
func (ResponseToolChoiceParam) Schema(reg huma.Registry) *huma.Schema {
	objSchema := reg.Schema(reflect.TypeFor[ResponseToolChoiceObject](), true, "ResponseToolChoiceObject")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string", Enum: []any{"none", "auto", "required"}},
			objSchema,
		},
	}
}

// ResponseToolChoiceObject ToolChoice 对象分支
// 联合：ToolChoiceAllowed（type=allowed_tools，含 mode + tools）
//
//	ToolChoiceTypes（type 为 file_search/web_search_preview/... 等）
//	ToolChoiceFunction（type=function，含 name）
//	ToolChoiceMcp（type=mcp，含 server_label + 可选 name）
//	ToolChoiceCustom（type=custom，含 name）
//	ToolChoiceApplyPatch（type=apply_patch）
//	ToolChoiceShell（type=shell）
//
// 为避免分支爆炸，采用扁平字段 + omitempty 的方式聚合所有可能字段。
type ResponseToolChoiceObject struct {
	Type string `json:"type" doc:"工具选择类型"`

	// ToolChoiceAllowed
	Mode  string                `json:"mode,omitempty" doc:"allowed_tools 模式: auto/required"`
	Tools []*JSONSchemaProperty `json:"tools,omitempty" doc:"允许的工具定义列表"`

	// ToolChoiceFunction / ToolChoiceCustom / ToolChoiceMcp
	Name string `json:"name,omitempty" doc:"工具/函数名称"`

	// ToolChoiceMcp
	ServerLabel string `json:"server_label,omitempty" doc:"MCP 服务器标签"`
}
