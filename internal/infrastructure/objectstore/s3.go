// Package objectstore 消息图片使用的 S3 兼容对象存储客户端（R2 / COS / AWS S3 / MinIO）
package objectstore

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/config"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/logger"
)

type s3Store struct {
	client *minio.Client
	bucket string
}

// Default 进程内共享的对象存储客户端（fx 装配、cron、CLI 共用同一实例）
//
//	首次调用时按配置构造；未配置或配置无效时恒为 nil，调用方按"未配置"处理。
//
//	@author centonhuang
//	@update 2026-11-06 10:00:00
var Default = sync.OnceValue(NewFromConfig)

// NewFromConfig 按配置构造对象存储（每次调用新建客户端，业务代码应使用 Default）
//
//	未配置 endpoint 或配置无效时返回 nil（不 panic，不发起网络请求），调用方按"未配置"处理。
//
//	@return blob.ObjectStore
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func NewFromConfig() blob.ObjectStore {
	if config.ObjectStorageEndpoint == "" {
		return nil
	}
	host, secure, err := parseEndpoint(config.ObjectStorageEndpoint, config.ObjectStorageTLS)
	if err != nil {
		logger.Logger().Error("[ObjectStore] Invalid object storage endpoint", zap.Error(err))
		return nil
	}
	lookup := minio.BucketLookupAuto
	if config.ObjectStoragePathStyle {
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(host, &minio.Options{
		Creds:        credentials.NewStaticV4(config.ObjectStorageAccessKey, config.ObjectStorageSecretKey, ""),
		Secure:       secure,
		Region:       config.ObjectStorageRegion,
		BucketLookup: lookup,
	})
	if err != nil {
		logger.Logger().Error("[ObjectStore] Failed to create object storage client", zap.Error(err))
		return nil
	}
	return &s3Store{client: client, bucket: config.ObjectStorageBucket}
}

// parseEndpoint 兼容带协议前缀的 endpoint：前缀决定 TLS，否则取配置
func parseEndpoint(endpoint string, tls bool) (host string, secure bool, err error) {
	if !strings.Contains(endpoint, constant.URLSchemeSeparator) {
		return endpoint, tls, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", false, ierr.Wrap(ierr.ErrValidation, err, "parse object storage endpoint")
	}
	if u.Host == "" {
		return "", false, ierr.New(ierr.ErrValidation, "object storage endpoint missing host")
	}
	return u.Host, u.Scheme == constant.HTTPSchemeHTTPS, nil
}

func (s *s3Store) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType:      contentType,
		DisableMultipart: true,
		// http endpoint 下 minio 默认走 aws-chunked 流式签名，部分 S3 兼容实现不支持；统一用 UNSIGNED-PAYLOAD（与 https 默认一致）
		DisableContentSha256: true,
	})
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "put object")
	}
	return nil
}

func (s *s3Store) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "get object")
	}
	defer func() { _ = obj.Close() }() //nolint:errcheck // 只读关闭
	// 已上传对象都是 ≤ BlobMaxStoredBytes 的存储版本，多读 1 字节用于识别异常大对象
	data, err := io.ReadAll(io.LimitReader(obj, constant.BlobMaxStoredBytes+1))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "read object")
	}
	if len(data) > constant.BlobMaxStoredBytes {
		return nil, ierr.Newf(ierr.ErrInternal, "object %s exceeds %d bytes", key, constant.BlobMaxStoredBytes)
	}
	return data, nil
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "remove object")
	}
	return nil
}

func (s *s3Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	signed, err := s.client.PresignedGetObject(ctx, s.bucket, key, ttl, url.Values{})
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "presign object")
	}
	return signed.String(), nil
}
