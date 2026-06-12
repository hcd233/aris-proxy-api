// Package dto Model DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

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
	ID   uint                `query:"id" required:"true" minimum:"1" doc:"Model ID"`
	Body *UpdateModelReqBody `json:"body" doc:"Request body"`
}

// UpdateModelReqBody 更新 Model 请求体
type UpdateModelReqBody struct {
	Alias      *string `json:"alias,omitempty" doc:"模型别名"`
	ModelName  *string `json:"modelName,omitempty" doc:"上游实际模型名"`
	EndpointID *uint   `json:"endpointID,omitempty" minimum:"1" doc:"关联 Endpoint ID"`
	Enabled    *bool   `json:"enabled,omitempty" doc:"是否启用"`
}

// DeleteModelReq 删除 Model 请求
type DeleteModelReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"Model ID"`
}

// ListModelsReq 列出 Model 请求
//
//	@author centonhuang
//	@update 2026-05-27 10:00:00
type ListModelsReq struct {
	model.CommonParam
}

// ListModelsRsp 列出 Model 响应
type ListModelsRsp struct {
	CommonRsp
	Models   []*ModelItem    `json:"models,omitempty" doc:"Model 列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// ModelItem Model 列表项
type ModelItem struct {
	ID        uint          `json:"id" doc:"Model ID"`
	Alias     string        `json:"alias" doc:"模型别名"`
	ModelName string        `json:"modelName" doc:"上游实际模型名"`
	Enabled   bool          `json:"enabled" doc:"是否启用"`
	Endpoint  *EndpointItem `json:"endpoint,omitempty" doc:"关联 Endpoint 详细信息"`
	CreatedAt time.Time     `json:"createdAt" doc:"创建时间"`
	UpdatedAt time.Time     `json:"updatedAt" doc:"更新时间"`
}
