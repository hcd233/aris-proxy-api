// Package dto Model DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// CreateModelReq 创建 Model 请求
type CreateModelReq struct {
	Body *CreateModelReqBody `json:"body" doc:"Request body"`
}

// CreateModelReqBody 创建 Model 请求体
type CreateModelReqBody struct {
	Alias           string               `json:"alias" required:"true" minLength:"1" doc:"模型别名（对外暴露）"`
	ModelID         *string              `json:"modelId,omitempty" doc:"业务模型ID；缺省时默认等于 alias"`
	UpstreamModel   string               `json:"upstreamModel" required:"true" minLength:"1" doc:"上游实际模型名"`
	EndpointID      uint                 `json:"endpointID" required:"true" minimum:"1" doc:"关联 Endpoint ID"`
	ContextLength   int                  `json:"contextLength,omitempty" minimum:"0" default:"128000" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens int                  `json:"maxOutputTokens,omitempty" minimum:"0" default:"64000" doc:"最大输出长度（tokens）"`
	Capabilities    []enum.InputModality `json:"capabilities,omitempty" doc:"模型能力（输入模态集合；合法值 text/image；必须包含 text；缺省为 [text]）"`
}

// UpdateModelReq 更新 Model 请求
type UpdateModelReq struct {
	ID   uint                `query:"id" required:"true" minimum:"1" doc:"Model ID"`
	Body *UpdateModelReqBody `json:"body" doc:"Request body"`
}

// UpdateModelReqBody 更新 Model 请求体
type UpdateModelReqBody struct {
	Alias           *string               `json:"alias,omitempty" doc:"模型别名"`
	ModelID         *string               `json:"modelId,omitempty" doc:"业务模型ID(非空)"`
	SyncHistory     *bool                 `json:"syncHistory,omitempty" doc:"modelId 变化时是否同步更新历史记录（audit/session/message）"`
	UpstreamModel   *string               `json:"upstreamModel,omitempty" doc:"上游实际模型名"`
	EndpointID      *uint                 `json:"endpointID,omitempty" minimum:"1" doc:"关联 Endpoint ID"`
	Enabled         *bool                 `json:"enabled,omitempty" doc:"是否启用"`
	ContextLength   *int                  `json:"contextLength,omitempty" minimum:"0" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens *int                  `json:"maxOutputTokens,omitempty" minimum:"0" doc:"最大输出长度（tokens）"`
	Capabilities    *[]enum.InputModality `json:"capabilities,omitempty" doc:"模型能力（输入模态集合；合法值 text/image；必须包含 text）"`
}

// ModelUpdateRsp 更新 Model 响应
//
// 三项计数为历史同步（syncHistory）的各表影响行数；未同步时全 0。
type ModelUpdateRsp struct {
	AuditCount   int64 `json:"auditCount" doc:"审计记录替换行数"`
	SessionCount int64 `json:"sessionCount" doc:"会话替换行数"`
	MessageCount int64 `json:"messageCount" doc:"消息替换行数"`
}

// DeleteModelReq 删除 Model 请求
type DeleteModelReq struct {
	ID uint `query:"id" required:"true" minimum:"1" doc:"Model ID"`
}

// ListModelsReq 平铺模型列表请求（GET + query）
//
// 分页/排序/筛选字段全部显式声明 query 标签，不复用 model.CommonParam 的嵌入字段：
// CommonParam.SortField 只有 json 标签而无 query 标签，huma 只认 path/query/header/
// form/cookie 这类来源标签，缺少时该字段会被静默忽略（永远零值）。
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ListModelsReq struct {
	Page       int       `query:"page" required:"true" minimum:"1" doc:"页码"`
	PageSize   int       `query:"pageSize" required:"true" minimum:"1" maximum:"500" doc:"每页条数"`
	Query      string    `query:"query" maxLength:"100" doc:"关键词（命中 alias / modelId / upstreamModel）"`
	Sort       enum.Sort `query:"sort" enum:"asc,desc" doc:"排序方向"`
	SortField  string    `query:"sortField" maxLength:"50" doc:"排序列（白名单：alias/context_length/max_output_tokens/created_at/endpoint_id/enabled；非法值回退 created_at）"`
	Status     string    `query:"status" enum:"enabled,disabled" doc:"启用状态筛选（缺省为全部）"`
	EndpointID uint      `query:"endpointID" minimum:"1" doc:"按所属端点过滤（0=不过滤）"`
	Capability string    `query:"capability" enum:"text,image" doc:"按输入模态过滤（缺省为全部）"`
	Username   string    `query:"username" maxLength:"64" doc:"按归属用户名过滤（仅管理员生效）"`
}

// ListModelsRsp 平铺模型列表响应
//
// 分页对象是 model 行：pageInfo.total 为当前筛选范围内模型总数。
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ListModelsRsp struct {
	CommonRsp
	Items    []*ModelListItem `json:"items,omitempty" doc:"模型列表"`
	PageInfo *model.PageInfo  `json:"pageInfo,omitempty" doc:"分页信息(total=模型数)"`
}

// ModelListEndpointItem 平铺列表中的所属端点（仅展示所需最小字段）
//
// 刻意不含 baseURL / apiKey：平铺视图面向模型编目，不应把上游凭据带进每行。
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ModelListEndpointItem struct {
	ID   uint   `json:"id" doc:"Endpoint ID"`
	Name string `json:"name" doc:"Endpoint 名称"`
}

// ModelListItem 平铺模型列表行
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ModelListItem struct {
	ID              uint                   `json:"id" doc:"Model ID"`
	User            *UpstreamUserItem      `json:"user,omitempty" doc:"归属用户信息（用户缺失或已删除时缺省）"`
	Endpoint        *ModelListEndpointItem `json:"endpoint,omitempty" doc:"所属端点（端点缺失时缺省）"`
	Alias           string                 `json:"alias" doc:"模型别名"`
	ModelID         string                 `json:"modelId" doc:"业务模型ID"`
	UpstreamModel   string                 `json:"upstreamModel" doc:"上游实际模型名（demo 权限下已脱敏）"`
	Enabled         bool                   `json:"enabled" doc:"是否启用"`
	ContextLength   int                    `json:"contextLength" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens int                    `json:"maxOutputTokens" doc:"最大输出长度（tokens）"`
	Capabilities    []enum.InputModality   `json:"capabilities" doc:"模型能力（输入模态集合）"`
	CreatedAt       time.Time              `json:"createdAt" doc:"创建时间"`
	UpdatedAt       time.Time              `json:"updatedAt" doc:"更新时间"`
}
