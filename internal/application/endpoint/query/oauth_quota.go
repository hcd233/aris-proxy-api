package query

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
)

type getOAuthQuotaHandler struct {
	endpointRepo llmproxy.EndpointRepository
	credRepo     llmproxy.OAuthCredentialRepository
}

// NewGetOAuthQuotaHandler 构造订阅端点额度查询处理器
func NewGetOAuthQuotaHandler(endpointRepo llmproxy.EndpointRepository, credRepo llmproxy.OAuthCredentialRepository) port.GetOAuthQuotaHandler {
	return &getOAuthQuotaHandler{endpointRepo: endpointRepo, credRepo: credRepo}
}

// Handle 返回订阅端点的凭据状态与额度快照。
//
// 视图只暴露状态/订阅类型/错误摘要/用量窗口——token 字段永不下发。
// 缺凭据行时返回空状态视图（视为 reauth_required，与解析层 fail-closed 口径一致）。
func (h *getOAuthQuotaHandler) Handle(ctx context.Context, q port.GetOAuthQuotaQuery) (*port.OAuthQuotaView, error) {
	ep, err := h.endpointRepo.FindByID(ctx, q.EndpointID, q.ScopeUserID)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "endpoint not found")
	}
	if !ep.AuthType().IsOAuth() {
		return nil, ierr.New(ierr.ErrValidation, "endpoint is not an oauth subscription endpoint")
	}

	cred, err := h.credRepo.FindByEndpointID(ctx, q.EndpointID)
	if err != nil {
		return nil, err
	}
	view := &port.OAuthQuotaView{Status: string(enum.OAuthCredentialStatusReauthRequired)}
	if cred == nil {
		return view, nil
	}
	view.Status = string(cred.Status())
	view.PlanType = cred.PlanType()
	view.LastError = cred.LastError()
	quota := cred.Quota()
	windows := make([]port.OAuthQuotaWindowView, 0, len(quota.Windows))
	for _, w := range quota.Windows {
		windows = append(windows, port.OAuthQuotaWindowView{
			Key:           w.Key,
			UsedPercent:   w.UsedPercent,
			ResetsAt:      w.ResetsAt,
			WindowMinutes: w.WindowMinutes,
		})
	}
	view.Quota = &port.OAuthQuotaSnapshotView{Windows: windows, PlanType: quota.PlanType, CapturedAt: quota.CapturedAt}
	updatedAt := cred.QuotaUpdatedAt()
	if !updatedAt.IsZero() {
		view.QuotaUpdatedAt = &updatedAt
	}
	return view, nil
}
