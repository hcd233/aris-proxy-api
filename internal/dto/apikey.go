// Package dto API Key DTO
package dto

// CreateAPIKeyReq 创建 API Key 请求
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type CreateAPIKeyReq struct {
	Body *CreateAPIKeyReqBody `json:"body" doc:"Request body containing API key name"`
}

// CreateAPIKeyReqBody 创建 API Key 请求体
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type CreateAPIKeyReqBody struct {
	Name string `json:"name" required:"true" minLength:"1" maxLength:"64" doc:"API Key 名称"`
}

// CreateAPIKeyRsp 创建 API Key 响应
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type CreateAPIKeyRsp struct {
	CommonRsp
	Key *APIKeyDetail `json:"key,omitempty" doc:"创建的 API Key 详情"`
}

// ListAPIKeysRsp 列出 API Key 响应
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type ListAPIKeysRsp struct {
	CommonRsp
	Keys []*APIKeyItem `json:"keys,omitempty" doc:"API Key 列表"`
}

// APIKeyItem API Key 列表项（masked key）
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type APIKeyItem struct {
	ID        uint   `json:"id" doc:"API Key ID"`
	Name      string `json:"name" doc:"API Key 名称"`
	Key       string `json:"key" doc:"Masked API Key 值"`
	CreatedAt string `json:"createdAt" doc:"创建时间"`
}

// APIKeyDetail API Key 详情（完整 key，仅创建时返回）
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type APIKeyDetail struct {
	ID        uint   `json:"id" doc:"API Key ID"`
	Name      string `json:"name" doc:"API Key 名称"`
	Key       string `json:"key" doc:"完整 API Key 值（仅创建时返回）"`
	CreatedAt string `json:"createdAt" doc:"创建时间"`
}

// DeleteAPIKeyReq 删除 API Key 请求
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type DeleteAPIKeyReq struct {
	ID uint `path:"id" required:"true" minimum:"1" doc:"API Key ID"`
}
