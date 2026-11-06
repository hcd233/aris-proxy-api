package query

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/logger"
)

// ExportImageResolver 导出时把图片内联为 data URI，一次导出内按 sha256 缓存
//
//	单个会话内联的 data URI 总字节受 BlobExportSessionMaxBytes 限制，超出的图片连同占位符一起跳过，
//	避免一行 JSON 把内存撑爆。
//
//	@author centonhuang
//	@update 2026-11-06 10:00:00
type ExportImageResolver struct {
	ctx          context.Context //nolint:containedctx // 解析器生命周期即一次导出请求
	reader       blob.ImageReader
	cache        map[string]string
	used         int
	sessionBytes int
	failed       int
	skipped      int
}

// NewExportImageResolver 构造导出图片解析器
//
//	@param ctx context.Context
//	@param reader blob.ImageReader 可为 nil：此时引用形态的图片一律跳过
//	@return *ExportImageResolver
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func NewExportImageResolver(ctx context.Context, reader blob.ImageReader) *ExportImageResolver {
	return &ExportImageResolver{ctx: ctx, reader: reader, cache: map[string]string{}}
}

// BeginSession 开始导出一个新会话，重置单会话图片字节预算
//
//	@receiver r *ExportImageResolver
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func (r *ExportImageResolver) BeginSession() { r.sessionBytes = 0 }

// Resolve 引用形态取回字节内联；data URI 与外部 URL 原样输出；取图失败或超出会话预算时跳过
//
//	@receiver r *ExportImageResolver
//	@param part *vo.UnifiedContentPart
//	@return string
//	@return bool
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func (r *ExportImageResolver) Resolve(part *vo.UnifiedContentPart) (string, bool) {
	sha, isRef := vo.ParseImageRef(part.ImageURL)
	if !isRef {
		if !strings.HasPrefix(part.ImageURL, constant.DataURLPrefix) {
			return part.ImageURL, part.ImageURL != ""
		}
		return r.charge(part.ImageURL)
	}
	if v, hit := r.cache[sha]; hit {
		return r.charge(v)
	}
	if r.reader == nil {
		r.failed++
		return "", false
	}
	mime, data, err := r.reader.ReadBytes(r.ctx, sha)
	if err != nil {
		logger.WithCtx(r.ctx).Warn("[DatasetExport] Failed to read image, skipped", zap.String("sha256", sha), zap.Error(err))
		r.failed++
		return "", false
	}
	v := fmt.Sprintf(constant.DataURLTemplate, mime, base64.StdEncoding.EncodeToString(data))
	// ponytail: 超限整表清空，够用；需要时换 LRU
	if r.used+len(v) > constant.BlobExportCacheMaxBytes {
		clear(r.cache)
		r.used = 0
	}
	if len(v) <= constant.BlobExportCacheMaxBytes {
		r.cache[sha] = v
		r.used += len(v)
	}
	return r.charge(v)
}

// charge 把一张内联图计入当前会话预算；超出时跳过
func (r *ExportImageResolver) charge(v string) (string, bool) {
	if r.sessionBytes+len(v) > constant.BlobExportSessionMaxBytes {
		r.skipped++
		return "", false
	}
	r.sessionBytes += len(v)
	return v, true
}

// Failed 本次导出中取图失败的次数
//
//	@receiver r *ExportImageResolver
//	@return int
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func (r *ExportImageResolver) Failed() int { return r.failed }

// Skipped 本次导出中因超出单会话预算被跳过的图片数
//
//	@receiver r *ExportImageResolver
//	@return int
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func (r *ExportImageResolver) Skipped() int { return r.skipped }

// PreviewImageResolver 格式预览不取图，只显示类型与大小
//
//	@param part *vo.UnifiedContentPart
//	@return string
//	@return bool
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func PreviewImageResolver(part *vo.UnifiedContentPart) (string, bool) {
	if _, isRef := vo.ParseImageRef(part.ImageURL); isRef {
		return fmt.Sprintf(constant.DatasetPreviewImageTemplate, part.ImageMime, kib(part.ImageSize)), true
	}
	if mime, raw, ok := vo.ParseImageDataURI(part.ImageURL); ok {
		return fmt.Sprintf(constant.DatasetPreviewImageTemplate, mime, kib(int64(len(raw)))), true
	}
	if strings.HasPrefix(part.ImageURL, constant.DataURLPrefix) {
		return "", false
	}
	return part.ImageURL, part.ImageURL != ""
}

func kib(n int64) int64 { return (n + constant.BytesPerKiB - 1) / constant.BytesPerKiB }
