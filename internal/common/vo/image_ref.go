package vo

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
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

// ImageRefSHAs 返回 message 中 image_url 部分引用的图片 sha256（按出现顺序，可能重复）
//
//	只认 type=image_url 且值为合法引用形态的部分；文本中出现的同名字符串不算引用。
//
//	@param msg *UnifiedMessage
//	@return []string
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func ImageRefSHAs(msg *UnifiedMessage) []string {
	if msg == nil || msg.Content == nil {
		return nil
	}
	var shas []string
	for _, p := range msg.Content.Parts {
		if p == nil || p.Type != enum.ContentPartTypeImageURL {
			continue
		}
		if sha, ok := ParseImageRef(p.ImageURL); ok {
			shas = append(shas, sha)
		}
	}
	return shas
}

// RewriteImageParts 写时复制地改写 image_url 部分
//
//	fn 返回非 nil 时替换该部分；有部分被替换的消息是副本，其余保持原指针，不修改入参。
//
//	@param msgs []*UnifiedMessage
//	@param fn func(msgIdx, partIdx int, part *UnifiedContentPart) *UnifiedContentPart
//	@return []*UnifiedMessage
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func RewriteImageParts(msgs []*UnifiedMessage, fn func(msgIdx, partIdx int, part *UnifiedContentPart) *UnifiedContentPart) []*UnifiedMessage {
	out, cloned := msgs, false
	for mi, m := range msgs {
		if m == nil || m.Content == nil {
			continue
		}
		var parts []*UnifiedContentPart
		for pi, p := range m.Content.Parts {
			if p == nil || p.Type != enum.ContentPartTypeImageURL {
				continue
			}
			next := fn(mi, pi, p)
			if next == nil {
				continue
			}
			if parts == nil {
				parts = slices.Clone(m.Content.Parts)
			}
			parts[pi] = next
		}
		if parts == nil {
			continue
		}
		if !cloned {
			out, cloned = slices.Clone(msgs), true
		}
		cp := *m
		cp.Content = &UnifiedContent{Text: m.Content.Text, Parts: parts}
		out[mi] = &cp
	}
	return out
}

// NeutralizeImageRefs 把客户端提交的图片引用改写为文本部分
//
//	引用形态只能由服务端外置图片时生成。客户端自带的 aris-blob 引用若原样入库，
//	持有者就能凭"本会话引用了该图"的校验读取别人的图片，因此落库前一律降级为文本。
//
//	@param msgs []*UnifiedMessage 来自客户端请求与上游响应的消息
//	@return out []*UnifiedMessage
//	@return n int 被改写的部分数
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func NeutralizeImageRefs(msgs []*UnifiedMessage) (out []*UnifiedMessage, n int) {
	out = RewriteImageParts(msgs, func(_, _ int, p *UnifiedContentPart) *UnifiedContentPart {
		if !strings.HasPrefix(p.ImageURL, constant.BlobRefPrefix) {
			return nil
		}
		n++
		return &UnifiedContentPart{Type: enum.ContentPartTypeText, Text: p.ImageURL}
	})
	return out, n
}
