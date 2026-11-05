package enum

// BlobStatus blobs 表图片上传状态
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type BlobStatus = string

const (
	// BlobStatusPending 待上传，图片字节暂存在 blobs.data
	BlobStatusPending BlobStatus = "pending"
	// BlobStatusUploaded 已上传对象存储，blobs.data 已清空
	BlobStatusUploaded BlobStatus = "uploaded"
	// BlobStatusFailed 超过重试上限，blobs.data 保留
	BlobStatusFailed BlobStatus = "failed"
	// BlobStatusUnprocessable 无法解码且超过存储上限，原始字节保留在 blobs.data，不上传
	BlobStatusUnprocessable BlobStatus = "unprocessable"
)
