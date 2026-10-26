package port

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// OAuthQuotaWindowView 额度窗口视图（应用层扁平结构，handler 逐字段映射到 DTO）
type OAuthQuotaWindowView struct {
	Key           string
	UsedPercent   float64
	ResetsAt      time.Time
	WindowMinutes int
}

// OAuthQuotaSnapshotView 额度快照视图
type OAuthQuotaSnapshotView struct {
	Windows    []OAuthQuotaWindowView
	PlanType   string
	CapturedAt time.Time
}

// ExchangeOAuthCommand 用授权码完成订阅账号绑定
//
// OperatorUserID 发起授权流的用户（与 flow 记录归属比对，防跨用户重放）；
// OwnerUserID 端点归属（admin 代建时为被代建用户，普通用户两者相同）。
type ExchangeOAuthCommand struct {
	OperatorUserID uint
	OwnerUserID    uint
	Name           string
	Provider       enum.OAuthProvider
	CodeInput      string
	State          string
}

// ExchangeOAuthResult 绑定结果
type ExchangeOAuthResult struct {
	EndpointID uint
}

// ExchangeOAuthHandler 订阅账号绑定处理器
type ExchangeOAuthHandler interface {
	Handle(ctx context.Context, cmd ExchangeOAuthCommand) (*ExchangeOAuthResult, error)
}

// ReauthOAuthCommand 重新授权既有订阅端点
//
// ScopeUserID 多租户隔离三态：nil（admin）不过滤；非 nil 精确匹配 user_id。
type ReauthOAuthCommand struct {
	OperatorUserID uint
	ScopeUserID    *uint
	EndpointID     uint
	// IsAdmin 降级保护：非 admin 不得重新授权已是 public 的端点。
	IsAdmin   bool
	CodeInput string
	State     string
}

// ReauthOAuthHandler 订阅账号重新授权处理器
type ReauthOAuthHandler interface {
	Handle(ctx context.Context, cmd ReauthOAuthCommand) error
}

// GetOAuthQuotaQuery 查询订阅端点额度快照
//
// ScopeUserID 多租户隔离三态：nil（admin）不过滤；非 nil 精确匹配 user_id。
type GetOAuthQuotaQuery struct {
	ScopeUserID *uint
	EndpointID  uint
}

// OAuthQuotaView 额度查询视图（凭据本身永不出现在响应中）
type OAuthQuotaView struct {
	Status         string
	PlanType       string
	LastError      string
	Quota          *OAuthQuotaSnapshotView
	QuotaUpdatedAt *time.Time
}

// NewOAuthQuotaView 凭据聚合 → 额度视图（只含展示字段，token 不入视图）。
//
// 查询与手动探测两个入口共用同一映射：各自维护一份时，字段一旦漂移，
// 同一端点会在两个接口返回不同形态。
func NewOAuthQuotaView(cred *aggregate.OAuthCredential) *OAuthQuotaView {
	quota := cred.Quota()
	windows := lo.Map(quota.Windows, func(w vo.QuotaWindow, _ int) OAuthQuotaWindowView {
		return OAuthQuotaWindowView{
			Key:           w.Key,
			UsedPercent:   w.UsedPercent,
			ResetsAt:      w.ResetsAt,
			WindowMinutes: w.WindowMinutes,
		}
	})
	view := &OAuthQuotaView{
		Status:    string(cred.Status()),
		PlanType:  cred.PlanType(),
		LastError: cred.LastError(),
		Quota:     &OAuthQuotaSnapshotView{Windows: windows, PlanType: quota.PlanType, CapturedAt: quota.CapturedAt},
	}
	if updatedAt := cred.QuotaUpdatedAt(); !updatedAt.IsZero() {
		view.QuotaUpdatedAt = &updatedAt
	}
	return view
}

// GetOAuthQuotaHandler 订阅端点额度查询处理器
type GetOAuthQuotaHandler interface {
	Handle(ctx context.Context, q GetOAuthQuotaQuery) (*OAuthQuotaView, error)
}

// ProbeOAuthQuotaCommand 手动触发一次额度探测（同步，超时由 handler 控制）
//
// ScopeUserID 多租户隔离三态：nil（admin）不过滤；非 nil 精确匹配 user_id。
type ProbeOAuthQuotaCommand struct {
	ScopeUserID *uint
	EndpointID  uint
}

// ProbeOAuthQuotaHandler 订阅端点额度探测处理器（返回探测后的最新视图）
type ProbeOAuthQuotaHandler interface {
	Handle(ctx context.Context, cmd ProbeOAuthQuotaCommand) (*OAuthQuotaView, error)
}
