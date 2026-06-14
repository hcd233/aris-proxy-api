package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

type CreateBlockedReq struct {
	Body *CreateBlockedReqBody `json:"body" doc:"Request body"`
}

type CreateBlockedReqBody struct {
	Word string `json:"word" required:"true" minLength:"1" maxLength:"512" doc:"敏感词"`
}

type DeleteBlockedReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"Blocked ID"`
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
	HitCount  uint      `json:"hitCount" doc:"命中次数"`
	CreatedAt time.Time `json:"createdAt" doc:"创建时间"`
}
