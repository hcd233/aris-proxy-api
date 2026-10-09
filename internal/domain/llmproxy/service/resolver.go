package service

import (
	"context"
	"math"
	"math/rand/v2"
	"slices"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// Candidate 端点解析候选：一次 alias 解析中可用的 (Endpoint, Model) 组合。
type Candidate struct {
	Endpoint *aggregate.Endpoint
	Model    *aggregate.Model
}

// EndpointResolver 模型端点解析领域服务
//
// 按 alias 查询 model 表 → 过滤可用 endpoint → 按调度优先级有序返回候选。
type EndpointResolver interface {
	ResolveCandidates(ctx context.Context, userID uint, alias vo.EndpointAlias, matcher func(*aggregate.Endpoint) bool) ([]Candidate, error)
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

// ResolveCandidates 按 alias 解析出全部可用候选，按调度优先级有序返回。
//
//  1. 查 model 表（按 alias，限定用户租户）→ 收集所有 endpointID
//
//  2. 用户名下未命中且开启共享池回退（gateway.shared_pool_fallback）时，
//     回查共享池（user_id=0 的存量/共享配置），供多租户化过渡期兜底；
//     共享池命中的模型只允许解析到共享池自己的 endpoint，避免借用任意用户的 endpoint
//
//  3. 过滤 enabled 与 matcher（matcher 为 nil 表示不筛选）
//
//  4. priority 升序分档（数字小=优先级高）；同档内按 weight 做 A-Res 加权洗牌
//     （key = rand^(1/weight)，key 大者在前）
//
//  5. 无候选返回 ErrDataNotExists
//
//     @param ctx context.Context
//     @param userID uint 请求用户（多租户隔离）
//     @param alias vo.EndpointAlias 模型别名
//     @param matcher func(*aggregate.Endpoint) bool 端点能力过滤（如协议支持），nil 不筛选
//     @return []Candidate 有序候选（首选在前）
//     @return error
//     @author centonhuang
//     @update 2026-10-09 10:00:00
func (r *endpointResolver) ResolveCandidates(ctx context.Context, userID uint, alias vo.EndpointAlias, matcher func(*aggregate.Endpoint) bool) ([]Candidate, error) {
	if alias.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "endpoint alias is empty")
	}
	cands, err := r.collectCandidates(ctx, userID, alias, matcher)
	if err != nil {
		return nil, err
	}
	if len(cands) == 0 {
		return nil, ierr.Newf(ierr.ErrDataNotExists, "model %q has no endpoint supporting requested API", alias.String())
	}
	orderByScheduling(cands)
	return cands, nil
}

// collectCandidates 查找并过滤候选：多租户 + 共享池回退 + enabled/matcher 过滤。
func (r *endpointResolver) collectCandidates(ctx context.Context, userID uint, alias vo.EndpointAlias, matcher func(*aggregate.Endpoint) bool) ([]Candidate, error) {
	tenantID := userID
	models, err := r.modelRepo.FindByAlias(ctx, alias, &tenantID)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 && r.sharedPoolFallback {
		sharedPoolID := constant.SharedPoolUserID
		models, err = r.modelRepo.FindByAlias(ctx, alias, &sharedPoolID)
		if err != nil {
			return nil, err
		}
	}
	if len(models) == 0 {
		return nil, ierr.Newf(ierr.ErrDataNotExists, "model %q not found", alias.String())
	}
	scope := &tenantID
	if models[0].UserID() == constant.SharedPoolUserID {
		// 共享池命中的模型只允许解析到共享池自己的 endpoint，避免借用任意用户的 endpoint
		sharedPoolID := constant.SharedPoolUserID
		scope = &sharedPoolID
	}

	cands := make([]Candidate, 0, len(models))
	for _, m := range models {
		if !m.Enabled() {
			continue
		}
		ep, findErr := r.endpointRepo.FindByID(ctx, m.EndpointID(), scope)
		if findErr != nil {
			return nil, findErr
		}
		if ep == nil {
			continue
		}
		if matcher == nil || matcher(ep) {
			cands = append(cands, Candidate{Endpoint: ep, Model: m})
		}
	}
	return cands, nil
}

// orderByScheduling 原地按调度优先级排序：priority 升序分档（数字小=优先级高）；
// 同档内按 weight 做 A-Res 加权洗牌（key = rand^(1/weight)，key 大者在前）。
func orderByScheduling(cands []Candidate) {
	slices.SortStableFunc(cands, func(a, b Candidate) int {
		return a.Model.Priority() - b.Model.Priority()
	})
	for i := 0; i < len(cands); {
		j := i + 1
		for j < len(cands) && cands[j].Model.Priority() == cands[i].Model.Priority() {
			j++
		}
		if j-i > 1 {
			shuffleWeighted(cands[i:j])
		}
		i = j
	}
}

// shuffleWeighted 对同一优先级档做 A-Res 加权洗牌（无需预展开权重）。
func shuffleWeighted(group []Candidate) {
	type keyed struct {
		idx int
		key float64
	}
	ks := make([]keyed, 0, len(group))
	for idx, c := range group {
		w := float64(c.Model.Weight())
		ks = append(ks, keyed{idx: idx, key: math.Pow(rand.Float64(), 1/w)}) //nolint:gosec // 调度洗牌无需加密随机
	}
	slices.SortStableFunc(ks, func(a, b keyed) int {
		switch {
		case a.key > b.key:
			return -1
		case a.key < b.key:
			return 1
		default:
			return 0
		}
	})
	perm := make([]Candidate, 0, len(group))
	for _, k := range ks {
		perm = append(perm, group[k.idx])
	}
	copy(group, perm)
}
