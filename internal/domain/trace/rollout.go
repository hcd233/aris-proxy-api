package trace

import (
	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// RolloutRecord 是 Codex rollout JSONL 的轻量解析结果。
type RolloutRecord struct {
	RecordType string
	Event      string
	TurnID     string
	CallID     string
	Arguments  string
	Raw        []byte
	Unknown    bool
}

// ParseRolloutRecord 解析一条 rollout envelope，未知类型保留原始数据。
func ParseRolloutRecord(raw []byte) (RolloutRecord, error) {
	var envelope map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(raw, &envelope); err != nil {
		return RolloutRecord{}, err
	}
	record := RolloutRecord{Raw: append([]byte(nil), raw...)}
	if err := sonic.Unmarshal(envelope["type"], &record.RecordType); err != nil {
		return RolloutRecord{}, err
	}

	knownTypes := map[string]bool{
		constant.TraceRolloutTypeSessionMeta:  true,
		constant.TraceRolloutTypeTurnContext:  true,
		constant.TraceRolloutTypeResponseItem: true,
		constant.TraceRolloutTypeEventMsg:     true,
	}
	if !knownTypes[record.RecordType] {
		record.Unknown = true
		record.Event = record.RecordType
		return record, nil
	}
	var payload map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(envelope["payload"], &payload); err != nil {
		return record, err
	}
	if value := payload[constant.TracePayloadFieldType]; len(value) > 0 {
		if err := sonic.Unmarshal(value, &record.Event); err != nil {
			return record, err
		}
	}
	if value := payload[constant.TracePayloadFieldTurnID]; len(value) > 0 {
		if err := sonic.Unmarshal(value, &record.TurnID); err != nil {
			return record, err
		}
	}
	if value := payload[constant.TracePayloadFieldCallID]; len(value) > 0 {
		if err := sonic.Unmarshal(value, &record.CallID); err != nil {
			return record, err
		}
	}
	if record.Event == constant.TraceConversationEventFunctionCall {
		if err := sonic.Unmarshal(payload[constant.TracePayloadFieldArguments], &record.Arguments); err != nil {
			return record, err
		}
	}
	return record, nil
}
