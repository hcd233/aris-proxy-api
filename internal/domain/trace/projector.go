package trace

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// conversationBuilders 按 agent 注册的对话投影构造器。新 agent 接入在此登记。
var conversationBuilders = map[string]func([]*TraceEvent) *Conversation{
	constant.TraceAgentCodex:  buildCodexConversation,
	constant.TraceAgentClaude: buildClaudeConversation,
}

// BuildConversationFor 按 agent 分发对话投影构造。
func BuildConversationFor(agent string, events []*TraceEvent) (*Conversation, error) {
	builder, ok := conversationBuilders[agent]
	if !ok {
		return nil, ierr.New(ierr.ErrValidation, "unknown trace agent")
	}
	return builder(events), nil
}
