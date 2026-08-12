package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

type CreateBlockedReq struct {
	Body *CreateBlockedReqBody `json:"body" doc:"Request body"`
}

type CreateBlockedReqBody struct {
	Word   string `json:"word" required:"true" minLength:"1" maxLength:"512" doc:"敏感词"`
	Action string `json:"action,omitempty" enum:"deny,omit" doc:"命中处理动作（默认 deny）"`
}

type UpdateBlockedReq struct {
	ID   uint                  `query:"id" required:"true" minimum:"1" doc:"Blocked ID"`
	Body *UpdateBlockedReqBody `json:"body" doc:"Request body"`
}

type UpdateBlockedReqBody struct {
	Action *string `json:"action,omitempty" enum:"deny,omit" doc:"命中处理动作"`
}

// DeleteBlockedReq 删除请求（支持逗号分隔批量）
type DeleteBlockedReq struct {
	IDs string `query:"ids" required:"true" minLength:"1" doc:"Blocked ID 列表，逗号分隔，如 1 或 1,2,3"`
}

// DeleteBlockedRsp 删除响应
type DeleteBlockedRsp struct {
	CommonRsp
	DeletedCount int `json:"deletedCount,omitempty" doc:"成功删除数量"`
}

type ListBlockedReq struct {
	model.CommonParam
}

type ListBlockedRsp struct {
	CommonRsp
	Blocked  []*BlockedItem  `json:"blocked,omitempty" doc:"Blocked 列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

type BlockedItem struct {
	ID        uint      `json:"id" doc:"Blocked ID"`
	Word      string    `json:"word" doc:"敏感词"`
	Action    string    `json:"action" doc:"命中处理动作"`
	HitCount  uint      `json:"hitCount" doc:"命中次数"`
	CreatedAt time.Time `json:"createdAt" doc:"创建时间"`
}
