// Package dto Model DTO
package dto

import "time"

// CreateModelReq 创建 Model 请求
type CreateModelReq struct {
	Body *CreateModelReqBody `json:"body" doc:"Request body"`
}

// CreateModelReqBody 创建 Model 请求体
type CreateModelReqBody struct {
	Alias      string `json:"alias" required:"true" minLength:"1" doc:"模型别名（对外暴露）"`
	ModelName  string `json:"modelName" required:"true" minLength:"1" doc:"上游实际模型名"`
	EndpointID uint   `json:"endpointID" required:"true" minimum:"1" doc:"关联 Endpoint ID"`
}

// UpdateModelReq 更新 Model 请求
type UpdateModelReq struct {
	ID   uint                `path:"id" required:"true" minimum:"1" doc:"Model ID"`
	Body *UpdateModelReqBody `json:"body" doc:"Request body"`
}

// UpdateModelReqBody 更新 Model 请求体
type UpdateModelReqBody struct {
	Alias      *string `json:"alias,omitempty" doc:"模型别名"`
	ModelName  *string `json:"modelName,omitempty" doc:"上游实际模型名"`
	EndpointID *uint   `json:"endpointID,omitempty" minimum:"1" doc:"关联 Endpoint ID"`
}

// DeleteModelReq 删除 Model 请求
type DeleteModelReq struct {
	ID uint `path:"id" required:"true" minimum:"1" doc:"Model ID"`
}

// ListModelsRsp 列出 Model 响应
type ListModelsRsp struct {
	CommonRsp
	Models []*ModelItem `json:"models,omitempty" doc:"Model 列表"`
}

// ModelItem Model 列表项
type ModelItem struct {
	ID         uint      `json:"id" doc:"Model ID"`
	Alias      string    `json:"alias" doc:"模型别名"`
	ModelName  string    `json:"modelName" doc:"上游实际模型名"`
	EndpointID uint      `json:"endpointID" doc:"关联 Endpoint ID"`
	CreatedAt  time.Time `json:"createdAt" doc:"创建时间"`
	UpdatedAt  time.Time `json:"updatedAt" doc:"更新时间"`
}
