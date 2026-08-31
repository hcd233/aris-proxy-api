package service

import (
	"context"
	"math/rand"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// EndpointResolver 模型端点解析领域服务
//
// 按 alias 查询 model 表 → 随机选择满足能力要求的 endpoint → 返回 endpoint + model。
type EndpointResolver interface {
	Resolve(ctx context.Context, userID uint, alias vo.EndpointAlias, matcher func(*aggregate.Endpoint) bool) (*aggregate.Endpoint, *aggregate.Model, error)
}

type endpointResolver struct {
	endpointRepo llmproxy.EndpointRepository
	modelRepo    llmproxy.ModelRepository
	// sharedPoolFallback 用户租户未命中 alias 时回退查共享池（多租户化过渡开关）
	sharedPoolFallback bool
}

// NewEndpointResolver 构造领域服务
//
// sharedPoolFallback 由装配层从配置（gateway.shared_pool_fallback）注入，
// domain 层不直接依赖 config。
func NewEndpointResolver(
	endpointRepo llmproxy.EndpointRepository,
	modelRepo llmproxy.ModelRepository,
	sharedPoolFallback bool,
) EndpointResolver {
	return &endpointResolver{
		endpointRepo:       endpointRepo,
		modelRepo:          modelRepo,
		sharedPoolFallback: sharedPoolFallback,
	}
}

// Resolve 按 alias 解析端点
//
//  1. 查 model 表（按 alias，限定用户租户）→ 收集所有 endpointID
//  2. 随机遍历 endpointID
//  3. 返回首个满足 matcher 的 endpoint + model
//  4. 用户名下未命中且开启共享池回退（gateway.shared_pool_fallback）时，
//     回查共享池（user_id=0 的存量/共享配置），供多租户化过渡期兜底
//  5. 若无匹配端点，返回 ErrDataNotExists
func (r *endpointResolver) Resolve(ctx context.Context, userID uint, alias vo.EndpointAlias, matcher func(*aggregate.Endpoint) bool) (*aggregate.Endpoint, *aggregate.Model, error) {
	if alias.IsEmpty() {
		return nil, nil, ierr.New(ierr.ErrValidation, "endpoint alias is empty")
	}
	tenantID := userID
	models, err := r.modelRepo.FindByAlias(ctx, alias, &tenantID)
	if err != nil {
		return nil, nil, err
	}
	if len(models) == 0 && r.sharedPoolFallback {
		sharedPoolID := constant.SharedPoolUserID
		models, err = r.modelRepo.FindByAlias(ctx, alias, &sharedPoolID)
		if err != nil {
			return nil, nil, err
		}
	}
	if len(models) == 0 {
		return nil, nil, ierr.Newf(ierr.ErrDataNotExists, "model %q not found", alias.String())
	}
	scope := &tenantID
	if models[0].UserID() == constant.SharedPoolUserID {
		// 共享池命中的模型只允许解析到共享池自己的 endpoint，避免借用任意用户的 endpoint
		sharedPoolID := constant.SharedPoolUserID
		scope = &sharedPoolID
	}
	for _, idx := range rand.Perm(len(models)) {
		m := models[idx]
		if !m.Enabled() {
			continue
		}
		ep, findErr := r.endpointRepo.FindByID(ctx, m.EndpointID(), scope)
		if findErr != nil {
			return nil, nil, findErr
		}
		if ep == nil {
			continue
		}
		if matcher == nil || matcher(ep) {
			return ep, m, nil
		}
	}
	return nil, nil, ierr.Newf(ierr.ErrDataNotExists, "model %q has no endpoint supporting requested API", alias.String())
}
