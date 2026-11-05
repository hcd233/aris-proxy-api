package vo

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/hcd233/aris-mass/internal/common/constant"
)

var base64Whitespace = strings.NewReplacer("\r", "", "\n", "")

// ParseImageDataURI 解析 base64 形态的 data URI
//
//	兼容 mime 后带参数（;charset=...）、无 padding、夹带换行的写法。
//
//	@param url string
//	@return mime string data URI 声明的 mime（仅供参考，不可信）
//	@return raw []byte 解码后的原始字节
//	@return ok bool 非 base64 data URI 或解码失败时为 false
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func ParseImageDataURI(url string) (mime string, raw []byte, ok bool) {
	rest, found := strings.CutPrefix(url, constant.DataURLPrefix)
	if !found {
		return "", nil, false
	}
	header, payload, found := strings.Cut(rest, constant.DataURLBase64Separator)
	if !found {
		return "", nil, false
	}
	mime, _, _ = strings.Cut(header, ";")
	payload = base64Whitespace.Replace(payload)
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil {
		return "", nil, false
	}
	return mime, raw, true
}

// ParseImageRef 解析引用形态 aris-blob:sha256:<hex>
//
//	@param url string
//	@return sha string 64 位小写 hex
//	@return ok bool
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func ParseImageRef(url string) (sha string, ok bool) {
	sha, found := strings.CutPrefix(url, constant.BlobRefPrefix)
	if !found || !IsSHA256Hex(sha) {
		return "", false
	}
	return sha, true
}

// IsSHA256Hex 判断是否为 64 位小写 hex
//
//	@param s string
//	@return bool
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func IsSHA256Hex(s string) bool {
	if len(s) != constant.BlobSHA256HexLen {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool {
		return (r < '0' || r > '9') && (r < 'a' || r > 'f')
	})
}

// ImageRefOf 由 sha256 hex 构造引用形态
//
//	@param sha string
//	@return string
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func ImageRefOf(sha string) string {
	return constant.BlobRefPrefix + sha
}

// SHA256Hex 计算字节的 sha256 小写 hex
//
//	@param raw []byte
//	@return string
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func SHA256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// imageChecksumKey 图片部分参与 checksum 的规范值；非托管图片返回 ok=false
func imageChecksumKey(url string) (string, bool) {
	if sha, ok := ParseImageRef(url); ok {
		return constant.BlobChecksumPrefix + sha, true
	}
	if _, raw, ok := ParseImageDataURI(url); ok {
		return constant.BlobChecksumPrefix + SHA256Hex(raw), true
	}
	return "", false
}
