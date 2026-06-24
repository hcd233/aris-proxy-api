package constant

const (
	UpstreamPathOpenAIChatCompletions = "/chat/completions"
	UpstreamPathOpenAIResponses       = "/responses"
	UpstreamPathAnthropicMessages     = "/messages"
	UpstreamPathAnthropicCountTokens  = "/messages/count_tokens"

	AnthropicAPIVersion = "2023-06-01"

	AnthropicMessageIDTemplate          = "msg_%s"
	OpenAIChunkIDTemplate               = "chatcmpl-%s"
	ConvertedChunkIDSuffix              = "converted"
	ResponseIDTemplate                  = "resp_%s"
	ResponseItemIDTemplate              = "msg_%s"
	ResponseStreamFieldType             = "type"
	ResponseStreamFieldDelta            = "delta"
	ResponseStreamFieldResponse         = "response"
	ResponseStreamFieldObject           = "object"
	ResponseStreamFieldModel            = "model"
	ResponseStreamFieldStatus           = "status"
	ResponseStreamFieldID               = "id"
	ResponseStreamFieldItemID           = "item_id"
	ResponseStreamFieldOutputIndex      = "output_index"
	ResponseStreamFieldContentIndex     = "content_index"
	ResponseStreamFieldOutputItem       = "output_index"
	ResponseStreamFieldItem             = "item"
	ResponseStreamFieldPart             = "part"
	ResponseStreamFieldText             = "text"
	ResponseStreamFieldAnnotations      = "annotations"
	ResponseStreamFieldRole             = "role"
	ResponseStreamFieldContent          = "content"
	ResponseStreamFieldTypeValue        = "message"
	ResponseStreamFieldStatusInProgress = "in_progress"
	ResponseStreamFieldStatusCompleted  = "completed"
	ResponseStreamFieldOutputTextType   = "output_text"
	ResponseStreamFieldSummary          = "summary"
	ResponseStreamFieldCallID           = "call_id"
	ResponseStreamFieldName             = "name"
	ResponseStreamFieldArguments        = "arguments"
	ResponseStreamFieldInput            = "input"
	OpenAIInvalidRequestErrorType       = "invalid_request_error"
	OpenAIModelNotFoundCode             = "model_not_found"
	OpenAIModelNotFoundMessageTemplate  = "The model `%s` does not exist"
	OpenAIInternalErrorShortMessage     = "Internal error"

	// ChatCompletionConvertToolDescFileSearch FileSearch 工具转换描述
	ChatCompletionConvertToolDescFileSearch = "File search tool for retrieving information from vector stores"
	// ChatCompletionConvertToolDescWebSearch WebSearch 工具转换描述
	ChatCompletionConvertToolDescWebSearch = "Web search tool for retrieving information from the internet"
	// ChatCompletionConvertToolDescWebSearchPreview WebSearchPreview 工具转换描述
	ChatCompletionConvertToolDescWebSearchPreview = "Web search preview tool"
	// ChatCompletionConvertToolDescComputer Computer 工具转换描述
	ChatCompletionConvertToolDescComputer = "Computer use tool for desktop automation"
	// ChatCompletionConvertToolDescComputerPreview ComputerUsePreview 工具转换描述
	ChatCompletionConvertToolDescComputerPreview = "Computer use preview tool"
	// ChatCompletionConvertToolDescMCPTemplate MCP 工具转换描述模板
	ChatCompletionConvertToolDescMCPTemplate = "MCP tool: %s"
	// ChatCompletionConvertToolDescCodeInterpreter CodeInterpreter 工具转换描述
	ChatCompletionConvertToolDescCodeInterpreter = "Code interpreter tool for executing code"
	// ChatCompletionConvertToolDescImageGeneration ImageGeneration 工具转换描述
	ChatCompletionConvertToolDescImageGeneration = "Image generation tool"
	// ChatCompletionConvertToolDescLocalShell LocalShell 工具转换描述
	ChatCompletionConvertToolDescLocalShell = "Local shell tool"
	// ChatCompletionConvertToolDescShell Shell 工具转换描述
	ChatCompletionConvertToolDescShell = "Shell tool"
	// ChatCompletionConvertToolDescApplyPatch ApplyPatch 工具转换描述
	ChatCompletionConvertToolDescApplyPatch = "Apply patch tool"
	OpenAIInternalErrorMessage              = "Internal server error"
	OpenAIInternalErrorType                 = "server_error"
	OpenAIInternalErrorCode                 = "internal_error"

	AnthropicNotFoundErrorType            = "not_found_error"
	AnthropicModelNotFoundMessageTemplate = "model: %s"
	AnthropicInternalErrorMessage         = "Internal server error"
	AnthropicInternalErrorType            = "api_error"
	AnthropicInternalErrorBodyType        = "error"

	UpstreamErrorType             = "upstream_error"
	UpstreamStatusMessageTemplate = "Upstream returned status %d"

	// UpstreamRetryableStatusThreshold 可重试的上游 HTTP 状态码阈值（>= 此值为 5xx 瞬时错误）
	UpstreamRetryableStatusThreshold = 500

	// ModuleOpenAIProxy OpenAI 代理模块名（用于日志前缀和重试模块标识）
	ModuleOpenAIProxy = "OpenAIProxy"
	// ModuleAnthropicProxy Anthropic 代理模块名（用于日志前缀和重试模块标识）
	ModuleAnthropicProxy = "AnthropicProxy"

	ResponseFailedAuditReason             = "response.failed"
	ResponseFailedAuditReasonTemplate     = "response.failed: %s"
	ResponseIncompleteAuditReason         = "response.incomplete"
	ResponseIncompleteAuditReasonTemplate = "response.incomplete: %s"
	ResponseStreamFieldCreatedAt          = "created_at"
	ResponseStreamFieldOutput             = "output"
)

var ResponseStreamFieldAnnotationsEmpty = []any{}
