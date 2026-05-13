package service

import (
	"context"
	"math/rand"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// EndpointResolver 模型端点解析领域服务
//
// 按 alias 查询 model 表 → 随机选一个 endpoint → 返回 endpoint + model。
type EndpointResolver interface {
	Resolve(ctx context.Context, alias vo.EndpointAlias) (*aggregate.Endpoint, *aggregate.Model, error)
}

type endpointResolver struct {
	endpointRepo llmproxy.EndpointRepository
	modelRepo    llmproxy.ModelRepository
}

// NewEndpointResolver 构造领域服务
func NewEndpointResolver(
	endpointRepo llmproxy.EndpointRepository,
	modelRepo llmproxy.ModelRepository,
) EndpointResolver {
	return &endpointResolver{
		endpointRepo: endpointRepo,
		modelRepo:    modelRepo,
	}
}

// Resolve 按 alias 解析端点
//
// 1. 查 model 表（按 alias）→ 收集所有 endpointID
// 2. 随机选一个 endpointID
// 3. 查 endpoint 表（按 id）
// 4. 返回 (endpoint, model)
func (r *endpointResolver) Resolve(ctx context.Context, alias vo.EndpointAlias) (*aggregate.Endpoint, *aggregate.Model, error) {
	if alias.IsEmpty() {
		return nil, nil, ierr.New(ierr.ErrValidation, "endpoint alias is empty")
	}
	models, err := r.modelRepo.FindByAlias(ctx, alias)
	if err != nil {
		return nil, nil, err
	}
	if len(models) == 0 {
		return nil, nil, ierr.Newf(ierr.ErrDataNotExists, "model %q not found", alias.String())
	}
	m := models[rand.Intn(len(models))]
	ep, err := r.endpointRepo.FindByID(ctx, m.EndpointID())
	if err != nil {
		return nil, nil, err
	}
	if ep == nil {
		return nil, nil, ierr.Newf(ierr.ErrDataNotExists, "endpoint %d not found for model %q", m.EndpointID(), alias.String())
	}
	return ep, m, nil
}
