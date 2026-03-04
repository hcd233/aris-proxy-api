// Package storage Minio对象存储模块
//
//	update 2024-12-09 15:58:58
package storage

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var minioClient *minio.Client

func initMinioClient() {
	minioClient = lo.Must1(minio.New(config.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.MinioAccessID, config.MinioAccessKey, ""),
		Secure: config.MinioTLS,
		Region: config.MinioRegion,
	}))

	_ = lo.Must1(minioClient.ListBuckets(context.Background()))

	logger.Logger().Info("[Object Storage] Connected to Minio", zap.String("endpoint", config.MinioEndpoint))
}

// GetMinioStorage 获取Minio存储客户端
func GetMinioStorage() *minio.Client {
	return minioClient
}
