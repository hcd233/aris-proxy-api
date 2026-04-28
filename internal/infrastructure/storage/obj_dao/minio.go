package objdao

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/minio/minio-go/v7"
	"github.com/samber/lo"
)

// MinioObjDAO 基础Minio对象存储DAO
//
//	author centonhuang
//	update 2025-01-05 22:45:43
type MinioObjDAO struct {
	ObjectType enum.ObjectType
	BucketName string
	client     *minio.Client
}

func (dao *MinioObjDAO) composeDirName(userID uint) string {
	return fmt.Sprintf(constant.ObjectStorageDirTemplate, userID, dao.ObjectType)
}

// GetBucketName 获取桶名
//
//	receiver dao *BaseMinioObjDAO
//	return bucketName string
//	author centonhuang
//	update 2025-01-19 14:13:22
func (dao *MinioObjDAO) GetBucketName(_ context.Context) string {
	return dao.BucketName
}

// CreateBucket 创建桶
//
//	receiver dao *BaseMinioObjDAO
//	param userID uint
//	return exist bool
//	return err error
//	author centonhuang
//	update 2025-01-05 17:37:41
func (dao *MinioObjDAO) CreateBucket(ctx context.Context) (err error) {
	err = dao.client.MakeBucket(ctx, dao.BucketName, minio.MakeBucketOptions{})

	return
}

// CreateDir 创建目录
//
//	receiver dao *BaseMinioObjDAO
//	param userID uint
//	return objectInfo *ObjectInfo
//	return err error
//	author centonhuang
//	update 2025-01-18 17:37:41
func (dao *MinioObjDAO) CreateDir(ctx context.Context, userID uint) (objectInfo *ObjectInfo, err error) {
	dirName := dao.composeDirName(userID)

	// 创建一个空的目录对象
	object, err := dao.client.PutObject(ctx, dao.BucketName, dirName+"/", nil, 0, minio.PutObjectOptions{})
	if err != nil {
		return
	}

	objectInfo = &ObjectInfo{
		ObjectName:   object.Key,
		ContentType:  "",
		Size:         object.Size,
		LastModified: object.LastModified,
		Expires:      time.Time{},
		ETag:         object.ETag,
	}

	return
}

// ListObjects 列出桶中的对象
//
//	receiver dao *BaseMinioObjDAO
//	param userID uint
//	return objectInfos []ObjectInfo
//	return err error
//	author centonhuang
//	update 2025-01-05 17:37:45
func (dao *MinioObjDAO) ListObjects(ctx context.Context, userID uint) (objectInfos []ObjectInfo, err error) {
	dirName := dao.composeDirName(userID)
	dirName += "/"

	objectCh := dao.client.ListObjects(ctx, dao.BucketName, minio.ListObjectsOptions{
		Prefix:     dirName,
		StartAfter: dirName,
	})

	for object := range objectCh {
		if object.Err != nil {
			err = object.Err
			return
		}

		// 跳过目录本身
		if object.Key == dirName {
			continue
		}

		objectInfos = append(objectInfos, ObjectInfo{
			ObjectName:   strings.TrimPrefix(object.Key, dirName),
			ContentType:  object.ContentType,
			Size:         object.Size,
			LastModified: object.LastModified,
			Expires:      object.Expires,
			ETag:         object.ETag,
		})
	}
	return
}

// UploadObject 上传对象
//
//	receiver dao *BaseMinioObjDAO
//	param userID uint
//	param objectName string
//	param size int64
//	param reader io.Reader
//	return err error
//	author centonhuang
//	update 2025-01-05 17:37:50
func (dao *MinioObjDAO) UploadObject(ctx context.Context, userID uint, objectName string, size int64, reader io.Reader) (err error) {
	dirName := dao.composeDirName(userID)
	objectName = path.Join(dirName, objectName)

	_, err = dao.client.PutObject(ctx, dao.BucketName, objectName, reader, size, minio.PutObjectOptions{})
	return
}

// DownloadObject 下载对象
//
//	receiver dao *BaseMinioObjDAO
//	param userID uint
//	param objectName string
//	param writer io.Writer
//	return objectInfo *ObjectInfo
//	return err error
//	author centonhuang
//	update 2025-01-05 17:37:57
func (dao *MinioObjDAO) DownloadObject(ctx context.Context, userID uint, objectName string, writer io.Writer) (objectInfo *ObjectInfo, err error) {
	dirName := dao.composeDirName(userID)
	objectName = path.Join(dirName, objectName)

	object, err := dao.client.GetObject(ctx, dao.BucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return
	}
	defer func() { _ = object.Close() }()

	stat := lo.Must1(object.Stat())

	objectInfo = &ObjectInfo{
		ObjectName:   stat.Key,
		ContentType:  stat.ContentType,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		Expires:      stat.Expires,
		ETag:         stat.ETag,
	}

	_, err = io.Copy(writer, object)

	return
}

// PresignObject 生成对象的预签名URL
//
//	receiver dao *BaseMinioObjDAO
//	param userID uint
//	param objectName string
//	param writer io.Writer
//	return url *url.URL
//	return err error
//	author centonhuang
//	update 2025-01-05 17:38:03
func (dao *MinioObjDAO) PresignObject(ctx context.Context, userID uint, objectName string) (presignedURL *url.URL, err error) {
	dirName := dao.composeDirName(userID)
	objectName = path.Join(dirName, objectName)

	// 设置响应头
	reqParams := make(url.Values)
	reqParams.Set(constant.HTTPContentDispositionParam, fmt.Sprintf(constant.HTTPAttachmentFilenameTemplate, filepath.Base(objectName)))

	// 根据文件扩展名获取 content type
	contentType := constant.MIMETypeOctetStream
	if ext := filepath.Ext(objectName); ext != "" {
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			contentType = mimeType
		}
	}
	reqParams.Set(constant.HTTPContentTypeParam, contentType)

	presignedURL, err = dao.client.PresignedGetObject(ctx, dao.BucketName, objectName, constant.PresignObjectExpire, reqParams)
	return
}

// DeleteObject 删除对象
//
//	receiver dao *BaseMinioObjDAO
//	param userID uint
//	param objectName string
//	return err error
//	author centonhuang
//	update 2025-01-05 17:38:09
func (dao *MinioObjDAO) DeleteObject(ctx context.Context, userID uint, objectName string) (err error) {
	dirName := dao.composeDirName(userID)
	objectName = path.Join(dirName, objectName)

	err = dao.client.RemoveObject(ctx, dao.BucketName, objectName, minio.RemoveObjectOptions{})
	return
}
