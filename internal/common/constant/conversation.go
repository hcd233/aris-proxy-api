package constant

const (
	ThinkTagOpen          = "<think>"
	ThinkTagRegexpPattern = `(?s)<think>(.*?)</think>`

	MessageFormatRole           = "Role: %s"
	MessageFormatName           = "Name: %s"
	MessageFormatContent        = "Content: %s"
	MessageFormatContentText    = "Content[text]: %s"
	MessageFormatContentImage   = "Content[image]: %s"
	MessageFormatContentAudio   = "Content[audio]: %s"
	MessageFormatContentFile    = "Content[file]: %s"
	MessageFormatContentRefusal = "Content[refusal]: %s"
	MessageFormatReasoning      = "Reasoning: %s"
	MessageFormatToolCall       = "ToolCall: %s(%s)"
	MessageFormatToolCallID     = "ToolCallID: %s"
	MessageFormatRefusal        = "Refusal: %s"
	MessageContentSeparator     = " | "
)
