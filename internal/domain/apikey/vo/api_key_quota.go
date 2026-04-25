package vo

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// APIKeyQuota API Key 配额值对象
//
// 表示每个用户可持有的最大 API Key 数量。配额超出由聚合根自身校验。
//
//	@author centonhuang
//	@update 2026-04-25 15:00:00
type APIKeyQuota struct {
	max int
}

// NewAPIKeyQuota 创建配额值对象
//
//	@param max int
//	@return APIKeyQuota
//	@author centonhuang
//	@update 2026-04-25 15:00:00
func NewAPIKeyQuota(max int) APIKeyQuota {
	return APIKeyQuota{max: max}
}

// DefaultAPIKeyQuota 返回默认配额（来自 constant.APIKeyMaxCount）
//
//	@return APIKeyQuota
//	@author centonhuang
//	@update 2026-04-25 15:00:00
func DefaultAPIKeyQuota() APIKeyQuota {
	return APIKeyQuota{max: constant.APIKeyMaxCount}
}

// MaxCount 返回配额上限
//
//	@receiver q APIKeyQuota
//	@return int
//	@author centonhuang
//	@update 2026-04-25 15:00:00
func (q APIKeyQuota) MaxCount() int {
	return q.max
}

// Allows 判断在当前已有数量下是否允许新建
//
//	@receiver q APIKeyQuota
//	@param existing int64
//	@return bool
//	@author centonhuang
//	@update 2026-04-25 15:00:00
func (q APIKeyQuota) Allows(existing int64) bool {
	return existing < int64(q.max)
}
