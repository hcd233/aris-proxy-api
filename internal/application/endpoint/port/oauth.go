package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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
	CodeInput      string
	State          string
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

// GetOAuthQuotaHandler 订阅端点额度查询处理器
type GetOAuthQuotaHandler interface {
	Handle(ctx context.Context, q GetOAuthQuotaQuery) (*OAuthQuotaView, error)
}
