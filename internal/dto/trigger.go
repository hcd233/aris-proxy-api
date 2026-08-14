package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

type CreateTriggerReq struct {
	Body *CreateTriggerReqBody `json:"body" doc:"Request body"`
}

type CreateTriggerReqBody struct {
	Word   string `json:"word" required:"true" minLength:"1" maxLength:"512" doc:"触发词"`
	Action string `json:"action,omitempty" enum:"deny,omit,capture" doc:"命中处理动作（默认 deny）"`
}

type UpdateTriggerReq struct {
	ID   uint                  `query:"id" required:"true" minimum:"1" doc:"Trigger ID"`
	Body *UpdateTriggerReqBody `json:"body" doc:"Request body"`
}

type UpdateTriggerReqBody struct {
	Action *string `json:"action,omitempty" enum:"deny,omit,capture" doc:"命中处理动作"`
}

// DeleteTriggerReq 删除请求（支持逗号分隔批量）
type DeleteTriggerReq struct {
	IDs string `query:"ids" required:"true" minLength:"1" doc:"Trigger ID 列表，逗号分隔，如 1 或 1,2,3"`
}

// DeleteTriggerRsp 删除响应
type DeleteTriggerRsp struct {
	CommonRsp
	DeletedCount int `json:"deletedCount,omitempty" doc:"成功删除数量"`
}

type ListTriggerReq struct {
	model.CommonParam
}

type ListTriggerRsp struct {
	CommonRsp
	Trigger  []*TriggerItem  `json:"trigger,omitempty" doc:"Trigger 列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

type TriggerItem struct {
	ID        uint      `json:"id" doc:"Trigger ID"`
	Word      string    `json:"word" doc:"触发词"`
	Action    string    `json:"action" doc:"命中处理动作"`
	HitCount  uint      `json:"hitCount" doc:"命中次数"`
	CreatedAt time.Time `json:"createdAt" doc:"创建时间"`
}
