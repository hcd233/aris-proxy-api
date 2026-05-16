package constant

const (
	UpstreamPathOpenAIChatCompletions = "/chat/completions"
	UpstreamPathOpenAIResponses       = "/responses"
	UpstreamPathAnthropicMessages     = "/messages"
	UpstreamPathAnthropicCountTokens  = "/messages/count_tokens"

	AnthropicAPIVersion = "2023-06-01"

	AnthropicMessageIDTemplate = "msg_%s"
	OpenAIChunkIDTemplate      = "chatcmpl-%s"
	ConvertedChunkIDSuffix     = "converted"
	ResponseStreamFieldType    = "type"
	ResponseStreamFieldDelta   = "delta"

	OpenAIInvalidRequestErrorType      = "invalid_request_error"
	OpenAIModelNotFoundCode            = "model_not_found"
	OpenAIModelNotFoundMessageTemplate = "The model `%s` does not exist"
	OpenAIInternalErrorShortMessage    = "Internal error"

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
	OpenAIInternalErrorMessage         = "Internal server error"
	OpenAIInternalErrorType            = "server_error"
	OpenAIInternalErrorCode            = "internal_error"

	AnthropicNotFoundErrorType            = "not_found_error"
	AnthropicModelNotFoundMessageTemplate = "model: %s"
	AnthropicInternalErrorMessage         = "Internal server error"
	AnthropicInternalErrorType            = "api_error"
	AnthropicInternalErrorBodyType        = "error"

	UpstreamErrorType             = "upstream_error"
	UpstreamStatusMessageTemplate = "Upstream returned status %d"

	CallStatusSuccess         = 200
	CallStatusConnectionError = -1
	CallStatusUnknownError    = 0

	ReasoningContentPlaceholder = " "

	ResponseFailedAuditReason             = "response.failed"
	ResponseFailedAuditReasonTemplate     = "response.failed: %s"
	ResponseIncompleteAuditReason         = "response.incomplete"
	ResponseIncompleteAuditReasonTemplate = "response.incomplete: %s"
)
