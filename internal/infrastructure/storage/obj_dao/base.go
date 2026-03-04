// Package objdao 对象存储DAO
package objdao

import (
	"context"
	"io"
	"net/url"
	"time"
)

// ObjDAO 对象存储DAO接口
//
//	author centonhuang
//	update 2025-01-05 22:45:30
type ObjDAO interface {
	GetBucketName(ctx context.Context) string
	CreateBucket(ctx context.Context) (err error)
	CreateDir(ctx context.Context, userID uint) (objectInfo *ObjectInfo, err error)
	ListObjects(ctx context.Context, userID uint) (objectInfos []ObjectInfo, err error)
	UploadObject(ctx context.Context, userID uint, objectName string, size int64, reader io.Reader) (err error)
	DownloadObject(ctx context.Context, userID uint, objectName string, writer io.Writer) (objectInfo *ObjectInfo, err error)
	PresignObject(ctx context.Context, userID uint, objectName string) (presignedURL *url.URL, err error)
	DeleteObject(ctx context.Context, userID uint, objectName string) (err error)
}

// ObjectInfo 对象信息
//
//	author centonhuang
//	update 2025-01-05 22:45:48
type ObjectInfo struct {
	ObjectName   string    `json:"objectName"`
	ContentType  string    `json:"contentType"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
	Expires      time.Time `json:"expires"`
	ETag         string    `json:"etag"`
}
