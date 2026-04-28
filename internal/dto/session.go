// Package dto Session DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// SessionSummary Session列表项摘要
//
//	@author centonhuang
//	@update 2026-04-25 15:00:00
type SessionSummary struct {
	ID         uint              `json:"id" doc:"Session ID"`
	CreatedAt  time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt  time.Time         `json:"updatedAt" doc:"更新时间"`
	Summary    string            `json:"summary" doc:"会话总结"`
	MessageIDs []uint            `json:"messageIds" doc:"消息ID列表"`
	ToolIDs    []uint            `json:"toolIds" doc:"工具ID列表"`
	Metadata   map[string]string `json:"metadata,omitempty" doc:"请求元数据"`
}

// SessionDetail Session详情
//
//	@author centonhuang
//	@update 2026-04-25 15:00:00
type SessionDetail struct {
	ID         uint              `json:"id" doc:"Session ID"`
	APIKeyName string            `json:"apiKeyName" doc:"API密钥名称"`
	CreatedAt  time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt  time.Time         `json:"updatedAt" doc:"更新时间"`
	Metadata   map[string]string `json:"metadata,omitempty" doc:"请求元数据"`
	Messages   []*MessageItem    `json:"messages" doc:"消息列表"`
	Tools      []*ToolItem       `json:"tools" doc:"工具列表"`
}

// MessageItem 消息列表项
//
//	@author centonhuang
//	@update 2026-04-25 15:00:00
type MessageItem struct {
	ID        uint               `json:"id" doc:"消息ID"`
	Model     string             `json:"model" doc:"模型名称"`
	Message   *vo.UnifiedMessage `json:"message" doc:"消息内容"`
	CreatedAt time.Time          `json:"createdAt" doc:"创建时间"`
}

// ToolItem 工具列表项
//
//	@author centonhuang
//	@update 2026-04-25 15:00:00
type ToolItem struct {
	ID        uint            `json:"id" doc:"工具ID"`
	Tool      *vo.UnifiedTool `json:"tool" doc:"工具内容"`
	CreatedAt time.Time       `json:"createdAt" doc:"创建时间"`
}

// ListSessionsReq 分页获取Session列表请求
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type ListSessionsReq struct {
	model.PageParam
}

// ListSessionsRsp 分页获取Session列表响应
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type ListSessionsRsp struct {
	CommonRsp
	Sessions []*SessionSummary `json:"sessions,omitempty" doc:"Session列表"`
	PageInfo *model.PageInfo   `json:"pageInfo,omitempty" doc:"分页信息"`
}

// GetSessionReq 获取Session详情请求
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type GetSessionReq struct {
	SessionID uint `query:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
}

// GetSessionRsp 获取Session详情响应
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type GetSessionRsp struct {
	CommonRsp
	Session *SessionDetail `json:"session,omitempty" doc:"Session详情"`
}
