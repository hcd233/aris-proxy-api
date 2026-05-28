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
	ShareID   string    `json:"shareId" doc:"分享ID (UUID)"`
	ExpiresAt time.Time `json:"expiresAt" doc:"过期时间"`
}

// GetShareContentReq 获取分享内容请求
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type GetShareContentReq struct {
	ShareID string `path:"id" required:"true" doc:"分享ID (UUID)"`
}

// GetShareContentRsp 获取分享内容响应
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type GetShareContentRsp struct {
	CommonRsp
	Session *ShareContentSessionDetail `json:"session,omitempty" doc:"Session详情（不含APIKeyName等敏感字段）"`
}

// ShareContentSessionDetail 分享内容中的Session详情（去除敏感字段）
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type ShareContentSessionDetail struct {
	ID        uint              `json:"id" doc:"Session ID"`
	CreatedAt time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt time.Time         `json:"updatedAt" doc:"更新时间"`
	Metadata  map[string]string `json:"metadata,omitempty" doc:"请求元数据"`
	Messages  []*MessageItem    `json:"messages" doc:"消息列表"`
	Tools     []*ToolItem       `json:"tools" doc:"工具列表"`
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
	ShareID   string    `json:"shareId" doc:"分享ID (UUID)"`
	SessionID uint      `json:"sessionId" doc:"Session ID"`
	CreatedAt time.Time `json:"createdAt" doc:"创建时间"`
	ExpiresAt time.Time `json:"expiresAt" doc:"过期时间"`
}

// DeleteShareReq 删除分享请求
//
//	@author centonhuang
//	@update 2026-05-28 10:00:00
type DeleteShareReq struct {
	ShareID string `path:"id" required:"true" doc:"分享ID (UUID)"`
}
