// Package service Identity 域领域服务
package service

// TokenSigner Token 签名器接口
//
// 领域层使用该接口抽象 JWT 编解码能力，具体实现在 infrastructure/jwt
// （目前仍位于 internal/jwt，保留原位置 re-export；Step 6 统一搬迁）。
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type TokenSigner interface {
	EncodeToken(userID uint) (token string, err error)
	DecodeToken(tokenString string) (userID uint, err error)
}
