package trace

import "github.com/hcd233/aris-proxy-api/internal/common/ierr"

// HookInfo 是各 agent hook stdin JSON 的归一化身份视图。
type HookInfo struct {
	SessionID      string
	EventName      string
	Model          string
	CWD            string
	SessionSource  string
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
