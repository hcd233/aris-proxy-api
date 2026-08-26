package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type listEndpointsHandler struct {
	repo     llmproxy.EndpointRepository
	userRepo identity.UserRepository
}

// NewListEndpointsHandler 构造查询处理器
func NewListEndpointsHandler(repo llmproxy.EndpointRepository, userRepo identity.UserRepository) port.ListEndpointsHandler {
	return &listEndpointsHandler{repo: repo, userRepo: userRepo}
}

// Handle 执行列表查询
//
// ScopeUserID>0 时仅返回该用户配置；==0（admin 视角）返回全量，可按 Username 过滤并回填归属用户名。
func (h *listEndpointsHandler) Handle(ctx context.Context, q port.ListEndpointsQuery) ([]*port.EndpointView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	scope := q.ScopeUserID
	if scope == 0 && q.Username != "" {
		u, err := h.userRepo.FindByName(ctx, q.Username)
		if err != nil {
			log.Error("[EndpointQuery] Find user by name failed", zap.Error(err))
			return nil, nil, err
		}
		if u == nil {
			// 用户不存在 → 空结果而非错误
			return []*port.EndpointView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize}, nil
		}
		scope = u.AggregateID()
	}

	endpoints, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam, scope)
	if err != nil {
		log.Error("[EndpointQuery] List endpoints failed", zap.Error(err))
		return nil, nil, err
	}

	usernamesByID := h.loadUsernames(ctx, endpoints)

	views := lo.Map(endpoints, func(ep *aggregate.Endpoint, _ int) *port.EndpointView {
		// demo 视角：name 展示明文（供演示辨认），仅屏蔽 baseURL 与 APIKey
		openaiBaseURL := ep.OpenaiBaseURL()
		anthropicBaseURL := ep.AnthropicBaseURL()
		if q.IsDemo {
			openaiBaseURL = commonutil.MaskSecret(openaiBaseURL)
			anthropicBaseURL = commonutil.MaskSecret(anthropicBaseURL)
		}
		return &port.EndpointView{
			ID:                          ep.AggregateID(),
			Username:                    usernamesByID[ep.UserID()],
			Name:                        ep.Name(),
			OpenaiBaseURL:               openaiBaseURL,
			AnthropicBaseURL:            anthropicBaseURL,
			MaskedAPIKey:                commonutil.MaskSecret(ep.APIKey()),
			SupportOpenAIChatCompletion: ep.SupportOpenAIChatCompletion(),
			SupportOpenAIResponse:       ep.SupportOpenAIResponse(),
			SupportAnthropicMessage:     ep.SupportAnthropicMessage(),
			CreatedAt:                   ep.CreatedAt(),
			UpdatedAt:                   ep.UpdatedAt(),
		}
	})

	log.Info("[EndpointQuery] List endpoints", zap.Int("count", len(views)))
	return views, pageInfo, nil
}

// loadUsernames 一次性拉取本页 endpoint 归属用户名，避免 N+1。
func (h *listEndpointsHandler) loadUsernames(ctx context.Context, endpoints []*aggregate.Endpoint) map[uint]string {
	out := make(map[uint]string, len(endpoints))
	ids := lo.Uniq(lo.Map(endpoints, func(ep *aggregate.Endpoint, _ int) uint { return ep.UserID() }))
	if len(ids) == 0 {
		return out
	}
	users, err := h.userRepo.BatchFindByIDs(ctx, ids)
	if err != nil {
		logger.WithCtx(ctx).Warn("[EndpointQuery] Load usernames failed", zap.Error(err))
		return out
	}
	for id, u := range users {
		out[id] = string(u.Name())
	}
	return out
}
