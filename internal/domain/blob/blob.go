// Package blob 消息图片外置存储的领域端口
package blob

import (
	"context"
	"time"
)

// ObjectStore S3 兼容对象存储
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type ObjectStore interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// ImageView 一张已外置图片的读取视图
//
//	已上传时给出预签名 URL；其余状态给出 blobs 表中暂存的字节。
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type ImageView struct {
	Mime          string
	Size          int64
	Width, Height int
	URL           string
	ExpiresAt     *time.Time
	Data          []byte
}

// ImageReader 已外置图片的读取端口
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type ImageReader interface {
	// IsReferenced 判断给定 message 中是否引用了该图片
	IsReferenced(ctx context.Context, messageIDs []uint, sha string) (bool, error)
	// Locate 查找图片的读取方式；不存在返回 nil, nil
	Locate(ctx context.Context, sha string) (*ImageView, error)
	// ReadBytes 读取图片字节（已上传的从对象存储取回）
	ReadBytes(ctx context.Context, sha string) (mime string, data []byte, err error)
}
