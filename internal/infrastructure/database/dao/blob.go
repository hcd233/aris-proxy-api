package dao

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
)

// BlobDAO 消息图片 blobs 表数据访问对象
//
//	blobs 不软删，不复用 baseDAO（其查询恒带 deleted_at = 0）。
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type BlobDAO struct{}

var blobDAOSingleton = &BlobDAO{}

// GetBlobDAO 获取 Blob DAO
//
//	@return *BlobDAO
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func GetBlobDAO() *BlobDAO {
	return blobDAOSingleton
}

// FindMetaBySHA256s 批量查询元数据（不读 data）
func (dao *BlobDAO) FindMetaBySHA256s(db *gorm.DB, shas []string) ([]*dbmodel.Blob, error) {
	var rows []*dbmodel.Blob
	if len(shas) == 0 {
		return rows, nil
	}
	err := db.Select(constant.BlobMetaFields).
		Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldSHA256), shas).
		Find(&rows).Error
	return rows, err
}

// InsertIgnoreConflict 插入；sha256 已存在时什么也不做（保留先到者）
func (dao *BlobDAO) InsertIgnoreConflict(db *gorm.DB, blob *dbmodel.Blob) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: constant.FieldSHA256}},
		DoNothing: true,
	}).Create(blob).Error
}

// GetBySHA256 按 sha256 查询完整行；不存在返回 nil, nil
func (dao *BlobDAO) GetBySHA256(db *gorm.DB, sha string) (*dbmodel.Blob, error) {
	return firstOrNil(db.Where(&dbmodel.Blob{SHA256: sha}))
}

// GetByID 按 ID 查询完整行；不存在返回 nil, nil
func (dao *BlobDAO) GetByID(db *gorm.DB, id uint) (*dbmodel.Blob, error) {
	return firstOrNil(db.Where(constant.DBConditionWhereIDIn, []uint{id}))
}

// ListDueRetryIDs 到期待重试的 pending 行 ID，按 next_retry_at 升序
func (dao *BlobDAO) ListDueRetryIDs(db *gorm.DB, now time.Time, limit int) ([]uint, error) {
	var ids []uint
	err := db.Model(&dbmodel.Blob{}).
		Where(constant.DBConditionBlobDue, enum.BlobStatusPending, now).
		Order(constant.DBOrderNextRetryAtAsc).
		Limit(limit).
		Pluck(constant.FieldID, &ids).Error
	return ids, err
}

// MarkUploaded 上传成功：状态改为 uploaded 并清空 data（只作用于 pending/failed）
func (dao *BlobDAO) MarkUploaded(db *gorm.DB, id uint) error {
	return retryableRow(db, id).Updates(map[string]any{
		constant.FieldStatus:    enum.BlobStatusUploaded,
		constant.FieldBlobData:  nil,
		constant.FieldUpdatedAt: time.Now().UTC(),
	}).Error
}

// MarkRetry 上传失败、尚未到上限：记录尝试次数与原因，推后下次重试
func (dao *BlobDAO) MarkRetry(db *gorm.DB, id uint, attempts int, lastError string, next time.Time) error {
	return retryableRow(db, id).Updates(map[string]any{
		constant.FieldStatus:      enum.BlobStatusPending,
		constant.FieldAttempts:    attempts,
		constant.FieldLastError:   lastError,
		constant.FieldNextRetryAt: next,
		constant.FieldUpdatedAt:   time.Now().UTC(),
	}).Error
}

// MarkFailed 超过重试上限：转 failed，data 保留
func (dao *BlobDAO) MarkFailed(db *gorm.DB, id uint, attempts int, lastError string) error {
	return retryableRow(db, id).Updates(map[string]any{
		constant.FieldStatus:    enum.BlobStatusFailed,
		constant.FieldAttempts:  attempts,
		constant.FieldLastError: lastError,
		constant.FieldUpdatedAt: time.Now().UTC(),
	}).Error
}

// ResetFailed 把全部 failed 行重置为立即可重试的 pending
func (dao *BlobDAO) ResetFailed(db *gorm.DB, now time.Time) (int64, error) {
	result := db.Model(&dbmodel.Blob{}).
		Where(constant.DBConditionBlobStatusEquals, enum.BlobStatusFailed).
		Updates(map[string]any{
			constant.FieldStatus:      enum.BlobStatusPending,
			constant.FieldAttempts:    0,
			constant.FieldNextRetryAt: now,
			constant.FieldUpdatedAt:   time.Now().UTC(),
		})
	return result.RowsAffected, result.Error
}

// CountByStatus 统计指定状态的行数
func (dao *BlobDAO) CountByStatus(db *gorm.DB, status enum.BlobStatus) (int64, error) {
	var count int64
	err := db.Model(&dbmodel.Blob{}).Where(constant.DBConditionBlobStatusEquals, status).Count(&count).Error
	return count, err
}

// retryableRow 只命中 pending/failed 行，保证 uploaded 不可被覆盖
func retryableRow(db *gorm.DB, id uint) *gorm.DB {
	return db.Model(&dbmodel.Blob{}).Where(constant.DBConditionBlobRetryable, id, constant.BlobRetryableStatuses)
}

func firstOrNil(query *gorm.DB) (*dbmodel.Blob, error) {
	var row dbmodel.Blob
	err := query.First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
