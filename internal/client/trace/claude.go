package trace

import (
	"bytes"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func init() {
	registerAdapter(claudeAdapter{})
}

type claudeAdapter struct{}

type claudeHookEnvelope struct {
	HookEventName  string `json:"hook_event_name"`
	SessionID      string `json:"session_id"`
	Model          string `json:"model,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	Source         string `json:"source,omitempty"`
	PromptID       string `json:"prompt_id,omitempty"`
	ToolUseID      string `json:"tool_use_id,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`
}

func (claudeAdapter) Name() string { return constant.TraceAgentClaude }

func (claudeAdapter) ParseHook(raw []byte) (HookInfo, error) {
	var env claudeHookEnvelope
	if err := sonic.Unmarshal(raw, &env); err != nil {
		return HookInfo{}, err
	}
	return HookInfo{
		SessionID:      env.SessionID,
		EventName:      env.HookEventName,
		Model:          env.Model,
		CWD:            env.CWD,
		SessionSource:  env.Source,
		TurnID:         env.PromptID,
		CallID:         env.ToolUseID,
		TranscriptPath: env.TranscriptPath,
	}, nil
}

// StdoutAck claude hook 的 stdout 会被注入上下文（SessionStart/UserPromptSubmit），必须恒静默。
func (claudeAdapter) StdoutAck(HookInfo) string { return "" }

type claudeTranscriptMessage struct {
	Content sonic.NoCopyRawMessage `json:"content"`
}

type claudeTranscriptRecord struct {
	Type    string                   `json:"type"`
	Subtype string                   `json:"subtype"`
	Message *claudeTranscriptMessage `json:"message"`
}

type claudeBlockIdentity struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	ToolUseID string `json:"tool_use_id"`
}

// claudePassthroughRecordTypes 记录级 type 原样透传的白名单；其余未识别 type 标记 unknown。
var claudePassthroughRecordTypes = map[string]bool{
	constant.TraceClaudeRecordPermissionMode:      true,
	constant.TraceClaudeRecordFileHistorySnapshot: true,
	constant.TraceClaudeRecordLastPrompt:          true,
	constant.TraceClaudeRecordSummary:             true,
	constant.TraceClaudeRecordProgress:            true,
}

func (claudeAdapter) ClassifyTranscriptLine(raw []byte) TranscriptMeta {
	var record claudeTranscriptRecord
	if err := sonic.Unmarshal(raw, &record); err != nil {
		return TranscriptMeta{RecordType: constant.TraceRolloutTypeUnknown, Event: constant.TraceRolloutTypeUnknown}
	}
	switch record.Type {
	case constant.TraceClaudeRecordUser:
		return classifyClaudeUserRecord(record.Message)
	case constant.TraceClaudeRecordAssistant:
		return classifyClaudeAssistantRecord(record.Message)
	case constant.TraceClaudeRecordSystem:
		event := record.Subtype
		if event == "" {
			event = constant.TraceClaudeRecordSystem
		}
		return TranscriptMeta{RecordType: constant.TraceClaudeRecordSystem, Event: event}
	case constant.TraceClaudeRecordAttachment:
		return TranscriptMeta{
			RecordType: constant.TraceClaudeRecordAttachment,
			Event:      constant.TraceClaudeRecordAttachment,
		}
	default:
		if claudePassthroughRecordTypes[record.Type] {
			return TranscriptMeta{RecordType: record.Type, Event: record.Type}
		}
		return TranscriptMeta{RecordType: constant.TraceRolloutTypeUnknown, Event: record.Type}
	}
}

// classifyClaudeUserRecord content 为字符串 → 真实用户输入；为数组 → 工具结果回传。
func classifyClaudeUserRecord(message *claudeTranscriptMessage) TranscriptMeta {
	meta := TranscriptMeta{RecordType: constant.TraceClaudeRecordUser, Event: constant.TraceClaudeEventUserPrompt}
	if message == nil || len(message.Content) == 0 {
		return meta
	}
	if bytes.HasPrefix(bytes.TrimSpace(message.Content), []byte{'['}) {
		meta.Event = constant.TraceClaudeEventToolResult
		var blocks []claudeBlockIdentity
		if err := sonic.Unmarshal(message.Content, &blocks); err == nil {
			for _, block := range blocks {
				if block.ToolUseID != "" {
					meta.CallID = block.ToolUseID
					break
				}
			}
		}
	}
	return meta
}

func classifyClaudeAssistantRecord(message *claudeTranscriptMessage) TranscriptMeta {
	meta := TranscriptMeta{
		RecordType: constant.TraceClaudeRecordAssistant,
		Event:      constant.TraceClaudeEventAssistantMessage,
	}
	if message == nil {
		return meta
	}
	var blocks []claudeBlockIdentity
	if err := sonic.Unmarshal(message.Content, &blocks); err == nil {
		for _, block := range blocks {
			if block.Type == constant.TraceClaudeBlockToolUse && block.ID != "" {
				meta.CallID = block.ID
				break
			}
		}
	}
	return meta
}
