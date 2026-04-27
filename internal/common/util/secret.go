// Package util 提供跨层共享的纯基础工具函数。
//
// 位于 common/ 下的 util 允许被 domain/application/infrastructure 任意层调用，
// 与 internal/util（提供 HTTP / SSE / Context / DTO 相关的基础设施感知工具）
// 形成依赖边界：domain 层禁止依赖 internal/util，但可依赖本包。
//
//	@author centonhuang
//	@update 2026-04-23 10:55:00
package util

import (
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// MaskSecret 掩码敏感信息，保留前 4 和后 4 个字符
//
// 长度 <=8 的秘密返回固定占位符，避免反推原始值。
//
//	@param key string
//	@return string
//	@author centonhuang
//	@update 2026-04-23 10:55:00
func MaskSecret(key string) string {
	if len(key) <= constant.MaskSecretMinLength {
		return constant.MaskSecretPlaceholder
	}
	return fmt.Sprintf("%s***%s", key[:4], key[len(key)-4:])
}
