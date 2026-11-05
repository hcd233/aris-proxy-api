package blobstore

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/internal/logger"
)

// RetryStats 一次重试的统计：Uploaded 为本次成功数，Pending/Failed 为执行后的存量
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type RetryStats struct{ Uploaded, Pending, Failed int64 }

// RetryUploads 补传一批到期的 pending 图片
//
//	先取 ID 再逐行读 data，避免一次把整批大字段读进内存；单行出错只影响该行。
//
//	@param ctx context.Context
//	@param db *gorm.DB
//	@param store blob.ObjectStore 为 nil 时直接返回
//	@param now time.Time
//	@return RetryStats
//	@return error
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func RetryUploads(ctx context.Context, db *gorm.DB, store blob.ObjectStore, now time.Time) (RetryStats, error) {
	var stats RetryStats
	if store == nil {
		return stats, nil
	}
	db = db.WithContext(ctx)
	blobDAO := dao.GetBlobDAO()
	ids, err := blobDAO.ListDueRetryIDs(db, now, constant.BlobRetryBatchSize)
	if err != nil {
		return stats, ierr.Wrap(ierr.ErrDBQuery, err, "list due blob retries")
	}
	for _, id := range ids {
		if retryOne(ctx, db, store, id, now) {
			stats.Uploaded++
		}
	}
	if stats.Pending, err = blobDAO.CountByStatus(db, enum.BlobStatusPending); err != nil {
		return stats, ierr.Wrap(ierr.ErrDBQuery, err, "count pending blobs")
	}
	if stats.Failed, err = blobDAO.CountByStatus(db, enum.BlobStatusFailed); err != nil {
		return stats, ierr.Wrap(ierr.ErrDBQuery, err, "count failed blobs")
	}
	return stats, nil
}

// retryOne 补传一行；返回是否上传成功
func retryOne(ctx context.Context, db *gorm.DB, store blob.ObjectStore, id uint, now time.Time) bool {
	log := logger.WithCtx(ctx)
	blobDAO := dao.GetBlobDAO()
	row, err := blobDAO.GetByID(db, id)
	if err != nil || row == nil {
		log.Error("[BlobRetry] Failed to load blob", zap.Uint("id", id), zap.Error(err))
		return false
	}
	if putErr := putRow(ctx, store, row); putErr != nil {
		recordFailure(ctx, db, row, putErr, now)
		return false
	}
	if err := blobDAO.MarkUploaded(db, id); err != nil {
		log.Error("[BlobRetry] Failed to mark blob uploaded", zap.Uint("id", id), zap.Error(err))
		return false
	}
	return true
}

func putRow(ctx context.Context, store blob.ObjectStore, row *dbmodel.Blob) error {
	if len(row.Data) == 0 {
		return ierr.New(ierr.ErrInternal, "blob data missing")
	}
	putCtx, cancel := context.WithTimeout(ctx, constant.BlobPutTimeout)
	defer cancel()
	return store.Put(putCtx, row.StorageKey, row.Mime, row.Data)
}

// recordFailure 按退避推后重试；达到上限转 failed
func recordFailure(ctx context.Context, db *gorm.DB, row *dbmodel.Blob, cause error, now time.Time) {
	log := logger.WithCtx(ctx)
	attempts := row.Attempts + 1
	lastError := TruncateError(cause)
	if attempts >= constant.BlobRetryMaxAttempts {
		log.Error("[BlobRetry] Blob upload exhausted retries", zap.Uint("id", row.ID), zap.Int("attempts", attempts), zap.Error(cause))
		if err := dao.GetBlobDAO().MarkFailed(db, row.ID, attempts, lastError); err != nil {
			log.Error("[BlobRetry] Failed to mark blob failed", zap.Uint("id", row.ID), zap.Error(err))
		}
		return
	}
	backoff := constant.BlobRetryBackoff[min(attempts-1, len(constant.BlobRetryBackoff)-1)]
	log.Warn("[BlobRetry] Blob upload failed, will retry", zap.Uint("id", row.ID), zap.Int("attempts", attempts), zap.Error(cause))
	if err := dao.GetBlobDAO().MarkRetry(db, row.ID, attempts, lastError, now.Add(backoff)); err != nil {
		log.Error("[BlobRetry] Failed to record blob retry", zap.Uint("id", row.ID), zap.Error(err))
	}
}
