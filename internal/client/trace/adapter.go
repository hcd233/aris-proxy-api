package trace

import (
	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// HookInfo 是各 agent hook stdin JSON 的归一化身份视图。
type HookInfo struct {
	SessionID      string
	EventName      string
	Model          string
	CWD            string
	TurnID         string // codex: turn_id；claude: prompt_id
	CallID         string // 工具调用关联 ID（tool_use_id）
	TranscriptPath string
	// 以下仅 SubagentStop hook 输入携带
	AgentTranscriptPath string // 子代理 transcript 文件路径
	AgentID             string // 子代理 id
	AgentType           string // 子代理类型
}

// TranscriptMeta 是单行 transcript/rollout 记录的归一化分类结果。
type TranscriptMeta struct {
	RecordType string
	Event      string
	TurnID     string
	CallID     string
	SessionID  string // session_meta 的 payload.id（用于稳定 dedup key）
}

// AgentAdapter 抹平不同 agent CLI 的 hook 与 transcript 格式差异。
// 新 agent 接入：实现本接口并在 init() 中 registerAdapter。
type AgentAdapter interface {
	Name() string
	// ParseHook 解析 hook stdin JSON；结构非法时返回 error（调用方 fail-open）。
	ParseHook(raw []byte) (HookInfo, error)
	// ClassifyTranscriptLine 分类单行 transcript；不返回 error，未知类型标记 unknown。
	ClassifyTranscriptLine(raw []byte) TranscriptMeta
	// StdoutAck 返回该 hook 事件需要回显的 stdout 内容；空串 = 静默。
	StdoutAck(info HookInfo) string
	// IgnoreTranscriptLine 返回 true 表示该行 transcript 记录不采集（不进 spool、
	// 不上报、不入库）。codex 用于 event_msg 白名单过滤与 turn_context 丢弃；
	// claude 恒返回 false（保持现状）。
	IgnoreTranscriptLine(meta TranscriptMeta) bool
}

var agentAdapters = map[string]AgentAdapter{}

func registerAdapter(adapter AgentAdapter) {
	agentAdapters[adapter.Name()] = adapter
}

// LookupAdapter 按名称查找 adapter；未知名称返回 error（调用方写本地日志并 fail-open）。
func LookupAdapter(name string) (AgentAdapter, error) {
	if adapter, ok := agentAdapters[name]; ok {
		return adapter, nil
	}
	return nil, ierr.New(ierr.ErrValidation, "unknown trace agent")
}

// TrimStopHookPayload 删除 Stop hook 输入中的 last_assistant_message 键
// （与 rollout 的 agent_message 重复，裁剪避免双源冗余）；其余字段原样保留。
// 输入非 JSON 或不含目标键时原样返回。
func TrimStopHookPayload(raw []byte) []byte {
	var root map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(raw, &root); err != nil {
		return raw
	}
	if _, ok := root[constant.ArisClientStopTrimKey]; !ok {
		return raw
	}
	delete(root, constant.ArisClientStopTrimKey)
	trimmed, err := sonic.Marshal(root)
	if err != nil {
		return raw
	}
	return trimmed
}
