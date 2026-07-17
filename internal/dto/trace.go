// Package dto Trace DTO
package dto

import (
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// TraceSummary trace 列表项
type TraceSummary struct {
	ID         uint      `json:"id" doc:"Trace ID"`
	SessionID  string    `json:"sessionId" doc:"codex session_id"`
	Agent      string    `json:"agent" doc:"agent 来源"`
	APIKeyName string    `json:"apiKeyName" doc:"归属 API Key"`
	Model      string    `json:"model" doc:"模型"`
	Source     string    `json:"source" doc:"startup/resume/clear/compact"`
	Status     string    `json:"status" doc:"active/done"`
	CreatedAt  time.Time `json:"createdAt" doc:"创建时间"`
	UpdatedAt  time.Time `json:"updatedAt" doc:"更新时间"`
}

// TraceDetail trace 详情
type TraceDetail struct {
	ID         uint              `json:"id" doc:"Trace ID"`
	SessionID  string            `json:"sessionId" doc:"codex session_id"`
	Agent      string            `json:"agent" doc:"agent 来源"`
	APIKeyName string            `json:"apiKeyName" doc:"归属 API Key"`
	Model      string            `json:"model" doc:"模型"`
	CWD        string            `json:"cwd" doc:"工作目录"`
	Source     string            `json:"source" doc:"startup/resume/clear/compact"`
	Status     string            `json:"status" doc:"active/done"`
	Metadata   map[string]string `json:"metadata,omitempty" doc:"扩展字段"`
	EventCount int64             `json:"eventCount" doc:"事件数"`
	CreatedAt  time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt  time.Time         `json:"updatedAt" doc:"更新时间"`
}

// TraceEventItem trace 事件项
type TraceEventItem struct {
	ID        uint                   `json:"id" doc:"事件 ID"`
	Event     string                 `json:"event" doc:"hook 事件名"`
	TurnID    string                 `json:"turnId" doc:"turn id"`
	Payload   sonic.NoCopyRawMessage `json:"payload" doc:"完整 hook 输入"`
	CreatedAt time.Time              `json:"createdAt" doc:"时间"`
}

// ListTracesRsp 列表响应
type ListTracesRsp struct {
	CommonRsp
	Traces   []*TraceSummary `json:"traces,omitempty" doc:"trace 列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ListTracesReq 列表请求（JWT）
type ListTracesReq struct {
	model.PageParam
}

// GetTraceRsp 详情响应
type GetTraceRsp struct {
	CommonRsp
	Trace *TraceDetail `json:"trace,omitempty" doc:"trace 详情"`
}

// GetTraceReq 详情请求（JWT）
type GetTraceReq struct {
	TraceID uint `query:"id" required:"true" minimum:"1" doc:"Trace ID"`
}

// ListTraceEventsRsp 事件时间线响应
type ListTraceEventsRsp struct {
	CommonRsp
	Events   []*TraceEventItem `json:"events,omitempty" doc:"事件列表"`
	PageInfo *model.PageInfo   `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ListTraceEventsReq 事件时间线请求（JWT）
type ListTraceEventsReq struct {
	TraceID uint `query:"id" required:"true" minimum:"1" doc:"Trace ID"`
	model.PageParam
}

// ReportTraceEventRsp 上报响应
type ReportTraceEventRsp struct {
	CommonRsp
}

// ReportTraceEventReq 上报请求（API Key 鉴权，codex hook stdin JSON）
type ReportTraceEventReq struct {
	Body *ReportTraceEventReqBody `json:"body" doc:"codex hook stdin 输入"`
}

// ReportTraceEventReqBody codex hook stdin 输入
//
// 显式建模 codex hook 各事件的字段；任意 JSON 字段（tool_input / tool_response）
// 用 sonic.NoCopyRawMessage 承载。handler 序列化整个结构体作为完整 hook JSON
// 透传存储到 events.payload。
type ReportTraceEventReqBody struct {
	// 公共字段（所有 hook 事件均携带）
	HookEventName  string `json:"hook_event_name" required:"true" minLength:"1" doc:"hook 事件名"`
	SessionID      string `json:"session_id" required:"true" minLength:"1" doc:"codex session_id"`
	Model          string `json:"model,omitempty" doc:"模型"`
	CWD            string `json:"cwd,omitempty" doc:"工作目录"`
	TranscriptPath string `json:"transcript_path,omitempty" doc:"transcript 路径"`
	PermissionMode string `json:"permission_mode,omitempty" doc:"权限模式"`
	// turn 级事件携带
	TurnID string `json:"turn_id,omitempty" doc:"turn id"`
	// SessionStart
	Source string `json:"source,omitempty" doc:"startup/resume/clear/compact"`
	// UserPromptSubmit
	Prompt string `json:"prompt,omitempty" doc:"用户输入文本"`
	// PreToolUse / PostToolUse
	ToolName     string                 `json:"tool_name,omitempty" doc:"工具名"`
	ToolUseID    string                 `json:"tool_use_id,omitempty" doc:"工具调用 ID"`
	ToolInput    sonic.NoCopyRawMessage `json:"tool_input,omitempty" doc:"工具输入（任意 JSON）"`
	ToolResponse sonic.NoCopyRawMessage `json:"tool_response,omitempty" doc:"工具响应（任意 JSON）"`
	// Stop / SubagentStop
	LastAssistantMessage string `json:"last_assistant_message,omitempty" doc:"最后 assistant 消息"`
	// SubagentStart / SubagentStop
	AgentID   string `json:"agent_id,omitempty" doc:"subagent ID"`
	AgentType string `json:"agent_type,omitempty" doc:"subagent 类型"`
	// PreCompact / PostCompact
	Trigger string `json:"trigger,omitempty" doc:"manual/auto"`
}
