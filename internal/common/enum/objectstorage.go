package enum

// ObjectStoragePlatform 存储提供商
type ObjectStoragePlatform string

const (
	// ObjectStoragePlatformMinio Minio存储
	ObjectStoragePlatformMinio ObjectStoragePlatform = "minio"
	// ObjectStoragePlatformCOS 腾讯云COS存储
	ObjectStoragePlatformCOS ObjectStoragePlatform = "cos"
)
