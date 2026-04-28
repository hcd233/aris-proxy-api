package objdao

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/samber/lo"
	"github.com/tencentyun/cos-go-sdk-v5"
)

// CosObjDAO 腾讯云COS对象存储DAO
//
//	author centonhuang
//	update 2025-01-19 14:13:22
type CosObjDAO struct {
	ObjectType enum.ObjectType
	BucketName string
	client     *cos.Client
}

func (dao *CosObjDAO) composeDirName(userID uint) string {
	return fmt.Sprintf(constant.ObjectStorageDirTemplate, userID, dao.ObjectType)
}

// GetBucketName 获取桶名
//
//	receiver dao *CosObjDAO
//	return bucketName string
//	author centonhuang
//	update 2025-01-19 14:13:22
func (dao *CosObjDAO) GetBucketName(_ context.Context) string {
	return dao.BucketName
}

// CreateBucket 创建桶
func (dao *CosObjDAO) CreateBucket(ctx context.Context) (err error) {
	_, err = dao.client.Bucket.Put(ctx, nil)
	return
}

// CreateDir 创建目录
func (dao *CosObjDAO) CreateDir(ctx context.Context, userID uint) (objectInfo *ObjectInfo, err error) {
	dirName := dao.composeDirName(userID)
	dirName += "/"

	_, err = dao.client.Object.Put(ctx, dirName, strings.NewReader(""), nil)
	if err != nil {
		return
	}

	head, err := dao.client.Object.Head(ctx, dirName, nil)
	if err != nil {
		return
	}

	lastModified, _ := http.ParseTime(head.Header.Get(constant.HTTPHeaderLastModified))

	objectInfo = &ObjectInfo{
		ObjectName:   dirName,
		ContentType:  head.Header.Get(constant.HTTPHeaderContentType),
		Size:         0,
		LastModified: lastModified,
		Expires:      time.Time{},
		ETag:         strings.Trim(head.Header.Get(constant.HTTPHeaderETag), constant.QuoteString),
	}

	return
}

// ListObjects 列出桶中的对象
func (dao *CosObjDAO) ListObjects(ctx context.Context, userID uint) (objectInfos []ObjectInfo, err error) {
	dirName := dao.composeDirName(userID)
	dirName += "/"

	opt := &cos.BucketGetOptions{
		Prefix:    dirName,
		MaxKeys:   constant.CosListObjectsMaxKeys,
		Delimiter: "/",
	}

	result, _, err := dao.client.Bucket.Get(ctx, opt)
	if err != nil {
		return
	}

	for _, object := range result.Contents {
		// 跳过目录本身
		if object.Key == dirName {
			continue
		}

		lastModified := lo.Must1(time.ParseInLocation(time.RFC3339, object.LastModified, time.UTC))

		objectInfos = append(objectInfos, ObjectInfo{
			ObjectName:   strings.TrimPrefix(object.Key, dirName),
			ContentType:  "",
			Size:         object.Size,
			LastModified: lastModified,
			Expires:      time.Time{},
			ETag:         strings.Trim(object.ETag, "\""),
		})
	}

	return
}

// UploadObject 上传对象
func (dao *CosObjDAO) UploadObject(ctx context.Context, userID uint, objectName string, _ int64, reader io.Reader) (err error) {
	dirName := dao.composeDirName(userID)
	objectName = path.Join(dirName, objectName)

	_, err = dao.client.Object.Put(ctx, objectName, reader, nil)
	return
}

// DownloadObject 下载对象
func (dao *CosObjDAO) DownloadObject(ctx context.Context, userID uint, objectName string, writer io.Writer) (objectInfo *ObjectInfo, err error) {
	dirName := dao.composeDirName(userID)
	objectName = path.Join(dirName, objectName)

	resp, err := dao.client.Object.Get(ctx, objectName, nil)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	head, err := dao.client.Object.Head(ctx, objectName, nil)
	if err != nil {
		return
	}

	lastModified, _ := http.ParseTime(head.Header.Get(constant.HTTPHeaderLastModified))

	objectInfo = &ObjectInfo{
		ObjectName:   objectName,
		ContentType:  head.Header.Get(constant.HTTPHeaderContentType),
		Size:         head.ContentLength,
		LastModified: lastModified,
		Expires:      time.Time{},
		ETag:         strings.Trim(head.Header.Get(constant.HTTPHeaderETag), constant.QuoteString),
	}

	_, err = io.Copy(writer, resp.Body)
	return
}

// PresignObject 生成对象的预签名URL
func (dao *CosObjDAO) PresignObject(ctx context.Context, userID uint, objectName string) (presignedURL *url.URL, err error) {
	dirName := dao.composeDirName(userID)
	objectName = path.Join(dirName, objectName)

	presignedURL, err = dao.client.Object.GetPresignedURL(ctx,
		http.MethodGet,
		objectName,
		dao.client.GetCredential().SecretID,
		dao.client.GetCredential().SecretKey,
		constant.PresignObjectExpire,
		nil,
	)
	return
}

// DeleteObject 删除对象
func (dao *CosObjDAO) DeleteObject(ctx context.Context, userID uint, objectName string) (err error) {
	dirName := dao.composeDirName(userID)
	objectName = path.Join(dirName, objectName)

	_, err = dao.client.Object.Delete(ctx, objectName)
	return
}
