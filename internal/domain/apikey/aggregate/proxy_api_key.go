// Package aggregate APIKey 域聚合根
package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
)

// ProxyAPIKey API Key 聚合根
//
// 封装代理 API Key 的业务状态与行为，包括签发（Issue）与吊销（Revoke）。
//
//	@author centonhuang
//	@update 2026-04-23 10:45:00
type ProxyAPIKey struct {
	aggregate.Base

	userID    uint
	name      vo.APIKeyName
	secret    vo.APIKeySecret
	createdAt time.Time
}

// IssueProxyAPIKey 签发新的 API Key 聚合
//
// 参数 existing 为用户当前已持有的 API Key 数量，由 Command Handler 在
// 加载聚合前从仓储取得；聚合在此处校验配额。
//
//	@param userID uint
//	@param name vo.APIKeyName
//	@param secret vo.APIKeySecret
//	@param quota vo.APIKeyQuota
//	@param existing int64 用户当前已持有的 Key 数量
//	@return *ProxyAPIKey
//	@return error 配额不足时返回 ierr.ErrQuotaExceeded；名称/密钥为空时返回 ierr.ErrValidation
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func IssueProxyAPIKey(userID uint, name vo.APIKeyName, secret vo.APIKeySecret, quota vo.APIKeyQuota, existing int64, now time.Time) (*ProxyAPIKey, error) {
	if name.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "api key name is empty")
	}
	if secret.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "api key secret is empty")
	}
	if !quota.Allows(existing) {
		return nil, ierr.New(ierr.ErrQuotaExceeded, "api key count exceeds quota")
	}

	return &ProxyAPIKey{
		userID:    userID,
		name:      name,
		secret:    secret,
		createdAt: now,
	}, nil
}

// RestoreProxyAPIKey 从仓储重建 ProxyAPIKey 聚合
//
//	@param id uint
//	@param userID uint
//	@param name vo.APIKeyName
//	@param secret vo.APIKeySecret
//	@param createdAt time.Time
//	@return *ProxyAPIKey
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func RestoreProxyAPIKey(id uint, userID uint, name vo.APIKeyName, secret vo.APIKeySecret, createdAt time.Time) *ProxyAPIKey {
	k := &ProxyAPIKey{
		userID:    userID,
		name:      name,
		secret:    secret,
		createdAt: createdAt,
	}
	k.SetID(id)
	return k
}

// AggregateType 实现 aggregate.Root 接口
//
//	@receiver *ProxyAPIKey
//	@return string
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func (*ProxyAPIKey) AggregateType() string { return constant.AggregateTypeAPIKey }

// UserID 返回关联用户 ID
func (k *ProxyAPIKey) UserID() uint { return k.userID }

// Name 返回名称
func (k *ProxyAPIKey) Name() vo.APIKeyName { return k.name }

// Secret 返回密钥值对象
func (k *ProxyAPIKey) Secret() vo.APIKeySecret { return k.secret }

// CreatedAt 返回创建时间
func (k *ProxyAPIKey) CreatedAt() time.Time { return k.createdAt }

// IsOwnedBy 判断指定用户是否为 Key 所有者
//
// 仅当 Key 的 UserID 严格匹配传入 userID 时返回 true。UserID==0 的 legacy Key
// 不再视为任何用户"共有"（历史上曾如此兼容，但导致任意登录用户可越权吊销）；
// 此类数据应由 admin 通过 PermissionAdmin 分支单独处理或通过迁移脚本重置。
//
//	@receiver k *ProxyAPIKey
//	@param userID uint
//	@return bool
//	@author centonhuang
//	@update 2026-04-23 10:40:00
func (k *ProxyAPIKey) IsOwnedBy(userID uint) bool {
	return k.userID != 0 && k.userID == userID
}
