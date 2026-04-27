// Package service APIKey 域领域服务
package service

import (
	"crypto/rand"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/vo"
)

// APIKeyGenerator API Key 密钥生成领域服务
//
// 使用 rejection sampling 避免字节分布偏差，逻辑等价于原
// service.generateAPIKey（矩阵 / 源码层行为一致）。
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type APIKeyGenerator interface {
	Generate() (vo.APIKeySecret, error)
}

// apiKeyGenerator APIKeyGenerator 的默认实现（crypto/rand）
type apiKeyGenerator struct{}

// NewAPIKeyGenerator 构造生成器
//
//	@return APIKeyGenerator
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewAPIKeyGenerator() APIKeyGenerator {
	return &apiKeyGenerator{}
}

// Generate 生成一个新的 API Key 密钥值对象
//
//	@receiver g *apiKeyGenerator
//	@return vo.APIKeySecret
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (g *apiKeyGenerator) Generate() (vo.APIKeySecret, error) {
	charsetLen := byte(len(constant.APIKeyCharset))
	// rejection sampling: 只保留 [0, ByteMax - ByteMax%charsetLen) 范围内的字节，避免分布偏差
	maxAccepted := byte(constant.ByteMax - constant.ByteMax%int(charsetLen)) //nolint:gosec // G115 ByteMax-ByteMax%charsetLen 保证在 byte 范围内
	result := make([]byte, constant.APIKeyRandomLength)
	buf := make([]byte, constant.APIKeyRandomLength*2)
	filled := 0
	for filled < constant.APIKeyRandomLength {
		if _, err := rand.Read(buf); err != nil {
			return vo.APIKeySecret{}, ierr.Wrap(ierr.ErrInternal, err, "generate random bytes for API key")
		}
		for _, b := range buf {
			if filled >= constant.APIKeyRandomLength {
				break
			}
			if b < maxAccepted {
				result[filled] = constant.APIKeyCharset[int(b)%int(charsetLen)]
				filled++
			}
		}
	}
	return vo.NewAPIKeySecret(constant.APIKeyPrefix + string(result))
}
