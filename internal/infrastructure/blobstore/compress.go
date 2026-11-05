// Package blobstore 消息图片外置：压缩、写入 blobs 表与对象存储、读取、重试与存量迁移
package blobstore

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"sync"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // 注册 webp 解码器

	"github.com/hcd233/aris-mass/internal/common/constant"
)

// CompressResult 压缩结果
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type CompressResult struct {
	Data          []byte
	Mime          string
	Width, Height int
	// Unprocessable 无法解码且超过存储上限：Data 为原始字节，不上传
	Unprocessable bool
}

// decodeMu 解码到编码全程串行：一张 5000 万像素图解码约占 200MB，pod 上限 512Mi
//
// ponytail: 全局串行，吞吐不够时换成容量为 N 的信号量
var decodeMu sync.Mutex

// Compress 把原始图片字节压成不超过 2MB 的存储版本
//
//	小图原样保留；超限时缩放并重新编码；无法处理时原样保留（≤2MB）或标记 Unprocessable。
//
//	@param raw []byte
//	@return CompressResult
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func Compress(raw []byte) CompressResult {
	mime := detectMime(raw)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width*cfg.Height > constant.BlobMaxDecodePixels {
		return verbatim(raw, mime)
	}
	if len(raw) <= constant.BlobMaxStoredBytes && max(cfg.Width, cfg.Height) <= constant.BlobMaxLongEdge {
		return CompressResult{Data: raw, Mime: mime, Width: cfg.Width, Height: cfg.Height}
	}
	if res, ok := reencode(raw, mime); ok {
		return res
	}
	return verbatim(raw, mime)
}

// StorageKey 对象 key：images/<sha[0:2]>/<sha[2:4]>/<sha><ext>
//
//	@param sha string 64 位 hex
//	@param mime string
//	@return string
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func StorageKey(sha, mime string) string {
	ext, ok := constant.BlobMimeExtensions[mime]
	if !ok {
		ext = constant.BlobDefaultExtension
	}
	return fmt.Sprintf(constant.BlobStorageKeyTemplate, sha[:2], sha[2:4], sha, ext)
}

func detectMime(raw []byte) string {
	mime := http.DetectContentType(raw)
	if _, ok := constant.BlobMimeExtensions[mime]; ok {
		return mime
	}
	return constant.MIMETypeOctetStream
}

// verbatim 无法重新编码时的兜底：≤2MB 原样存储，否则标记 Unprocessable
func verbatim(raw []byte, mime string) CompressResult {
	return CompressResult{Data: raw, Mime: mime, Unprocessable: len(raw) > constant.BlobMaxStoredBytes}
}

// reencode 解码（按 EXIF 摆正）→ 缩放 → PNG/JPEG 逐档尝试，直到不超过存储上限
func reencode(raw []byte, mime string) (res CompressResult, ok bool) {
	decodeMu.Lock()
	defer decodeMu.Unlock()
	defer func() {
		if recover() != nil {
			res, ok = CompressResult{}, false
		}
	}()

	img, err := imaging.Decode(bytes.NewReader(raw), imaging.AutoOrientation(true))
	if err != nil {
		return CompressResult{}, false
	}
	edge := min(max(img.Bounds().Dx(), img.Bounds().Dy()), constant.BlobMaxLongEdge)
	for edge > 0 {
		scaled := imaging.Fit(img, edge, edge, imaging.Lanczos)
		if data, fits := encodeWithinLimit(scaled, mime); fits {
			b := scaled.Bounds()
			return CompressResult{Data: data, Mime: detectMime(data), Width: b.Dx(), Height: b.Dy()}, true
		}
		edge = int(float64(edge) * constant.BlobDownscaleFactor)
	}
	return CompressResult{}, false
}

// encodeWithinLimit PNG 源先试 PNG，再铺白底按质量档位试 JPEG
func encodeWithinLimit(img *image.NRGBA, mime string) ([]byte, bool) {
	var buf bytes.Buffer
	if mime == constant.MIMETypeImagePNG {
		if err := png.Encode(&buf, img); err == nil && buf.Len() <= constant.BlobMaxStoredBytes {
			return buf.Bytes(), true
		}
	}
	b := img.Bounds()
	flat := imaging.OverlayCenter(imaging.New(b.Dx(), b.Dy(), color.White), img, 1)
	for _, quality := range constant.BlobJPEGQualities {
		buf.Reset()
		if err := jpeg.Encode(&buf, flat, &jpeg.Options{Quality: quality}); err == nil && buf.Len() <= constant.BlobMaxStoredBytes {
			return buf.Bytes(), true
		}
	}
	return nil, false
}
