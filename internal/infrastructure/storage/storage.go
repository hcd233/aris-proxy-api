package storage

import (
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
)

// InitObjectStorage 初始化对象存储
//
//	author centonhuang
//	update 2024-12-09 15:59:06
func InitObjectStorage() {
	switch GetPlatform() {
	case enum.ObjectStoragePlatformMinio:
		initMinioClient()
	case enum.ObjectStoragePlatformCOS:
		initCosClient()
	}
}

// GetPlatform 获取存储提供商
//
//	return Platform
//	author centonhuang
//	update 2025-01-19 14:13:22
func GetPlatform() enum.ObjectStoragePlatform {
	// 优先使用 COS
	if config.CosAppID != "" {
		return enum.ObjectStoragePlatformCOS
	}

	if config.MinioEndpoint != "" {
		return enum.ObjectStoragePlatformMinio
	}

	panic("no object storage configured")
}
