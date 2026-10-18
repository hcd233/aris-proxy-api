package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonagg "github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// OAuthCredential 订阅账号 OAuth 凭据聚合根
//
// 与 Endpoint 一对一（endpoint_id 唯一）。聚合持明文的 access/refresh token；
// 落库加密是持久化层职责（repository 加解密），领域层不感知密文。
type OAuthCredential struct {
	commonagg.Base

	endpointID     uint
	provider       enum.OAuthProvider
	accessToken    string
	refreshToken   string
	expiresAt      time.Time
	accountRef     string
	accountEmail   string
	planType       string
	status         enum.OAuthCredentialStatus
	lastError      string
	quota          vo.QuotaSnapshot
	quotaUpdatedAt time.Time
	createdAt      time.Time
	updatedAt      time.Time
}

// CreateOAuthCredential 构造凭据聚合根（必填校验）
func CreateOAuthCredential(endpointID uint, provider enum.OAuthProvider, accessToken, refreshToken string, expiresAt time.Time, accountRef, accountEmail, planType string) (*OAuthCredential, error) {
	cred := newOAuthCredential(endpointID, provider, accessToken, refreshToken, expiresAt, accountRef, accountEmail, planType)
	if err := cred.validate(); err != nil {
		return nil, err
	}
	return cred, nil
}

// RehydrateOAuthCredential 从持久化状态恢复凭据聚合根，不做不变量校验。
//
// 与 RehydrateEndpoint 同理：恢复失败的代价是凭据读取不可用（转发链路拿不到 token），
// 校验型工厂只该守在写边界上。
func RehydrateOAuthCredential(
	id, endpointID uint,
	provider enum.OAuthProvider,
	accessToken, refreshToken string,
	expiresAt time.Time,
	accountRef, accountEmail, planType string,
	status enum.OAuthCredentialStatus,
	lastError string,
	quota vo.QuotaSnapshot,
	quotaUpdatedAt time.Time,
) *OAuthCredential {
	cred := newOAuthCredential(endpointID, provider, accessToken, refreshToken, expiresAt, accountRef, accountEmail, planType)
	cred.SetID(id)
	cred.status = status
	cred.lastError = lastError
	cred.quota = quota
	cred.quotaUpdatedAt = quotaUpdatedAt
	return cred
}

func newOAuthCredential(endpointID uint, provider enum.OAuthProvider, accessToken, refreshToken string, expiresAt time.Time, accountRef, accountEmail, planType string) *OAuthCredential {
	return &OAuthCredential{
		endpointID:   endpointID,
		provider:     provider,
		accessToken:  accessToken,
		refreshToken: refreshToken,
		expiresAt:    expiresAt,
		accountRef:   accountRef,
		accountEmail: accountEmail,
		planType:     planType,
		status:       enum.OAuthCredentialStatusActive,
		quota:        vo.QuotaSnapshot{Windows: []vo.QuotaWindow{}},
	}
}

func (c *OAuthCredential) validate() error {
	// endpointID 允许为 0：创建路径先建端点、事务内回填（见 SetEndpointID）；
	// 恢复路径不校验。归属完整性由凭据行 endpoint_id 唯一索引保证。
	switch c.provider {
	case enum.OAuthProviderAnthropic, enum.OAuthProviderOpenAI:
	default:
		return ierr.New(ierr.ErrValidation, "oauth credential provider is invalid")
	}
	if c.accessToken == "" {
		return ierr.New(ierr.ErrValidation, "oauth credential access token is required")
	}
	if c.refreshToken == "" {
		return ierr.New(ierr.ErrValidation, "oauth credential refresh token is required")
	}
	return nil
}

// SetEndpointID 绑定所属端点 ID（创建路径在端点落库后回填）
func (c *OAuthCredential) SetEndpointID(endpointID uint) { c.endpointID = endpointID }

// ApplyTokens 刷新成功后更新令牌。
//
// refreshToken 为空时保留原值（Claude 刷新可能不轮换 refresh token，空值覆盖会把凭据刷废）；
// 同时清空 lastError 并恢复 active——刷新成功即凭据可用。
func (c *OAuthCredential) ApplyTokens(accessToken, refreshToken string, expiresAt time.Time) {
	c.accessToken = accessToken
	if refreshToken != "" {
		c.refreshToken = refreshToken
	}
	c.expiresAt = expiresAt
	c.lastError = ""
	c.status = enum.OAuthCredentialStatusActive
}

// SetLastError 记录非致命错误（如额度探测失败）供展示；不改状态机（区别于 MarkReauth）。
func (c *OAuthCredential) SetLastError(errMsg string) { c.lastError = errMsg }

// MarkReauth 标记为需重新授权（刷新被上游拒绝，如 invalid_grant）。
func (c *OAuthCredential) MarkReauth(errMsg string) {
	c.status = enum.OAuthCredentialStatusReauthRequired
	c.lastError = errMsg
}

// SetQuota 写入额度快照与采集时间。
func (c *OAuthCredential) SetQuota(snapshot vo.QuotaSnapshot, at time.Time) {
	if snapshot.Windows == nil {
		snapshot.Windows = []vo.QuotaWindow{}
	}
	c.quota = snapshot
	c.quotaUpdatedAt = at
}

// EndpointID 返回所属端点 ID
func (c *OAuthCredential) EndpointID() uint { return c.endpointID }

// Provider 返回 OAuth 提供方
func (c *OAuthCredential) Provider() enum.OAuthProvider { return c.provider }

// AccessToken 返回明文 access token（仅限内存链路使用，禁止进日志/响应）
func (c *OAuthCredential) AccessToken() string { return c.accessToken }

// RefreshToken 返回明文 refresh token（仅限刷新链路使用，禁止进日志/响应）
func (c *OAuthCredential) RefreshToken() string { return c.refreshToken }

// ExpiresAt 返回 access token 到期时间
func (c *OAuthCredential) ExpiresAt() time.Time { return c.expiresAt }

// AccountRef 返回账号标识（Claude organization uuid / Codex chatgpt account id）
func (c *OAuthCredential) AccountRef() string { return c.accountRef }

// AccountEmail 返回账号邮箱（展示用）
func (c *OAuthCredential) AccountEmail() string { return c.accountEmail }

// PlanType 返回订阅类型（展示用）
func (c *OAuthCredential) PlanType() string { return c.planType }

// Status 返回凭据状态
func (c *OAuthCredential) Status() enum.OAuthCredentialStatus { return c.status }

// LastError 返回最近一次错误（脱敏后展示）
func (c *OAuthCredential) LastError() string { return c.lastError }

// Quota 返回额度快照
func (c *OAuthCredential) Quota() vo.QuotaSnapshot { return c.quota }

// QuotaUpdatedAt 返回额度快照时间
func (c *OAuthCredential) QuotaUpdatedAt() time.Time { return c.quotaUpdatedAt }

// SetTimestamps 设置创建/更新时间（repository 从 DB 恢复时使用）
func (c *OAuthCredential) SetTimestamps(createdAt, updatedAt time.Time) {
	c.createdAt = createdAt
	c.updatedAt = updatedAt
}

// CreatedAt 返回创建时间
func (c *OAuthCredential) CreatedAt() time.Time { return c.createdAt }

// UpdatedAt 返回更新时间
func (c *OAuthCredential) UpdatedAt() time.Time { return c.updatedAt }
