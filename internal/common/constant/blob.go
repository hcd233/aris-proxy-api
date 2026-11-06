package constant

import (
	"time"

	"github.com/hcd233/aris-mass/internal/common/enum"
)

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
	// BlobDefaultExtension 非白名单 MIME 的对象扩展名
	BlobDefaultExtension = ".bin"

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
	// BlobPresignTTL 预签名 GET 链接有效期
	BlobPresignTTL = 10 * time.Minute
	// BlobPoolMaxInflightBytes 上传池内存中排队与处理中的原图字节总量上限，超出时直接落 blobs 表
	BlobPoolMaxInflightBytes = 64 << 20

	// BlobRetryBatchSize 重试任务每批处理行数
	BlobRetryBatchSize = 50
	// BlobRetryMaxAttempts 上传尝试次数上限，达到后转 failed
	BlobRetryMaxAttempts = 10
	// BlobLastErrorMaxLen last_error 列截断长度（按字符计）
	BlobLastErrorMaxLen = 512

	// BlobExportCacheMaxBytes 一次导出内跨会话的图片缓存总量上限
	BlobExportCacheMaxBytes = 16 << 20
	// BlobExportSessionMaxBytes 单个会话内联图片（data URI）的总字节上限，超出的图片被跳过
	BlobExportSessionMaxBytes = 32 << 20
	// BlobMigrateDefaultBatchSize 存量迁移默认批大小
	BlobMigrateDefaultBatchSize = 200
	// BlobMigrateLikePattern 存量迁移预过滤：sonic 序列化后含 data URI 图片的 message
	BlobMigrateLikePattern = `%"image_url":"data:%`
	// BlobAnyRefLikePattern 预过滤：含任意图片引用的 message
	BlobAnyRefLikePattern = `%"image_url":"aris-blob:sha256:%`
	// BlobRefLikePatternTemplate 预过滤：含指定图片引用的 message（参数为完整引用）
	BlobRefLikePatternTemplate = `%%"image_url":"%s"%%`
	// BlobRefCheckMaxRows 引用校验时最多解析的候选 message 行数
	BlobRefCheckMaxRows = 50
)

var (
	// BlobMetaFields 不含 data 的元数据列
	BlobMetaFields = []string{FieldID, FieldSHA256, FieldStatus, FieldStorageKey, FieldMime, FieldSize, FieldWidth, FieldHeight}
	// BlobRetryableStatuses 允许被改写的状态
	BlobRetryableStatuses = []string{enum.BlobStatusPending, enum.BlobStatusFailed}
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
