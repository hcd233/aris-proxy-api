package secret

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
)

// testSecretBoxKey 返回 32 字节固定密钥（AES-256 合法长度）。
func testSecretBoxKey() []byte { return []byte("0123456789abcdef0123456789abcdef") }

// TestSecretBoxRoundTrip 加密→解密得原文；同一明文两次密文不同（nonce 随机）。
func TestSecretBoxRoundTrip(t *testing.T) {
	t.Parallel()

	const plain = "oauth-refresh-token-value"
	enc, err := commonutil.EncryptSecret(testSecretBoxKey(), plain)
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	if enc == plain || enc == "" {
		t.Fatal("ciphertext must be non-empty and differ from plaintext")
	}

	dec, err := commonutil.DecryptSecret(testSecretBoxKey(), enc)
	if err != nil {
		t.Fatalf("DecryptSecret: %v", err)
	}
	if dec != plain {
		t.Fatalf("round trip mismatch: got %q want %q", dec, plain)
	}

	enc2, err := commonutil.EncryptSecret(testSecretBoxKey(), plain)
	if err != nil {
		t.Fatalf("EncryptSecret second call: %v", err)
	}
	if enc2 == enc {
		t.Fatal("two encryptions of the same plaintext must differ (random nonce)")
	}
}

// TestSecretBoxVersionPrefix 密文带版本前缀，且解密兼容加前缀之前写入的历史密文。
func TestSecretBoxVersionPrefix(t *testing.T) {
	t.Parallel()

	const plain = "oauth-access-token-value"
	enc, err := commonutil.EncryptSecret(testSecretBoxKey(), plain)
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	if !strings.HasPrefix(enc, constant.SecretBoxCiphertextPrefixV1) {
		t.Fatalf("ciphertext must carry version prefix %q, got %q", constant.SecretBoxCiphertextPrefixV1, enc)
	}

	// 无前缀（加前缀之前写入的行）按 v1 处理，升级不要求一次性重写数据
	legacy := strings.TrimPrefix(enc, constant.SecretBoxCiphertextPrefixV1)
	dec, err := commonutil.DecryptSecret(testSecretBoxKey(), legacy)
	if err != nil || dec != plain {
		t.Fatalf("legacy ciphertext decrypt = %q, err = %v, want %q", dec, err, plain)
	}
}

// TestSecretBoxWrongKeyFails 错误密钥解密必须失败（GCM 认证）。
func TestSecretBoxWrongKeyFails(t *testing.T) {
	t.Parallel()

	enc, err := commonutil.EncryptSecret(testSecretBoxKey(), "secret")
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	other := []byte("abcdef0123456789abcdef0123456789")
	if _, err := commonutil.DecryptSecret(other, enc); err == nil {
		t.Fatal("decrypt with wrong key must fail")
	}
}

// TestSecretBoxTamperFails 篡改密文任一字节必须失败（完整性校验）。
func TestSecretBoxTamperFails(t *testing.T) {
	t.Parallel()

	enc, err := commonutil.EncryptSecret(testSecretBoxKey(), "secret")
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(enc, constant.SecretBoxCiphertextPrefixV1))
	if err != nil {
		t.Fatalf("ciphertext payload must be base64: %v", err)
	}
	raw[len(raw)/2] ^= 0xFF
	tampered := constant.SecretBoxCiphertextPrefixV1 + base64.StdEncoding.EncodeToString(raw)

	if _, err := commonutil.DecryptSecret(testSecretBoxKey(), tampered); err == nil {
		t.Fatal("tampered ciphertext must fail")
	}
}

// TestSecretBoxRejectsBadKeyLength 密钥长度非 32 字节或缺失时快速失败（fail-closed）。
func TestSecretBoxRejectsBadKeyLength(t *testing.T) {
	t.Parallel()

	if _, err := commonutil.EncryptSecret([]byte("short"), "x"); err == nil {
		t.Fatal("non-32-byte key must be rejected")
	}
	if _, err := commonutil.EncryptSecret(nil, "x"); err == nil {
		t.Fatal("nil key must be rejected")
	}
}
