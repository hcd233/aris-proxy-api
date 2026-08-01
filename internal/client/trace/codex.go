package trace

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func init() {
	registerAdapter(codexAdapter{})
}

type codexAdapter struct{}

type codexHookEnvelope struct {
	HookEventName       string `json:"hook_event_name"`
	SessionID           string `json:"session_id"`
	Model               string `json:"model,omitempty"`
	CWD                 string `json:"cwd,omitempty"`
	Source              string `json:"source,omitempty"`
	TurnID              string `json:"turn_id,omitempty"`
	ToolUseID           string `json:"tool_use_id,omitempty"`
	TranscriptPath      string `json:"transcript_path,omitempty"`
	AgentTranscriptPath string `json:"agent_transcript_path,omitempty"`
	AgentID             string `json:"agent_id,omitempty"`
	AgentType           string `json:"agent_type,omitempty"`
}

func (codexAdapter) Name() string { return constant.TraceAgentCodex }

var rolloutSessionIDPattern = regexp.MustCompile(`^rollout-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-([0-9a-fA-F-]+)$`)

type codexSessionMetaID struct {
	ID string `json:"id"`
}

// SubagentSessionIDFromPath 从子代理 transcript 文件路径解析子代理 session id。
// 优先按文件名 rollout-<ts>-<id>.jsonl 提取；文件名无法解析时回退读取首行 session_meta.payload.id。
func SubagentSessionIDFromPath(path string) string {
	if path == "" {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(path), constant.TraceClientRolloutFileSuffix)
	if m := rolloutSessionIDPattern.FindStringSubmatch(base); len(m) == 2 {
		return m[1]
	}
	return subagentSessionIDFromFile(path)
}

func subagentSessionIDFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // best-effort close
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var env codexRolloutEnvelope
		if err := sonic.Unmarshal(scanner.Bytes(), &env); err != nil {
			continue
		}
		if env.Type != constant.TraceRolloutTypeSessionMeta || len(env.Payload) == 0 {
			continue
		}
		var meta codexSessionMetaID
		if err := sonic.Unmarshal(env.Payload, &meta); err != nil || meta.ID == "" {
			continue
		}
		return meta.ID
	}
	return ""
}

func (codexAdapter) ParseHook(raw []byte) (HookInfo, error) {
	var env codexHookEnvelope
	if err := sonic.Unmarshal(raw, &env); err != nil {
		return HookInfo{}, err
	}
	return HookInfo{
		SessionID:           env.SessionID,
		EventName:           env.HookEventName,
		Model:               env.Model,
		CWD:                 env.CWD,
		SessionSource:       env.Source,
		TurnID:              env.TurnID,
		CallID:              env.ToolUseID,
		TranscriptPath:      env.TranscriptPath,
		AgentTranscriptPath: env.AgentTranscriptPath,
		AgentID:             env.AgentID,
		AgentType:           env.AgentType,
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
	ID          string `json:"id,omitempty"`
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
		SessionID:  payload.ID,
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

// codexEventMsgWhitelist event_msg 记录不丢弃的白名单。task_complete/task_started 为任务
// 生命周期标记；token_count 每条都带累计统计，上报后由服务端按固定 dedup key 覆盖写入
// （RolloutDedupKey），库里只留最后一条。其余 event_msg（agent_message/agent_reasoning/
// user_message/thread_settings_applied/world_state 等）与 response_item 双源重复或纯噪音，
// 客户端直接丢弃。
var codexEventMsgWhitelist = map[string]bool{
	constant.TraceEventTaskStarted:  true,
	constant.TraceEventTaskComplete: true,
	constant.TraceEventTokenCount:   true,
}

func (codexAdapter) IgnoreTranscriptLine(meta TranscriptMeta) bool {
	switch meta.RecordType {
	case constant.TraceRolloutTypeTurnContext:
		return true // 无消费者，model/cwd/turn_id 已被 traces 表与 response_item 覆盖
	case constant.TraceRolloutTypeEventMsg:
		return !codexEventMsgWhitelist[meta.Event]
	default:
		return false
	}
}
