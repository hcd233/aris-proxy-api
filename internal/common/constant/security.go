package constant

const (
	MaskSecretMinLength   = 8
	MaskSecretPlaceholder = "***"
	MaskSecretTemplate    = "%s***%s"
	ByteMax               = 256

	// SecretBoxKeyLength OAuth 凭据加密密钥长度（AES-256-GCM，32 字节 = hex 64 字符）
	SecretBoxKeyLength = 32
)
