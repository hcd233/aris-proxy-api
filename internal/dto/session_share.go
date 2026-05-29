// Package dto Session Share DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// CreateShareReq 创建分享请求
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type CreateShareReq struct {
	Body *CreateShareReqBody `json:"body" doc:"Request body containing session ID"`
}

// CreateShareReqBody 创建分享请求体
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type CreateShareReqBody struct {
	SessionID uint `json:"sessionId" required:"true" minimum:"1" doc:"Session ID"`
}

// CreateShareRsp 创建分享响应
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type CreateShareRsp struct {
	CommonRsp
	ShareID   string    `json:"shareId" doc:"分享ID (6-8 位大小写字母+数字短码)"`
	ExpiresAt time.Time `json:"expiresAt" doc:"过期时间"`
}

// ListSharesReq 获取分享列表请求
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ListSharesReq struct {
	model.PageParam
}

// ListSharesRsp 获取分享列表响应
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ListSharesRsp struct {
	CommonRsp
	Shares   []*ShareItem    `json:"shares,omitempty" doc:"分享列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ShareItem 分享列表项
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ShareItem struct {
	ShareID   string    `json:"shareId" doc:"分享ID (6-8 位大小写字母+数字短码)"`
	SessionID uint      `json:"sessionId" doc:"Session ID"`
	CreatedAt time.Time `json:"createdAt" doc:"创建时间"`
	ExpiresAt time.Time `json:"expiresAt" doc:"过期时间"`
}

// DeleteShareReq 删除分享请求
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type DeleteShareReq struct {
	ShareID string `query:"id" required:"true" doc:"分享ID (6-8 位大小写字母+数字短码)"`
}

// ─── Share 分页接口（公开，IP 限流） ─────────────────────────────────────────

// ShareSessionMetadata 分享 Session 元数据（不含敏感字段）
//
//	@author centonhuang
//	@update 2026-05-29 16:00:00
type ShareSessionMetadata struct {
	ID           uint              `json:"id" doc:"Session ID"`
	CreatedAt    time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt    time.Time         `json:"updatedAt" doc:"更新时间"`
	Metadata     map[string]string `json:"metadata,omitempty" doc:"请求元数据"`
	MessageCount int               `json:"messageCount" doc:"消息总数"`
	ToolCount    int               `json:"toolCount" doc:"工具总数"`
}

// GetShareMetadataReq 获取分享 Session 元数据请求
//
//	@author centonhuang
//	@update 2026-05-29 16:00:00
type GetShareMetadataReq struct {
	ShareID string `query:"id" required:"true" doc:"分享ID (6-8 位大小写字母+数字短码)"`
}

// GetShareMetadataRsp 获取分享 Session 元数据响应
//
//	@author centonhuang
//	@update 2026-05-29 16:00:00
type GetShareMetadataRsp struct {
	CommonRsp
	Session *ShareSessionMetadata `json:"session,omitempty" doc:"Session 元数据（不含敏感字段）"`
}

// ListShareMessagesReq 分页获取分享 Session 消息请求
//
//	@author centonhuang
//	@update 2026-05-29 16:00:00
type ListShareMessagesReq struct {
	ShareID  string `query:"id" required:"true" doc:"分享ID (6-8 位大小写字母+数字短码)"`
	Page     int    `query:"page" required:"true" minimum:"1" doc:"页码"`
	PageSize int    `query:"pageSize" required:"true" minimum:"1" maximum:"200" default:"50" doc:"每页条数"`
}

// ListShareMessagesRsp 分页获取分享 Session 消息响应
//
//	@author centonhuang
//	@update 2026-05-29 16:00:00
type ListShareMessagesRsp struct {
	CommonRsp
	Messages []*MessageItem  `json:"messages,omitempty" doc:"消息列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ListShareToolsReq 分页获取分享 Session 工具请求
//
//	@author centonhuang
//	@update 2026-05-29 16:00:00
type ListShareToolsReq struct {
	ShareID  string `query:"id" required:"true" doc:"分享ID (6-8 位大小写字母+数字短码)"`
	Page     int    `query:"page" required:"true" minimum:"1" doc:"页码"`
	PageSize int    `query:"pageSize" required:"true" minimum:"1" maximum:"200" default:"20" doc:"每页条数"`
}

// ListShareToolsRsp 分页获取分享 Session 工具响应
//
//	@author centonhuang
//	@update 2026-05-29 16:00:00
type ListShareToolsRsp struct {
	CommonRsp
	Tools    []*ToolItem     `json:"tools,omitempty" doc:"工具列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}
