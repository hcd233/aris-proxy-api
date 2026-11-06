package blobstore

import (
	"context"
	"fmt"
	"slices"
	"time"

	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/dao"
)

type reader struct {
	db    *gorm.DB
	store blob.ObjectStore
}

// NewReader 构造已外置图片的读取端
//
//	@param db *gorm.DB
//	@param store blob.ObjectStore 可为 nil：此时 uploaded 图片无法读取
//	@return blob.ImageReader
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func NewReader(db *gorm.DB, store blob.ObjectStore) blob.ImageReader {
	return &reader{db: db, store: store}
}

// IsReferenced 判断给定 message 中是否有 image_url 部分引用了该图片
//
//	messages.message 是 text 列，按 JSON 字段判断需要逐行 ::jsonb 解析且只有 PG 支持；
//	这里先用 LIKE 按序列化字节把候选收窄到含该引用的行，再在 Go 里反序列化逐部分精确比对，
//	文本里出现的同名字符串不算引用。
func (r *reader) IsReferenced(ctx context.Context, messageIDs []uint, sha string) (bool, error) {
	if len(messageIDs) == 0 || !vo.IsSHA256Hex(sha) {
		return false, nil
	}
	var rows []messageRow
	err := r.db.WithContext(ctx).Table(constant.TableMessages).
		Select(constant.MessageRawFields).
		Where(constant.DBConditionIDInAndMessageLike, messageIDs, fmt.Sprintf(constant.BlobRefLikePatternTemplate, vo.ImageRefOf(sha))).
		Limit(constant.BlobRefCheckMaxRows).Find(&rows).Error
	if err != nil {
		return false, ierr.Wrap(ierr.ErrDBQuery, err, "check image reference")
	}
	return slices.ContainsFunc(rows, func(row messageRow) bool {
		return slices.Contains(parseImageRefs(ctx, row), sha)
	}), nil
}

func (r *reader) Locate(ctx context.Context, sha string) (*blob.ImageView, error) {
	row, err := dao.GetBlobDAO().GetBySHA256(r.db.WithContext(ctx), sha)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get blob")
	}
	if row == nil {
		return nil, nil
	}
	view := &blob.ImageView{Mime: row.Mime, Size: row.Size, Width: row.Width, Height: row.Height}
	// unprocessable 是无法压缩到上限内的原图（> 2MB），不作为预览返回
	if row.Status == enum.BlobStatusUnprocessable {
		return view, nil
	}
	if row.Status != enum.BlobStatusUploaded {
		view.Data = row.Data
		return view, nil
	}
	if r.store == nil {
		return nil, ierr.New(ierr.ErrInternal, "object storage is not configured")
	}
	if view.URL, err = r.store.PresignGet(ctx, row.StorageKey, constant.BlobPresignTTL); err != nil {
		return nil, err
	}
	expires := time.Now().Add(constant.BlobPresignTTL)
	view.ExpiresAt = &expires
	return view, nil
}

func (r *reader) ReadBytes(ctx context.Context, sha string) (mime string, data []byte, err error) {
	row, err := dao.GetBlobDAO().GetBySHA256(r.db.WithContext(ctx), sha)
	if err != nil {
		return "", nil, ierr.Wrap(ierr.ErrDBQuery, err, "get blob")
	}
	if row == nil {
		return "", nil, ierr.New(ierr.ErrDataNotExists, "blob not found")
	}
	if row.Status != enum.BlobStatusUploaded {
		return row.Mime, row.Data, nil
	}
	if r.store == nil {
		return "", nil, ierr.New(ierr.ErrInternal, "object storage is not configured")
	}
	data, err = r.store.Get(ctx, row.StorageKey)
	return row.Mime, data, err
}
