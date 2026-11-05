package constant

import "time"

// 消息图片外置（blobs 表 + 对象存储）相关常量
const (
	// BlobRefPrefix message 中图片引用形态的前缀，完整形态为 aris-blob:sha256:<hex>
	BlobRefPrefix = "aris-blob:sha256:"
	// BlobChecksumPrefix checksum 规范化时图片统一映射成的前缀
	BlobChecksumPrefix = "sha256:"
	// BlobStorageKeyTemplate 对象 key 模板：images/<sha[0:2]>/<sha[2:4]>/<sha><ext>
	BlobStorageKeyTemplate = "images/%s/%s/%s%s"
	// BlobSHA256HexLen sha256 hex 长度
	BlobSHA256HexLen = 64

	// BlobMaxStoredBytes 存储版本字节上限（2MB）
	BlobMaxStoredBytes = 2 << 20
	// BlobMaxLongEdge 存储版本长边像素上限
	BlobMaxLongEdge = 4096
	// BlobMaxDecodePixels 允许解码的像素总数上限（防解压炸弹）
	BlobMaxDecodePixels = 50_000_000
	// BlobDownscaleFactor 各档 JPEG 质量仍超限时长边的缩放系数
	BlobDownscaleFactor = 0.75

	// BlobPutTimeout 单张图片 PUT 超时
	BlobPutTimeout = 10 * time.Second
	// BlobTaskTimeout 单个落库任务的图片外置总超时
	BlobTaskTimeout = 60 * time.Second
	// BlobPresignTTL 预签名 GET 链接有效期
	BlobPresignTTL = 10 * time.Minute

	// BlobRetryBatchSize 重试任务每批处理行数
	BlobRetryBatchSize = 50
	// BlobRetryMaxAttempts 上传尝试次数上限，达到后转 failed
	BlobRetryMaxAttempts = 10
	// BlobLastErrorMaxLen last_error 列截断长度
	BlobLastErrorMaxLen = 512

	// BlobExportCacheMaxBytes 一次导出内图片缓存总量上限
	BlobExportCacheMaxBytes = 64 << 20
	// BlobMigrateDefaultBatchSize 存量迁移默认批大小
	BlobMigrateDefaultBatchSize = 200
	// BlobMigrateLikePattern 存量迁移预过滤：sonic 序列化后含 data URI 图片的 message
	BlobMigrateLikePattern = `%"image_url":"data:%`

	// ShareGPTImagePlaceholder ShareGPT 多模态图片占位符（LLaMA-Factory 约定）
	ShareGPTImagePlaceholder = "<image>"

	// MIMETypeImagePNG 等：存储版本允许的图片 MIME（按字节内容判定）
	MIMETypeImagePNG  = "image/png"
	MIMETypeImageJPEG = "image/jpeg"
	MIMETypeImageGIF  = "image/gif"
	MIMETypeImageWebP = "image/webp"
	// BlobDefaultExtension 非白名单 MIME 的对象扩展名
	BlobDefaultExtension = ".bin"
)

var (
	// BlobMimeExtensions 白名单图片 MIME → 对象扩展名
	BlobMimeExtensions = map[string]string{
		MIMETypeImagePNG:  ".png",
		MIMETypeImageJPEG: ".jpg",
		MIMETypeImageGIF:  ".gif",
		MIMETypeImageWebP: ".webp",
	}
	// BlobJPEGQualities 转 JPEG 时依次尝试的质量
	BlobJPEGQualities = []int{90, 80, 70, 60}
	// BlobRetryBackoff 第 n 次失败后的重试间隔（超出末项按末项）
	BlobRetryBackoff = []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour}
)
