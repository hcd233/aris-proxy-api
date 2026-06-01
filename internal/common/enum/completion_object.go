package enum

type CompletionObject = string

const (
	CompletionObjectChatCompletion      CompletionObject = "chat.completion"
	CompletionObjectChatCompletionChunk CompletionObject = "chat.completion.chunk"
	CompletionObjectResponse            CompletionObject = "response"
)
