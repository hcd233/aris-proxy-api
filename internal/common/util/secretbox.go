package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// EncryptSecret 使用 AES-256-GCM 加密明文，返回 base64(nonce||ciphertext||tag)。
//
// 密钥必须为 constant.SecretBoxKeyLength 字节；缺失或长度不符时快速失败（fail-closed）——
// 绝不在密钥异常时降级为明文存储。
func EncryptSecret(key []byte, plaintext string) (string, error) {
	gcm, err := newSecretBoxGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "read nonce")
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptSecret 解密 EncryptSecret 的输出；密文被篡改、密钥不符或格式非法时返回错误。
func DecryptSecret(key []byte, ciphertext string) (string, error) {
	gcm, err := newSecretBoxGCM(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrValidation, err, "decode ciphertext")
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", ierr.New(ierr.ErrValidation, "ciphertext is shorter than nonce")
	}
	plaintext, err := gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrValidation, err, "decrypt ciphertext")
	}
	return string(plaintext), nil
}

// newSecretBoxGCM 构造 AES-256-GCM AEAD；密钥长度错误时返回 ErrValidation。
func newSecretBoxGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != constant.SecretBoxKeyLength {
		return nil, ierr.Newf(ierr.ErrValidation, "secret box key must be %d bytes, got %d", constant.SecretBoxKeyLength, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "create aes cipher")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "create gcm aead")
	}
	return gcm, nil
}
