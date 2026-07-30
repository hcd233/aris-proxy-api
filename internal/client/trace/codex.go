package trace

import (
	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func init() {
	registerAdapter(codexAdapter{})
}

type codexAdapter struct{}

type codexHookEnvelope struct {
	HookEventName  string `json:"hook_event_name"`
	SessionID      string `json:"session_id"`
	Model          string `json:"model,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	Source         string `json:"source,omitempty"`
	TurnID         string `json:"turn_id,omitempty"`
	ToolUseID      string `json:"tool_use_id,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`
}

func (codexAdapter) Name() string { return constant.TraceAgentCodex }

func (codexAdapter) ParseHook(raw []byte) (HookInfo, error) {
	var env codexHookEnvelope
	if err := sonic.Unmarshal(raw, &env); err != nil {
		return HookInfo{}, err
	}
	return HookInfo{
		SessionID:      env.SessionID,
		EventName:      env.HookEventName,
		Model:          env.Model,
		CWD:            env.CWD,
		SessionSource:  env.Source,
		TurnID:         env.TurnID,
		CallID:         env.ToolUseID,
		TranscriptPath: env.TranscriptPath,
	}, nil
}

func (codexAdapter) StdoutAck(info HookInfo) string {
	if info.EventName == constant.TraceEventStop {
		return constant.EmptyJSONObject
	}
	return ""
}

type codexRolloutEnvelope struct {
	Type    string                 `json:"type"`
	Payload sonic.NoCopyRawMessage `json:"payload"`
}

type codexRolloutPayload struct {
	Type        string `json:"type"`
	TurnID      string `json:"turn_id,omitempty"`
	CallID      string `json:"call_id,omitempty"`
	Passthrough struct {
		TurnID string `json:"turn_id,omitempty"`
	} `json:"internal_chat_message_metadata_passthrough"`
}

// turnID 优先取顶层 turn_id；response_item 记录的 turn_id 嵌套在
// internal_chat_message_metadata_passthrough 中，顶层缺失时回退读取该字段。
func (p codexRolloutPayload) turnID() string {
	if p.TurnID != "" {
		return p.TurnID
	}
	return p.Passthrough.TurnID
}

func (codexAdapter) ClassifyTranscriptLine(raw []byte) TranscriptMeta {
	var envelope codexRolloutEnvelope
	if err := sonic.Unmarshal(raw, &envelope); err != nil {
		return TranscriptMeta{RecordType: constant.TraceRolloutTypeUnknown, Event: constant.TraceRolloutTypeUnknown}
	}
	var payload codexRolloutPayload
	if len(envelope.Payload) > 0 {
		_ = sonic.Unmarshal(envelope.Payload, &payload) //nolint:errcheck // best-effort field extraction
	}
	return TranscriptMeta{
		RecordType: codexRolloutRecordType(envelope.Type),
		Event:      payload.Type,
		TurnID:     payload.turnID(),
		CallID:     payload.CallID,
	}
}

func codexRolloutRecordType(recordType string) string {
	switch recordType {
	case constant.TraceRolloutTypeSessionMeta,
		constant.TraceRolloutTypeTurnContext,
		constant.TraceRolloutTypeResponseItem,
		constant.TraceRolloutTypeEventMsg:
		return recordType
	default:
		return constant.TraceRolloutTypeUnknown
	}
}
