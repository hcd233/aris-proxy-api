// Package dto Upstream DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// ListUpstreamReq 列出 Upstream 分组请求
//
// 只嵌 PageParam/QueryParam，不复用 model.CommonParam：其 SortParam（sort/sortField）
// 在本接口无消费方（分组固定按 endpoint id 升序），前端（web/src/lib/api-client.ts
// listUpstream）也只传 page/pageSize/query/username——排序参数被静默忽略，
// 按契约删除而非假装支持（2026-09-25 CR P2）。
//
//	@author centonhuang
//	@update 2026-09-26 00:00:00
type ListUpstreamReq struct {
	model.PageParam
	model.QueryParam
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
	Endpoint        *UpstreamEndpointItem `json:"endpoint" required:"true" doc:"端点详情"`
	Models          []*UpstreamModelItem  `json:"models" doc:"端点下模型（组内不分页，上限 200）"`
	ModelCount      int                   `json:"modelCount" doc:"模型数量(截断后口径)"`
	TotalModelCount int                   `json:"totalModelCount" doc:"模型总数(截断前口径；未截断时等于 modelCount)"`
	Truncated       bool                  `json:"truncated,omitempty" doc:"组内模型是否被截断"`
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
	ID                          uint               `json:"id" doc:"Endpoint ID"`
	User                        *UpstreamUserItem  `json:"user,omitempty" doc:"归属用户信息（用户缺失或已删除时缺省）"`
	Name                        string             `json:"name" doc:"Endpoint 名称"`
	OpenaiBaseURL               string             `json:"openaiBaseURL" doc:"OpenAI Base URL"`
	AnthropicBaseURL            string             `json:"anthropicBaseURL" doc:"Anthropic Base URL"`
	MaskedAPIKey                string             `json:"maskedAPIKey" doc:"Masked API Key"`
	AuthType                    string             `json:"authType" doc:"认证类型(api_key/oauth_anthropic/oauth_openai)"`
	OAuth                       *UpstreamOAuthItem `json:"oauth,omitempty" doc:"订阅凭据投影（仅 OAuth 端点）"`
	SupportOpenAIChatCompletion bool               `json:"supportOpenAIChatCompletion" doc:"是否支持 OpenAI Chat Completion"`
	SupportOpenAIResponse       bool               `json:"supportOpenAIResponse" doc:"是否支持 OpenAI Response"`
	SupportAnthropicMessage     bool               `json:"supportAnthropicMessage" doc:"是否支持 Anthropic Message"`
	SupportOpenAIDecision       bool               `json:"supportOpenAIDecision" doc:"是否支持 OpenAI Decision"`
	CreatedAt                   time.Time          `json:"createdAt" doc:"创建时间"`
	UpdatedAt                   time.Time          `json:"updatedAt" doc:"更新时间"`
}

// UpstreamOAuthItem 订阅端点凭据投影（仅展示字段；token 永不下发）
type UpstreamOAuthItem struct {
	AccountEmail   string              `json:"accountEmail,omitempty" doc:"账号邮箱"`
	PlanType       string              `json:"planType,omitempty" doc:"订阅类型"`
	Status         string              `json:"status" doc:"凭据状态(active/reauth_required)"`
	LastError      string              `json:"lastError,omitempty" doc:"最近错误（脱敏展示）"`
	Quota          *OAuthQuotaSnapshot `json:"quota,omitempty" doc:"额度快照"`
	QuotaUpdatedAt *time.Time          `json:"quotaUpdatedAt,omitempty" doc:"额度快照采集时间"`
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
	Priority        int                  `json:"priority" doc:"调度优先级（数字小=优先级高）"`
	Weight          int                  `json:"weight" doc:"同优先级加权随机权重"`
	ContextLength   int                  `json:"contextLength" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens int                  `json:"maxOutputTokens" doc:"最大输出长度（tokens）"`
	Capabilities    []enum.InputModality `json:"capabilities" doc:"模型能力（输入模态集合）"`
	Pricing         *PricingDTO          `json:"pricing,omitempty" doc:"定价（未计价缺省）"`
	ConfigMissing   []enum.ConfigMissing `json:"configMissing,omitempty" enum:"pricing,spec" doc:"配置缺失项: pricing(未计价)/spec(未填规格)"`
	CreatedAt       time.Time            `json:"createdAt" doc:"创建时间"`
	UpdatedAt       time.Time            `json:"updatedAt" doc:"更新时间"`
}
