// Package aggregate llmproxy 域聚合根
package aggregate

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// Endpoint 模型端点聚合根
//
// 表示一个「模型别名 + 上游协议 + 上游连接信息」的完整绑定。
// 一个 Alias 可绑定多个 Provider（openai / anthropic），Resolver 负责按优先级选择。
//
// Endpoint 是 llmproxy 域的只读聚合（Query 侧），不产生领域事件；
// 它的可变性通过 CRUD 仓储操作表达，不通过聚合行为表达。
//
//	@author centonhuang
//	@update 2026-04-22 16:30:00
type Endpoint struct {
	aggregate.Base

	alias    vo.EndpointAlias
	provider enum.ProviderType
	creds    vo.UpstreamCreds
}

// NewEndpoint 构造 Endpoint 聚合根
//
//	@param id uint 聚合唯一 ID（DB 主键）
//	@param alias vo.EndpointAlias 对外暴露的模型别名
//	@param provider enum.ProviderType 上游协议类型（openai/anthropic）
//	@param creds vo.UpstreamCreds 上游接入凭证
//	@return *Endpoint
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func NewEndpoint(id uint, alias vo.EndpointAlias, provider enum.ProviderType, creds vo.UpstreamCreds) *Endpoint {
	ep := &Endpoint{
		alias:    alias,
		provider: provider,
		creds:    creds,
	}
	ep.SetID(id)
	return ep
}

// AggregateType 实现 aggregate.Root 接口
//
//	@receiver *Endpoint
//	@return string
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func (*Endpoint) AggregateType() string { return constant.AggregateTypeEndpoint }

// Alias 返回别名
//
//	@receiver e *Endpoint
//	@return vo.EndpointAlias
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func (e *Endpoint) Alias() vo.EndpointAlias { return e.alias }

// Provider 返回上游协议
//
//	@receiver e *Endpoint
//	@return enum.ProviderType
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func (e *Endpoint) Provider() enum.ProviderType { return e.provider }

// Creds 返回上游凭证
//
//	@receiver e *Endpoint
//	@return vo.UpstreamCreds
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func (e *Endpoint) Creds() vo.UpstreamCreds { return e.creds }
