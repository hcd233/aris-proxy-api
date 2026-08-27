// Package dto Upstream DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// ListUpstreamReq 列出 Upstream 分组请求
//
//	@author centonhuang
//	@update 2026-08-27 10:00:00
type ListUpstreamReq struct {
	model.CommonParam
	Username string `query:"username,omitempty" doc:"按归属用户名过滤(仅管理员生效)"`
}

// ListUpstreamRsp 列出 Upstream 分组响应
//
// 分页对象是 endpoint 组：pageInfo.total 为端点数，modelTotal 为当前筛选范围内模型总数。
//
//	@author centonhuang
//	@update 2026-08-27 10:00:00
type ListUpstreamRsp struct {
	CommonRsp
	Groups     []*UpstreamGroupItem `json:"groups,omitempty" doc:"Endpoint 分组列表"`
	PageInfo   *model.PageInfo      `json:"pageInfo,omitempty" doc:"端点组分页信息(total=端点数)"`
	ModelTotal int64                `json:"modelTotal" doc:"当前筛选范围内模型总数"`
}

// UpstreamGroupItem 单个 Endpoint 及其名下模型的分组项
//
//	@author centonhuang
//	@update 2026-08-27 10:00:00
type UpstreamGroupItem struct {
	Endpoint   *UpstreamEndpointItem `json:"endpoint" required:"true" doc:"端点详情"`
	Models     []*UpstreamModelItem  `json:"models" doc:"端点下模型（组内不分页，上限 200）"`
	ModelCount int                   `json:"modelCount" doc:"模型数量(截断后口径)"`
	Truncated  bool                  `json:"truncated,omitempty" doc:"组内模型是否被截断"`
}

// UpstreamUserItem Upstream 归属用户信息（列表嵌套展示）
//
//	@author centonhuang
//	@update 2026-08-27 10:00:00
type UpstreamUserItem struct {
	ID     uint   `json:"id" doc:"用户 ID"`
	Name   string `json:"name" doc:"用户名"`
	Avatar string `json:"avatar" doc:"头像 URL"`
}

// UpstreamEndpointItem Endpoint 分组头条目
//
//	@author centonhuang
//	@update 2026-08-27 10:00:00
type UpstreamEndpointItem struct {
	ID                          uint              `json:"id" doc:"Endpoint ID"`
	User                        *UpstreamUserItem `json:"user,omitempty" doc:"归属用户信息（用户缺失或已删除时缺省）"`
	Name                        string            `json:"name" doc:"Endpoint 名称"`
	OpenaiBaseURL               string            `json:"openaiBaseURL" doc:"OpenAI Base URL"`
	AnthropicBaseURL            string            `json:"anthropicBaseURL" doc:"Anthropic Base URL"`
	MaskedAPIKey                string            `json:"maskedAPIKey" doc:"Masked API Key"`
	SupportOpenAIChatCompletion bool              `json:"supportOpenAIChatCompletion" doc:"是否支持 OpenAI Chat Completion"`
	SupportOpenAIResponse       bool              `json:"supportOpenAIResponse" doc:"是否支持 OpenAI Response"`
	SupportAnthropicMessage     bool              `json:"supportAnthropicMessage" doc:"是否支持 Anthropic Message"`
	CreatedAt                   time.Time         `json:"createdAt" doc:"创建时间"`
	UpdatedAt                   time.Time         `json:"updatedAt" doc:"更新时间"`
}

// UpstreamModelItem 组内 Model 条目
//
// 归属关系由分组结构表达，不再嵌套 endpoint 字段。
//
//	@author centonhuang
//	@update 2026-08-27 10:00:00
type UpstreamModelItem struct {
	ID              uint                 `json:"id" doc:"Model ID"`
	User            *UpstreamUserItem    `json:"user,omitempty" doc:"归属用户信息（用户缺失或已删除时缺省）"`
	Alias           string               `json:"alias" doc:"模型别名"`
	ModelID         string               `json:"modelId" doc:"业务模型ID"`
	UpstreamModel   string               `json:"upstreamModel" doc:"上游实际模型名"`
	Enabled         bool                 `json:"enabled" doc:"是否启用"`
	ContextLength   int                  `json:"contextLength" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens int                  `json:"maxOutputTokens" doc:"最大输出长度（tokens）"`
	Capabilities    []enum.InputModality `json:"capabilities" doc:"模型能力（输入模态集合）"`
	CreatedAt       time.Time            `json:"createdAt" doc:"创建时间"`
	UpdatedAt       time.Time            `json:"updatedAt" doc:"更新时间"`
}
