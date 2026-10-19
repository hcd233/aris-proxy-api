package constant

const (
	MaskSecretMinLength   = 8
	MaskSecretPlaceholder = "***"
	MaskSecretTemplate    = "%s***%s"
	ByteMax               = 256

	// SecretBoxKeyLength OAuth 凭据加密密钥长度（AES-256-GCM，32 字节 = hex 64 字符）
	SecretBoxKeyLength = 32

	// SecretBoxCiphertextPrefixV1 凭据密文版本前缀（v1 = AES-256-GCM，
	// base64(nonce||ciphertext||tag)）；解密兼容无前缀的历史密文，
	// 将来轮换密钥/算法时据此区分版本。
	SecretBoxCiphertextPrefixV1 = "v1:"
)
