package constant

import "time"

const (
	CosListObjectsMaxKeys             = 1000
	ObjectStorageDirTemplate          = "user-%d/%s"
	ObjectStorageNotConfiguredMessage = "no object storage configured"
	ObjectStorageUnsupportedMessage   = "unsupported storage type"
	COSBucketURLTemplate              = "https://%s-%s.cos.%s.myqcloud.com"
	PresignObjectExpire               = 5 * time.Minute
)
