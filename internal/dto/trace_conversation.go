package dto

// TraceConversation Trace 对话投影。
type TraceConversation struct {
	TraceID   uint                     `json:"traceId" doc:"Trace ID"`
	SessionID string                   `json:"sessionId" doc:"Codex session ID"`
	Turns     []*TraceConversationTurn `json:"turns" doc:"turn 列表"`
}

// TraceConversationTurn 一个 Codex turn。
type TraceConversationTurn struct {
	TurnID string                   `json:"turnId" doc:"turn ID"`
	Items  []*TraceConversationItem `json:"items" doc:"对话项"`
}

// TraceConversationItem 对话项。
type TraceConversationItem struct {
	Kind      string `json:"kind" doc:"message/tool_call"`
	Role      string `json:"role,omitempty" doc:"角色"`
	Content   string `json:"content,omitempty" doc:"消息内容"`
	ToolName  string `json:"toolName,omitempty" doc:"工具名"`
	CallID    string `json:"callId,omitempty" doc:"工具调用 ID"`
	Arguments string `json:"arguments,omitempty" doc:"工具参数"`
	Output    string `json:"output,omitempty" doc:"工具输出"`
	Source    string `json:"source" doc:"数据来源"`
	RecordIDs []uint `json:"recordIds" doc:"原始记录 ID"`
}

// GetTraceConversationRsp Trace 对话响应。
type GetTraceConversationRsp struct {
	CommonRsp
	Conversation *TraceConversation `json:"conversation,omitempty" doc:"Trace 对话"`
}

// GetTraceConversationReq Trace 对话请求。
type GetTraceConversationReq struct {
	TraceID uint `query:"id" required:"true" minimum:"1" doc:"Trace ID"`
}
