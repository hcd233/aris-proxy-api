package vo

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// OAuthCredentialDisplay 订阅凭据的只读展示投影。
//
// 结构上不包含任何 token 字段：管理端列表/详情只需要账号、状态、额度，
// 用投影读取可避免把明文 access/refresh token 带进内存与 HTTP 视图。
type OAuthCredentialDisplay struct {
	EndpointID     uint
	Provider       enum.OAuthProvider
	AccountEmail   string
	PlanType       string
	Status         enum.OAuthCredentialStatus
	LastError      string
	Quota          QuotaSnapshot
	QuotaUpdatedAt time.Time
}
