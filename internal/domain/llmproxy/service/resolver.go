// Package service llmproxy 域领域服务
package service

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// EndpointResolver 模型端点解析领域服务
//
// 按「主 Provider → 回退 Provider」的顺序查找端点。规则继承自原
// service.findEndpoint（openai 入站优先 openai、回退 anthropic；anthropic 入站反之）。
//
//	@author centonhuang
//	@update 2026-04-22 16:30:00
type EndpointResolver interface {
	// Resolve 按主/回退 Provider 查询别名对应的端点
	//
	//	@param ctx context.Context
	//	@param alias vo.EndpointAlias
	//	@param primary enum.ProviderType 主 provider
	//	@param fallback enum.ProviderType 回退 provider
	//	@return *aggregate.Endpoint
	//	@return error 两个 provider 都未命中时返回 ierr.ErrDataNotExists
	Resolve(ctx context.Context, alias vo.EndpointAlias, primary, fallback enum.ProviderType) (*aggregate.Endpoint, error)
}

type endpointResolver struct {
	repo llmproxy.EndpointRepository
}

// NewEndpointResolver 构造领域服务
//
//	@param repo llmproxy.EndpointRepository
//	@return EndpointResolver
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func NewEndpointResolver(repo llmproxy.EndpointRepository) EndpointResolver {
	return &endpointResolver{repo: repo}
}

// Resolve 实现接口
//
// 按 primary → fallback 顺序查询；仓储未找到返回 (nil, nil)，真 DB 错误直接上抛，
// 不再向 fallback 降级，避免把基础设施故障伪装成 ErrDataNotExists。
//
//	@receiver r *endpointResolver
//	@param ctx context.Context
//	@param alias vo.EndpointAlias
//	@param primary enum.ProviderType
//	@param fallback enum.ProviderType
//	@return *aggregate.Endpoint
//	@return error
//	@author centonhuang
//	@update 2026-04-24 14:00:00
func (r *endpointResolver) Resolve(ctx context.Context, alias vo.EndpointAlias, primary, fallback enum.ProviderType) (*aggregate.Endpoint, error) {
	if alias.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "endpoint alias is empty")
	}
	ep, err := r.repo.FindByAliasAndProvider(ctx, alias, primary)
	if err != nil {
		return nil, err
	}
	if ep != nil {
		return ep, nil
	}
	ep, err = r.repo.FindByAliasAndProvider(ctx, alias, fallback)
	if err != nil {
		return nil, err
	}
	if ep != nil {
		return ep, nil
	}
	return nil, ierr.Newf(ierr.ErrDataNotExists, "endpoint %q not found for providers [%s, %s]", alias.String(), primary, fallback)
}
