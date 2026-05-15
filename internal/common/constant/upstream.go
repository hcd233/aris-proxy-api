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
