// Package dto Session DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// SessionSummary Session列表项摘要
//
//	@author centonhuang
//	@update 2026-04-25 15:00:00
type SessionSummary struct {
	ID           uint              `json:"id" doc:"Session ID"`
	CreatedAt    time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt    time.Time         `json:"updatedAt" doc:"更新时间"`
	Summary      string            `json:"summary" doc:"会话总结"`
	MessageCount int               `json:"messageCount" doc:"消息数量"`
	ToolCount    int               `json:"toolCount" doc:"工具数量"`
	Metadata     map[string]string `json:"metadata,omitempty" doc:"请求元数据"`
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
	ShareID    string            `json:"shareID" doc:"分享ID（已分享时非空）"`
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

// ListSessionsRsp 分页获取Session列表响应
//
//	@author centonhuang
//	@update 2026-03-19 10:00:00
type ListSessionsRsp struct {
	CommonRsp
	Sessions []*SessionSummary `json:"sessions,omitempty" doc:"Session列表"`
	PageInfo *model.PageInfo   `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ListSessionsByUserReq 分页获取当前用户Session列表请求（JWT认证）
//
//	@author centonhuang
//	@update 2026-06-01 10:00:00
type ListSessionsByUserReq struct {
	model.PageParam
	Sort      enum.Sort `query:"sort" enum:"asc,desc"`
	SortField string    `query:"sortField" maxLength:"50"`
	StartTime time.Time `query:"startTime"`
	EndTime   time.Time `query:"endTime"`
}

// GetSessionByUserReq 获取当前用户Session详情请求（JWT认证）
//
//	@author centonhuang
//	@update 2026-05-24 10:00:00
type GetSessionByUserReq struct {
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

// SessionMetadata Session 元数据（不含 messages/tools 内容）
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type SessionMetadata struct {
	ID           uint              `json:"id" doc:"Session ID"`
	APIKeyName   string            `json:"apiKeyName" doc:"API密钥名称"`
	CreatedAt    time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt    time.Time         `json:"updatedAt" doc:"更新时间"`
	Metadata     map[string]string `json:"metadata,omitempty" doc:"请求元数据"`
	MessageCount int               `json:"messageCount" doc:"消息总数"`
	ToolCount    int               `json:"toolCount" doc:"工具总数"`
	ShareID      string            `json:"shareID" doc:"分享ID（已分享时非空）"`
}

// GetSessionMetadataReq 获取 Session 元数据请求（JWT 认证）
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type GetSessionMetadataReq struct {
	SessionID uint `query:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
}

// GetSessionMetadataRsp 获取 Session 元数据响应
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type GetSessionMetadataRsp struct {
	CommonRsp
	Session *SessionMetadata `json:"session,omitempty" doc:"Session 元数据"`
}

// ListSessionMessagesReq 分页获取 Session 消息请求
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListSessionMessagesReq struct {
	SessionID uint `query:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
	Page      int  `query:"page" required:"true" minimum:"1" doc:"页码"`
	PageSize  int  `query:"pageSize" required:"true" minimum:"1" maximum:"200" default:"50" doc:"每页条数"`
}

// ListSessionMessagesRsp 分页获取 Session 消息响应
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListSessionMessagesRsp struct {
	CommonRsp
	Messages []*MessageItem  `json:"messages,omitempty" doc:"消息列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ListSessionToolsReq 分页获取 Session 工具请求
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListSessionToolsReq struct {
	SessionID uint `query:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
	Page      int  `query:"page" required:"true" minimum:"1" doc:"页码"`
	PageSize  int  `query:"pageSize" required:"true" minimum:"1" maximum:"200" default:"20" doc:"每页条数"`
}

// ListSessionToolsRsp 分页获取 Session 工具响应
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListSessionToolsRsp struct {
	CommonRsp
	Tools    []*ToolItem     `json:"tools,omitempty" doc:"工具列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}
