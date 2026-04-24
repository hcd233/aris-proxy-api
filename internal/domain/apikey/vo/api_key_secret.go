package vo

import "github.com/hcd233/aris-proxy-api/internal/common/util"

// APIKeySecret API Key 密钥值（对客户端隐藏的敏感数据）
//
// 作为值对象，统一处理：
//
//   - 脱敏展示（用于日志和列表响应）
//
//   - 不变性（一旦创建不能修改）
//
//     @author centonhuang
//     @update 2026-04-22 17:00:00
type APIKeySecret struct {
	value string
}

// NewAPIKeySecret 构造 API Key 密钥值对象
//
//	@param raw string
//	@return APIKeySecret
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewAPIKeySecret(raw string) APIKeySecret {
	return APIKeySecret{value: raw}
}

// Raw 返回原始密钥值（仅创建时返回给客户端；后续不可获取）
//
//	@receiver s APIKeySecret
//	@return string
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (s APIKeySecret) Raw() string { return s.value }

// Masked 返回脱敏后的密钥值（用于列表响应和日志）
//
//	@receiver s APIKeySecret
//	@return string
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (s APIKeySecret) Masked() string { return util.MaskSecret(s.value) }

// IsEmpty 判断密钥值是否为空
//
//	@receiver s APIKeySecret
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (s APIKeySecret) IsEmpty() bool { return s.value == "" }
