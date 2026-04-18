// Package enum provides common enums for the application.
package enum

// ==================== Response API Input Item Types ====================
//
// 参考 docs/openai/create_response.md 第 77-2896 行 InputItemList 定义。
// 28 种 input item 通过 `type` 字段区分。
//
//	@author centonhuang
//	@update 2026-04-17 17:00:00

// ResponseInputItemType Response API input item 的 type 枚举
type ResponseInputItemType = string

const (
	// ResponseInputItemTypeMessage 消息输入（EasyInputMessage/Message/OutputMessage）
	ResponseInputItemTypeMessage ResponseInputItemType = "message"
	// ResponseInputItemTypeFileSearchCall 文件检索工具调用
	ResponseInputItemTypeFileSearchCall ResponseInputItemType = "file_search_call"
	// ResponseInputItemTypeComputerCall 计算机使用工具调用
	ResponseInputItemTypeComputerCall ResponseInputItemType = "computer_call"
	// ResponseInputItemTypeComputerCallOutput 计算机使用工具调用输出
	ResponseInputItemTypeComputerCallOutput ResponseInputItemType = "computer_call_output"
	// ResponseInputItemTypeWebSearchCall 网络搜索工具调用
	ResponseInputItemTypeWebSearchCall ResponseInputItemType = "web_search_call"
	// ResponseInputItemTypeFunctionCall 函数调用
	ResponseInputItemTypeFunctionCall ResponseInputItemType = "function_call"
	// ResponseInputItemTypeFunctionCallOutput 函数调用输出
	ResponseInputItemTypeFunctionCallOutput ResponseInputItemType = "function_call_output"
	// ResponseInputItemTypeToolSearchCall 工具搜索调用
	ResponseInputItemTypeToolSearchCall ResponseInputItemType = "tool_search_call"
	// ResponseInputItemTypeToolSearchOutput 工具搜索输出
	ResponseInputItemTypeToolSearchOutput ResponseInputItemType = "tool_search_output"
	// ResponseInputItemTypeReasoning 推理内容项
	ResponseInputItemTypeReasoning ResponseInputItemType = "reasoning"
	// ResponseInputItemTypeCompaction 压缩摘要项
	ResponseInputItemTypeCompaction ResponseInputItemType = "compaction"
	// ResponseInputItemTypeImageGenerationCall 图像生成调用
	ResponseInputItemTypeImageGenerationCall ResponseInputItemType = "image_generation_call"
	// ResponseInputItemTypeCodeInterpreterCall 代码解释器调用
	ResponseInputItemTypeCodeInterpreterCall ResponseInputItemType = "code_interpreter_call"
	// ResponseInputItemTypeLocalShellCall 本地 shell 调用
	ResponseInputItemTypeLocalShellCall ResponseInputItemType = "local_shell_call"
	// ResponseInputItemTypeLocalShellCallOutput 本地 shell 调用输出
	ResponseInputItemTypeLocalShellCallOutput ResponseInputItemType = "local_shell_call_output"
	// ResponseInputItemTypeShellCall shell 调用
	ResponseInputItemTypeShellCall ResponseInputItemType = "shell_call"
	// ResponseInputItemTypeShellCallOutput shell 调用输出
	ResponseInputItemTypeShellCallOutput ResponseInputItemType = "shell_call_output"
	// ResponseInputItemTypeApplyPatchCall apply_patch 工具调用
	ResponseInputItemTypeApplyPatchCall ResponseInputItemType = "apply_patch_call"
	// ResponseInputItemTypeApplyPatchCallOutput apply_patch 调用输出
	ResponseInputItemTypeApplyPatchCallOutput ResponseInputItemType = "apply_patch_call_output"
	// ResponseInputItemTypeMcpListTools MCP 工具列表
	ResponseInputItemTypeMcpListTools ResponseInputItemType = "mcp_list_tools"
	// ResponseInputItemTypeMcpApprovalRequest MCP 审批请求
	ResponseInputItemTypeMcpApprovalRequest ResponseInputItemType = "mcp_approval_request"
	// ResponseInputItemTypeMcpApprovalResponse MCP 审批响应
	ResponseInputItemTypeMcpApprovalResponse ResponseInputItemType = "mcp_approval_response"
	// ResponseInputItemTypeMcpCall MCP 工具调用
	ResponseInputItemTypeMcpCall ResponseInputItemType = "mcp_call"
	// ResponseInputItemTypeCustomToolCall 自定义工具调用
	ResponseInputItemTypeCustomToolCall ResponseInputItemType = "custom_tool_call"
	// ResponseInputItemTypeCustomToolCallOutput 自定义工具调用输出
	ResponseInputItemTypeCustomToolCallOutput ResponseInputItemType = "custom_tool_call_output"
	// ResponseInputItemTypeItemReference item 引用
	ResponseInputItemTypeItemReference ResponseInputItemType = "item_reference"
)

// ResponseContentType Response API content part 的 type 枚举
type ResponseContentType = string

const (
	// ResponseContentTypeInputText 输入文本块
	ResponseContentTypeInputText ResponseContentType = "input_text"
	// ResponseContentTypeInputImage 输入图像块
	ResponseContentTypeInputImage ResponseContentType = "input_image"
	// ResponseContentTypeInputFile 输入文件块
	ResponseContentTypeInputFile ResponseContentType = "input_file"
	// ResponseContentTypeOutputText 输出文本块
	ResponseContentTypeOutputText ResponseContentType = "output_text"
	// ResponseContentTypeRefusal 拒答块
	ResponseContentTypeRefusal ResponseContentType = "refusal"
	// ResponseContentTypeSummaryText Reasoning 摘要文本块
	ResponseContentTypeSummaryText ResponseContentType = "summary_text"
	// ResponseContentTypeReasoningText Reasoning 文本块
	ResponseContentTypeReasoningText ResponseContentType = "reasoning_text"
)

// ResponseComputerActionType ComputerCall 动作类型
type ResponseComputerActionType = string

const (
	// ResponseComputerActionTypeClick 单击
	ResponseComputerActionTypeClick ResponseComputerActionType = "click"
	// ResponseComputerActionTypeDoubleClick 双击
	ResponseComputerActionTypeDoubleClick ResponseComputerActionType = "double_click"
	// ResponseComputerActionTypeDrag 拖拽
	ResponseComputerActionTypeDrag ResponseComputerActionType = "drag"
	// ResponseComputerActionTypeKeypress 按键
	ResponseComputerActionTypeKeypress ResponseComputerActionType = "keypress"
	// ResponseComputerActionTypeMove 移动鼠标
	ResponseComputerActionTypeMove ResponseComputerActionType = "move"
	// ResponseComputerActionTypeScreenshot 截图
	ResponseComputerActionTypeScreenshot ResponseComputerActionType = "screenshot"
	// ResponseComputerActionTypeScroll 滚动
	ResponseComputerActionTypeScroll ResponseComputerActionType = "scroll"
	// ResponseComputerActionTypeType 输入文本
	ResponseComputerActionTypeType ResponseComputerActionType = "type"
	// ResponseComputerActionTypeWait 等待
	ResponseComputerActionTypeWait ResponseComputerActionType = "wait"
)

// ResponseWebSearchActionType WebSearchCall 动作类型
type ResponseWebSearchActionType = string

const (
	// ResponseWebSearchActionTypeSearch 查询操作
	ResponseWebSearchActionTypeSearch ResponseWebSearchActionType = "search"
	// ResponseWebSearchActionTypeOpenPage 打开页面
	ResponseWebSearchActionTypeOpenPage ResponseWebSearchActionType = "open_page"
	// ResponseWebSearchActionTypeFindInPage 页面内查找
	ResponseWebSearchActionTypeFindInPage ResponseWebSearchActionType = "find_in_page"
)

// ResponseShellEnvironmentType Shell 环境类型
type ResponseShellEnvironmentType = string

const (
	// ResponseShellEnvironmentTypeContainerAuto 自动创建容器
	ResponseShellEnvironmentTypeContainerAuto ResponseShellEnvironmentType = "container_auto"
	// ResponseShellEnvironmentTypeLocal 本地环境
	ResponseShellEnvironmentTypeLocal ResponseShellEnvironmentType = "local"
	// ResponseShellEnvironmentTypeContainerReference 引用已有容器
	ResponseShellEnvironmentTypeContainerReference ResponseShellEnvironmentType = "container_reference"
)

// ResponseContainerNetworkPolicyType 容器网络策略类型
type ResponseContainerNetworkPolicyType = string

const (
	// ResponseContainerNetworkPolicyTypeDisabled 禁用出站网络
	ResponseContainerNetworkPolicyTypeDisabled ResponseContainerNetworkPolicyType = "disabled"
	// ResponseContainerNetworkPolicyTypeAllowlist 允许列表
	ResponseContainerNetworkPolicyTypeAllowlist ResponseContainerNetworkPolicyType = "allowlist"
)

// ResponseShellSkillType Shell skill 类型
type ResponseShellSkillType = string

const (
	// ResponseShellSkillTypeSkillReference 引用已注册的 skill
	ResponseShellSkillTypeSkillReference ResponseShellSkillType = "skill_reference"
	// ResponseShellSkillTypeInline inline skill
	ResponseShellSkillTypeInline ResponseShellSkillType = "inline"
)

// ResponseShellOutcomeType Shell 调用结果类型
type ResponseShellOutcomeType = string

const (
	// ResponseShellOutcomeTypeTimeout 超时
	ResponseShellOutcomeTypeTimeout ResponseShellOutcomeType = "timeout"
	// ResponseShellOutcomeTypeExit 正常退出
	ResponseShellOutcomeTypeExit ResponseShellOutcomeType = "exit"
)

// ResponseApplyPatchOpType apply_patch 操作类型
type ResponseApplyPatchOpType = string

const (
	// ResponseApplyPatchOpTypeCreateFile 创建文件
	ResponseApplyPatchOpTypeCreateFile ResponseApplyPatchOpType = "create_file"
	// ResponseApplyPatchOpTypeDeleteFile 删除文件
	ResponseApplyPatchOpTypeDeleteFile ResponseApplyPatchOpType = "delete_file"
	// ResponseApplyPatchOpTypeUpdateFile 更新文件
	ResponseApplyPatchOpTypeUpdateFile ResponseApplyPatchOpType = "update_file"
)

// ==================== Response API Tool Types ====================

// ResponseToolType Response API tools 数组元素的 type 枚举
type ResponseToolType = string

const (
	// ResponseToolTypeFunction 函数工具
	ResponseToolTypeFunction ResponseToolType = "function"
	// ResponseToolTypeFileSearch 文件检索工具
	ResponseToolTypeFileSearch ResponseToolType = "file_search"
	// ResponseToolTypeComputer 计算机使用工具
	ResponseToolTypeComputer ResponseToolType = "computer"
	// ResponseToolTypeComputerUsePreview 计算机使用（预览版）
	ResponseToolTypeComputerUsePreview ResponseToolType = "computer_use_preview"
	// ResponseToolTypeWebSearch 网络搜索
	ResponseToolTypeWebSearch ResponseToolType = "web_search"
	// ResponseToolTypeWebSearch20250826 网络搜索（2025-08-26 版）
	ResponseToolTypeWebSearch20250826 ResponseToolType = "web_search_2025_08_26"
	// ResponseToolTypeMcp MCP 工具
	ResponseToolTypeMcp ResponseToolType = "mcp"
	// ResponseToolTypeCodeInterpreter 代码解释器
	ResponseToolTypeCodeInterpreter ResponseToolType = "code_interpreter"
	// ResponseToolTypeImageGeneration 图像生成
	ResponseToolTypeImageGeneration ResponseToolType = "image_generation"
	// ResponseToolTypeLocalShell 本地 shell
	ResponseToolTypeLocalShell ResponseToolType = "local_shell"
	// ResponseToolTypeShell shell
	ResponseToolTypeShell ResponseToolType = "shell"
	// ResponseToolTypeCustom 自定义工具
	ResponseToolTypeCustom ResponseToolType = "custom"
	// ResponseToolTypeNamespace 命名空间工具组
	ResponseToolTypeNamespace ResponseToolType = "namespace"
	// ResponseToolTypeToolSearch 工具搜索
	ResponseToolTypeToolSearch ResponseToolType = "tool_search"
	// ResponseToolTypeWebSearchPreview 网络搜索预览
	ResponseToolTypeWebSearchPreview ResponseToolType = "web_search_preview"
	// ResponseToolTypeWebSearchPreview311 网络搜索预览（2025-03-11 版）
	ResponseToolTypeWebSearchPreview311 ResponseToolType = "web_search_preview_2025_03_11"
	// ResponseToolTypeApplyPatch apply_patch 工具
	ResponseToolTypeApplyPatch ResponseToolType = "apply_patch"
)

// ResponseFileSearchFilterType FileSearch 过滤器类型
type ResponseFileSearchFilterType = string

const (
	// ResponseFileSearchFilterTypeEq 等于
	ResponseFileSearchFilterTypeEq ResponseFileSearchFilterType = "eq"
	// ResponseFileSearchFilterTypeNe 不等于
	ResponseFileSearchFilterTypeNe ResponseFileSearchFilterType = "ne"
	// ResponseFileSearchFilterTypeGt 大于
	ResponseFileSearchFilterTypeGt ResponseFileSearchFilterType = "gt"
	// ResponseFileSearchFilterTypeGte 大于等于
	ResponseFileSearchFilterTypeGte ResponseFileSearchFilterType = "gte"
	// ResponseFileSearchFilterTypeLt 小于
	ResponseFileSearchFilterTypeLt ResponseFileSearchFilterType = "lt"
	// ResponseFileSearchFilterTypeLte 小于等于
	ResponseFileSearchFilterTypeLte ResponseFileSearchFilterType = "lte"
	// ResponseFileSearchFilterTypeIn 在集合中
	ResponseFileSearchFilterTypeIn ResponseFileSearchFilterType = "in"
	// ResponseFileSearchFilterTypeNin 不在集合中
	ResponseFileSearchFilterTypeNin ResponseFileSearchFilterType = "nin"
	// ResponseFileSearchFilterTypeAnd 与组合
	ResponseFileSearchFilterTypeAnd ResponseFileSearchFilterType = "and"
	// ResponseFileSearchFilterTypeOr 或组合
	ResponseFileSearchFilterTypeOr ResponseFileSearchFilterType = "or"
)

// ResponseCustomToolFormatType Custom tool 格式类型
type ResponseCustomToolFormatType = string

const (
	// ResponseCustomToolFormatTypeText 文本格式
	ResponseCustomToolFormatTypeText ResponseCustomToolFormatType = "text"
	// ResponseCustomToolFormatTypeGrammar 语法格式
	ResponseCustomToolFormatTypeGrammar ResponseCustomToolFormatType = "grammar"
)

// ==================== tool_choice ====================

// ResponseToolChoiceOption tool_choice 字符串形态
type ResponseToolChoiceOption = string

const (
	// ResponseToolChoiceOptionNone 禁用工具
	ResponseToolChoiceOptionNone ResponseToolChoiceOption = "none"
	// ResponseToolChoiceOptionAuto 自动选择
	ResponseToolChoiceOptionAuto ResponseToolChoiceOption = "auto"
	// ResponseToolChoiceOptionRequired 强制使用工具
	ResponseToolChoiceOptionRequired ResponseToolChoiceOption = "required"
)

// ResponseToolChoiceType tool_choice 对象形态 type 字段
type ResponseToolChoiceType = string

const (
	// ResponseToolChoiceTypeAllowedTools 限定允许的工具集合
	ResponseToolChoiceTypeAllowedTools ResponseToolChoiceType = "allowed_tools"
	// ResponseToolChoiceTypeFileSearch 强制 file_search
	ResponseToolChoiceTypeFileSearch ResponseToolChoiceType = "file_search"
	// ResponseToolChoiceTypeWebSearchPrv 强制 web_search_preview
	ResponseToolChoiceTypeWebSearchPrv ResponseToolChoiceType = "web_search_preview"
	// ResponseToolChoiceTypeComputer 强制 computer
	ResponseToolChoiceTypeComputer ResponseToolChoiceType = "computer"
	// ResponseToolChoiceTypeComputerUseP 强制 computer_use_preview
	ResponseToolChoiceTypeComputerUseP ResponseToolChoiceType = "computer_use_preview"
	// ResponseToolChoiceTypeComputerUse 强制 computer_use
	ResponseToolChoiceTypeComputerUse ResponseToolChoiceType = "computer_use"
	// ResponseToolChoiceTypeWebSrchPrv11 强制 web_search_preview_2025_03_11
	ResponseToolChoiceTypeWebSrchPrv11 ResponseToolChoiceType = "web_search_preview_2025_03_11"
	// ResponseToolChoiceTypeImageGen 强制 image_generation
	ResponseToolChoiceTypeImageGen ResponseToolChoiceType = "image_generation"
	// ResponseToolChoiceTypeCodeInterp 强制 code_interpreter
	ResponseToolChoiceTypeCodeInterp ResponseToolChoiceType = "code_interpreter"
	// ResponseToolChoiceTypeFunction 强制特定函数
	ResponseToolChoiceTypeFunction ResponseToolChoiceType = "function"
	// ResponseToolChoiceTypeMcp 强制特定 MCP 工具
	ResponseToolChoiceTypeMcp ResponseToolChoiceType = "mcp"
	// ResponseToolChoiceTypeCustom 强制特定自定义工具
	ResponseToolChoiceTypeCustom ResponseToolChoiceType = "custom"
	// ResponseToolChoiceTypeApplyPatch 强制 apply_patch
	ResponseToolChoiceTypeApplyPatch ResponseToolChoiceType = "apply_patch"
	// ResponseToolChoiceTypeShell 强制 shell
	ResponseToolChoiceTypeShell ResponseToolChoiceType = "shell"
)

// ==================== 顶层字段枚举 ====================

// ResponseReasoningSummary reasoning.summary / generate_summary
type ResponseReasoningSummary = string

const (
	// ResponseReasoningSummaryAuto 自动摘要
	ResponseReasoningSummaryAuto ResponseReasoningSummary = "auto"
	// ResponseReasoningSummaryConcise 简明摘要
	ResponseReasoningSummaryConcise ResponseReasoningSummary = "concise"
	// ResponseReasoningSummaryDetailed 详细摘要
	ResponseReasoningSummaryDetailed ResponseReasoningSummary = "detailed"
)

// ResponseTextFormatType text.format.type
type ResponseTextFormatType = string

const (
	// ResponseTextFormatTypeText 文本格式
	ResponseTextFormatTypeText ResponseTextFormatType = "text"
	// ResponseTextFormatTypeJSONSchema JSON Schema 格式
	ResponseTextFormatTypeJSONSchema ResponseTextFormatType = "json_schema"
	// ResponseTextFormatTypeJSONObject JSON 对象格式（已弃用）
	ResponseTextFormatTypeJSONObject ResponseTextFormatType = "json_object"
)

// ResponseContextManagementType context_management[].type
type ResponseContextManagementType = string

const (
	// ResponseContextManagementTypeCompaction 压缩模式
	ResponseContextManagementTypeCompaction ResponseContextManagementType = "compaction"
)

// ResponseStatus Response API 响应生命周期状态
type ResponseStatus = string

const (
	// ResponseStatusInProgress 响应进行中
	ResponseStatusInProgress ResponseStatus = "in_progress"
	// ResponseStatusCompleted 响应成功完成
	ResponseStatusCompleted ResponseStatus = "completed"
	// ResponseStatusFailed 响应失败
	ResponseStatusFailed ResponseStatus = "failed"
	// ResponseStatusIncomplete 响应未完成
	ResponseStatusIncomplete ResponseStatus = "incomplete"
	// ResponseStatusCancelled 响应被取消
	ResponseStatusCancelled ResponseStatus = "cancelled"
	// ResponseStatusQueued 响应排队中
	ResponseStatusQueued ResponseStatus = "queued"
)

// ResponseStreamEventType Response API SSE 事件类型（只枚举网关感知的终态事件）
type ResponseStreamEventType = string

const (
	// ResponseStreamEventCompleted 响应成功完成
	ResponseStreamEventCompleted ResponseStreamEventType = "response.completed"
	// ResponseStreamEventFailed 响应失败
	ResponseStreamEventFailed ResponseStreamEventType = "response.failed"
	// ResponseStreamEventIncomplete 响应未完成
	ResponseStreamEventIncomplete ResponseStreamEventType = "response.incomplete"
)

// ResponseStreamEventDeltaSuffix Response API 承载增量 token 的 SSE 事件后缀
//
// 所有真正携带模型生成内容的流式事件（response.output_text.delta、
// response.reasoning_text.delta、response.function_call_arguments.delta、
// response.audio.delta、response.custom_tool_call_input.delta 等）都以
// `.delta` 结尾；而 response.created / response.in_progress /
// response.output_item.added / response.content_part.added 等元数据事件
// 不带 token。用统一后缀判定可以兼容上游未来新增的 delta 类型。
const ResponseStreamEventDeltaSuffix = ".delta"

// ResponseIncludable include 字段单项枚举
type ResponseIncludable = string

const (
	// ResponseIncludeFileSearchResults 包含 file_search 结果
	ResponseIncludeFileSearchResults ResponseIncludable = "file_search_call.results"
	// ResponseIncludeWebSearchResults 包含 web_search 结果
	ResponseIncludeWebSearchResults ResponseIncludable = "web_search_call.results"
	// ResponseIncludeWebSearchSources 包含 web_search 来源
	ResponseIncludeWebSearchSources ResponseIncludable = "web_search_call.action.sources"
	// ResponseIncludeMessageInputImageURL 包含消息输入图像 URL
	ResponseIncludeMessageInputImageURL ResponseIncludable = "message.input_image.image_url"
	// ResponseIncludeComputerCallOutputImage 包含 computer_call_output 图像 URL
	ResponseIncludeComputerCallOutputImage ResponseIncludable = "computer_call_output.output.image_url"
	// ResponseIncludeCodeInterpreterOutputs 包含 code_interpreter 输出
	ResponseIncludeCodeInterpreterOutputs ResponseIncludable = "code_interpreter_call.outputs"
	// ResponseIncludeReasoningEncryptedContent 包含 reasoning 加密内容
	ResponseIncludeReasoningEncryptedContent ResponseIncludable = "reasoning.encrypted_content"
	// ResponseIncludeMessageOutputTextLogprobs 包含 message.output_text.logprobs
	ResponseIncludeMessageOutputTextLogprobs ResponseIncludable = "message.output_text.logprobs"
)
