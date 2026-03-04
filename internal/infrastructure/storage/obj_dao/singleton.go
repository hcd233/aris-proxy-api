package objdao

import (
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/storage"
)

// createObjectStorageDAO 创建对象存储DAO
func createObjectStorageDAO(objectType enum.ObjectType) ObjDAO {
	switch storage.GetPlatform() {
	case enum.ObjectStoragePlatformMinio:
		return &MinioObjDAO{
			ObjectType: objectType,
			BucketName: config.MinioBucketName,
			client:     storage.GetMinioStorage(),
		}
	case enum.ObjectStoragePlatformCOS:
		return &CosObjDAO{
			ObjectType: objectType,
			BucketName: config.CosBucketName,
			client:     storage.GetCosClient(),
		}
	default:
		panic("unsupported storage type")
	}
}

// GetAudioObjDAO 获取音频对象DAO单例
//
//	return ObjDAO
//	author centonhuang
//	update 2024-10-18 01:10:28
func GetAudioObjDAO() ObjDAO {
	return createObjectStorageDAO(enum.ObjectTypeAudio)
}
