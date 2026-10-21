package query

import (
	"context"
	"strings"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/upstream/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityaggregate "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type listUpstreamHandler struct {
	endpointRepo llmproxy.EndpointRepository
	modelRepo    llmproxy.ModelRepository
	userRepo     identity.UserRepository
	credRepo     llmproxy.OAuthCredentialRepository
}

// NewListUpstreamHandler 构造 upstream 分组列表查询处理器
func NewListUpstreamHandler(endpointRepo llmproxy.EndpointRepository, modelRepo llmproxy.ModelRepository, userRepo identity.UserRepository, credRepo llmproxy.OAuthCredentialRepository) port.ListUpstreamHandler {
	return &listUpstreamHandler{endpointRepo: endpointRepo, modelRepo: modelRepo, userRepo: userRepo, credRepo: credRepo}
}

// Handle 执行分组列表查询：endpoint 组分页，组内模型随组全量返回。
func (h *listUpstreamHandler) Handle(ctx context.Context, q port.ListUpstreamQuery) ([]*port.UpstreamGroupView, int64, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	// scope 三态：nil=admin 全量；非 nil（含 0）=精确匹配该 userID（0 命中共享池）。
	// admin 叠加 username 筛选时反查 uid 转为精确匹配；普通用户的 username 由 handler 忽略。
	scope := q.ScopeUserID
	if scope == nil && q.Username != "" {
		u, err := h.userRepo.FindByName(ctx, q.Username)
		if err != nil {
			log.Error("[UpstreamQuery] Find user by name failed", zap.Error(err))
			return nil, 0, nil, err
		}
		if u == nil {
			// 用户不存在 → 空结果而非错误
			return []*port.UpstreamGroupView{}, 0,
				&model.PageInfo{Page: q.Page, PageSize: q.PageSize}, nil
		}
		uid := u.AggregateID()
		scope = &uid
	}

	allIDs, err := h.endpointRepo.FindIDsByScope(ctx, scope, q.IncludePublic)
	if err != nil {
		log.Error("[UpstreamQuery] Find endpoint ids failed", zap.Error(err))
		return nil, 0, nil, err
	}

	epsByID, err := h.endpointRepo.BatchFindByIDs(ctx, allIDs)
	if err != nil {
		log.Error("[UpstreamQuery] Load endpoints failed", zap.Error(err))
		return nil, 0, nil, err
	}

	models, err := h.modelRepo.ListByEndpointIDs(ctx, allIDs)
	if err != nil {
		log.Error("[UpstreamQuery] Load models failed", zap.Error(err))
		return nil, 0, nil, err
	}

	modelsByEp := make(map[uint][]*llmagg.Model, len(allIDs))
	for _, m := range models {
		modelsByEp[m.EndpointID()] = append(modelsByEp[m.EndpointID()], m)
	}

	matchedIDs := allIDs
	if q.Query != "" {
		kw := strings.ToLower(q.Query)
		// keyword 命中 endpoint 名称或其下任一模型字段 → 整组保留
		matchedIDs = lo.Filter(allIDs, func(id uint, _ int) bool {
			if ep := epsByID[id]; ep != nil && strings.Contains(strings.ToLower(ep.Name()), kw) {
				return true
			}
			return lo.SomeBy(modelsByEp[id], func(m *llmagg.Model) bool {
				return strings.Contains(strings.ToLower(m.Alias().String()), kw) ||
					strings.Contains(strings.ToLower(m.ModelID()), kw) ||
					strings.Contains(strings.ToLower(m.UpstreamModel()), kw)
			})
		})
	}

	start := (q.Page - 1) * q.PageSize
	start = min(start, len(matchedIDs))
	end := min(start+q.PageSize, len(matchedIDs))
	pageIDs := matchedIDs[start:end]

	usersByID := h.loadUsers(ctx, epsByID, modelsByEp)
	credsByEp := h.loadOAuthDisplays(ctx, pageIDs, epsByID)

	groups := make([]*port.UpstreamGroupView, 0, len(pageIDs))
	var modelTotal int64
	for _, id := range pageIDs {
		var cred *vo.OAuthCredentialDisplay
		if display, ok := credsByEp[id]; ok {
			cred = &display
		}
		// 非归属者（普通用户看他人公开端点）与 demo 一样脱敏订阅凭据投影：
		// 账号邮箱/错误详情属跨租户敏感信息。
		maskOAuth := q.IsDemo || !ownsEndpoint(q.ScopeUserID, epsByID[id])
		groups = append(groups, h.toGroupView(ctx, epsByID[id], modelsByEp[id], usersByID, cred, q.IsDemo, maskOAuth))
	}
	// modelTotal 口径跟随当前筛选范围的全量模型数（含非当前页组），不受组内截断影响。
	for _, id := range matchedIDs {
		modelTotal += int64(len(modelsByEp[id]))
	}

	pageInfo := &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: int64(len(matchedIDs))}
	log.Info("[UpstreamQuery] List upstream",
		zap.Int("groups", len(groups)), zap.Int64("modelTotal", modelTotal), zap.Int64("total", pageInfo.Total))
	return groups, modelTotal, pageInfo, nil
}

// loadUsers 一次性拉取本页 endpoint 与 model 归属用户，避免 N+1。
func (h *listUpstreamHandler) loadUsers(ctx context.Context, epsByID map[uint]*llmagg.Endpoint, modelsByEp map[uint][]*llmagg.Model) map[uint]*identityaggregate.User {
	ids := lo.Map(lo.Values(epsByID), func(ep *llmagg.Endpoint, _ int) uint { return ep.UserID() })
	for _, ms := range modelsByEp {
		for _, m := range ms {
			ids = append(ids, m.UserID())
		}
	}
	ids = lo.Uniq(ids)

	out := make(map[uint]*identityaggregate.User, len(ids))
	users, err := h.userRepo.BatchFindByIDs(ctx, ids)
	if err != nil {
		logger.WithCtx(ctx).Warn("[UpstreamQuery] Load users failed", zap.Error(err))
		return out
	}
	return users
}

// loadOAuthDisplays 批量拉取本页 OAuth 端点的凭据展示投影（一次 IN 查询，不解密 token）；
// 查询失败 fail-soft（该组 OAuth 视图缺省）。
func (h *listUpstreamHandler) loadOAuthDisplays(ctx context.Context, pageIDs []uint, epsByID map[uint]*llmagg.Endpoint) map[uint]vo.OAuthCredentialDisplay {
	oauthIDs := make([]uint, 0, len(pageIDs))
	for _, id := range pageIDs {
		if ep := epsByID[id]; ep != nil && ep.AuthType().IsOAuth() {
			oauthIDs = append(oauthIDs, id)
		}
	}
	if len(oauthIDs) == 0 {
		return map[uint]vo.OAuthCredentialDisplay{}
	}
	displays, err := h.credRepo.ListDisplayByEndpointIDs(ctx, oauthIDs)
	if err != nil {
		logger.WithCtx(ctx).Warn("[UpstreamQuery] Load oauth credential displays failed",
			zap.Int("endpointCount", len(oauthIDs)), zap.Error(err))
		return map[uint]vo.OAuthCredentialDisplay{}
	}
	return displays
}

func (h *listUpstreamHandler) toGroupView(ctx context.Context, ep *llmagg.Endpoint, models []*llmagg.Model, usersByID map[uint]*identityaggregate.User, cred *vo.OAuthCredentialDisplay, isDemo, maskOAuth bool) *port.UpstreamGroupView {
	mvs := lo.Map(models, func(m *llmagg.Model, _ int) *port.UpstreamModelView {
		return toModelView(m, usersByID, isDemo)
	})
	// totalModelCount 是截断前口径：UI 据此显示"N 中显示 M"，
	// 否则用户无法判断该端点到底有多少模型被藏起来了。
	totalModelCount := len(mvs)
	truncated := false
	if totalModelCount > constant.UpstreamGroupModelLimit {
		mvs = mvs[:constant.UpstreamGroupModelLimit]
		truncated = true
		logger.WithCtx(ctx).Warn("[UpstreamQuery] Group model list truncated",
			zap.String("endpoint", ep.Name()),
			zap.Int("total", totalModelCount),
			zap.Int("limit", constant.UpstreamGroupModelLimit))
	}
	return &port.UpstreamGroupView{
		Endpoint:        toEndpointView(ep, usersByID, cred, isDemo, maskOAuth),
		Models:          mvs,
		ModelCount:      len(mvs),
		TotalModelCount: totalModelCount,
		Truncated:       truncated,
	}
}

// ownsEndpoint 视角用户是否就是端点归属者（nil scope = admin 全量视角，视为归属者）。
func ownsEndpoint(scope *uint, ep *llmagg.Endpoint) bool {
	return ep == nil || scope == nil || *scope == ep.UserID()
}

func toEndpointView(ep *llmagg.Endpoint, usersByID map[uint]*identityaggregate.User, cred *vo.OAuthCredentialDisplay, isDemo, maskOAuth bool) *port.UpstreamEndpointView {
	if ep == nil {
		return nil
	}
	name := ep.Name()
	openaiBaseURL := ep.OpenaiBaseURL()
	anthropicBaseURL := ep.AnthropicBaseURL()
	if isDemo {
		openaiBaseURL = commonutil.MaskSecret(openaiBaseURL)
		anthropicBaseURL = commonutil.MaskSecret(anthropicBaseURL)
	}
	return &port.UpstreamEndpointView{
		ID:                          ep.AggregateID(),
		User:                        toUserView(usersByID[ep.UserID()]),
		Name:                        name,
		OpenaiBaseURL:               openaiBaseURL,
		AnthropicBaseURL:            anthropicBaseURL,
		MaskedAPIKey:                commonutil.MaskSecret(ep.APIKey()),
		AuthType:                    string(ep.AuthType()),
		Visibility:                  string(ep.Visibility()),
		OAuth:                       toOAuthView(ep.AuthType(), cred, maskOAuth),
		SupportOpenAIChatCompletion: ep.SupportOpenAIChatCompletion(),
		SupportOpenAIResponse:       ep.SupportOpenAIResponse(),
		SupportAnthropicMessage:     ep.SupportAnthropicMessage(),
		SupportOpenAIDecision:       ep.SupportOpenAIDecision(),
		CreatedAt:                   ep.CreatedAt(),
		UpdatedAt:                   ep.UpdatedAt(),
	}
}

// toOAuthView 装配订阅凭据投影。
//
// 数据源是凭据展示投影（vo.OAuthCredentialDisplay，结构上不含 token）；
// mask 视角（demo 或非归属者看公开端点）掩码邮箱并隐藏错误细节。
// 缺凭据行时同样返回视图（status=reauth_required）——与解析层 fail-closed 口径一致，
// 前端才能显示"需重新授权"而非静默无数据。
func toOAuthView(authType enum.EndpointAuthType, cred *vo.OAuthCredentialDisplay, mask bool) *port.UpstreamOAuthView {
	if !authType.IsOAuth() {
		return nil
	}
	if cred == nil {
		return &port.UpstreamOAuthView{
			Status: string(enum.OAuthCredentialStatusReauthRequired),
			Quota:  &vo.QuotaSnapshot{Windows: []vo.QuotaWindow{}},
		}
	}
	email := cred.AccountEmail
	lastError := cred.LastError
	if mask {
		email = commonutil.MaskIdentity(email)
		lastError = ""
	}
	quota := cred.Quota
	if quota.Windows == nil {
		quota.Windows = []vo.QuotaWindow{}
	}
	view := &port.UpstreamOAuthView{
		AccountEmail: email,
		PlanType:     cred.PlanType,
		Status:       string(cred.Status),
		LastError:    lastError,
		Quota:        &quota,
	}
	if !cred.QuotaUpdatedAt.IsZero() {
		updatedAt := cred.QuotaUpdatedAt
		view.QuotaUpdatedAt = &updatedAt
	}
	return view
}

func toModelView(m *llmagg.Model, usersByID map[uint]*identityaggregate.User, isDemo bool) *port.UpstreamModelView {
	upstreamModel := m.UpstreamModel()
	if isDemo {
		upstreamModel = commonutil.MaskSecret(upstreamModel)
	}
	return &port.UpstreamModelView{
		ID:              m.AggregateID(),
		User:            toUserView(usersByID[m.UserID()]),
		Alias:           m.Alias().String(),
		ModelID:         m.ModelID(),
		UpstreamModel:   upstreamModel,
		Enabled:         m.Enabled(),
		Priority:        m.Priority(),
		Weight:          m.Weight(),
		ContextLength:   m.ContextLength(),
		MaxOutputTokens: m.MaxOutputTokens(),
		Capabilities:    m.Capabilities(),
		Pricing:         m.Pricing(),
		ConfigMissing:   m.MissingConfig(),
		CreatedAt:       m.CreatedAt(),
		UpdatedAt:       m.UpdatedAt(),
	}
}

func toUserView(u *identityaggregate.User) *port.UpstreamUserView {
	if u == nil {
		return nil
	}
	return &port.UpstreamUserView{
		ID:     u.AggregateID(),
		Name:   string(u.Name()),
		Avatar: string(u.Avatar()),
	}
}
