package model

// Trace agent 运行观测记录（与 proxy session 正交）
type Trace struct {
	BaseModel
	Agent      string            `json:"agent" gorm:"column:agent;not null;default:'codex';comment:agent 来源"`
	SessionID  string            `json:"session_id" gorm:"column:session_id;not null;uniqueIndex:uniq_trace_session;comment:codex session_id"`
	APIKeyName string            `json:"api_key_name" gorm:"column:api_key_name;not null;default:'';comment:归属 API Key 名称"`
	UserID     uint              `json:"user_id" gorm:"column:user_id;not null;default:0;comment:归属用户"`
	Model      string            `json:"model" gorm:"column:model;not null;default:'';comment:活跃模型 slug"`
	CWD        string            `json:"cwd" gorm:"column:cwd;not null;default:'';comment:工作目录"`
	Source     string            `json:"source" gorm:"column:source;not null;default:'';comment:startup/resume/clear/compact"`
	Status     string            `json:"status" gorm:"column:status;not null;default:'active';comment:active/done"`
	Metadata   map[string]string `json:"metadata" gorm:"column:metadata;serializer:json;comment:扩展字段"`
}

// TraceEvent agent 运行内单个 hook 事件
type TraceEvent struct {
	BaseModel
	TraceID        uint   `json:"trace_id" gorm:"column:trace_id;not null;index:idx_trace_event_trace;comment:关联 trace id"`
	SessionID      string `json:"session_id" gorm:"column:session_id;not null;index:idx_trace_event_session;comment:codex session_id"`
	Source         string `json:"source" gorm:"column:source;not null;default:'hook';comment:记录来源"`
	RecordType     string `json:"record_type" gorm:"column:record_type;not null;default:'hook_event';comment:记录类型"`
	Event          string `json:"event" gorm:"column:event;not null;comment:hook_event_name 或 rollout payload.type"`
	TurnID         string `json:"turn_id" gorm:"column:turn_id;not null;default:'';index:idx_trace_event_turn;comment:codex turn id"`
	CallID         string `json:"call_id" gorm:"column:call_id;not null;default:'';index:idx_trace_event_call;comment:工具调用关联 ID"`
	TranscriptLine *int64 `json:"transcript_line,omitempty" gorm:"column:transcript_line;comment:rollout 原文件行号"`
	ClientSequence int64  `json:"client_sequence" gorm:"column:client_sequence;not null;default:0;comment:客户端序号"`
	DedupKey       string `json:"dedup_key" gorm:"column:dedup_key;not null;default:'';index:uniq_trace_event_dedup,unique,where:dedup_key <> '';comment:幂等键"`
	Payload        []byte `json:"payload" gorm:"column:payload;type:jsonb;comment:完整原始输入（透传）"`
}
