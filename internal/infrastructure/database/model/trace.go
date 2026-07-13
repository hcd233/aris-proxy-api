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
	TraceID   uint   `json:"trace_id" gorm:"column:trace_id;not null;index:idx_trace_event_trace;comment:关联 trace id"`
	SessionID string `json:"session_id" gorm:"column:session_id;not null;index:idx_trace_event_session;comment:codex session_id"`
	Event     string `json:"event" gorm:"column:event;not null;comment:hook_event_name"`
	TurnID    string `json:"turn_id" gorm:"column:turn_id;not null;default:'';comment:codex turn id"`
	Payload   []byte `json:"payload" gorm:"column:payload;type:jsonb;comment:完整 hook 输入（透传）"`
}
