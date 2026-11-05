// Package cron 消息图片补传定时任务
//
// 把 blobs 表中到期的 pending 图片上传到对象存储，失败按退避重试，超过上限转 failed。
// 对象存储未配置时每次执行直接跳过。
//
//	author centonhuang
//	update 2026-11-05 10:00:00
package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/constant"
	commonmodel "github.com/hcd233/aris-mass/internal/common/model"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	"github.com/hcd233/aris-mass/internal/infrastructure/objectstore"
	"github.com/hcd233/aris-mass/internal/lock"
	"github.com/hcd233/aris-mass/internal/logger"
)

// BlobUploadRetryCron 消息图片补传定时任务
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type BlobUploadRetryCron struct {
	cron    *cron.Cron
	db      *gorm.DB
	store   blob.ObjectStore
	locker  *lock.RedisLocker
	lockKey string
}

// NewBlobUploadRetryCron 创建消息图片补传定时任务
//
//	@param db *gorm.DB
//	@param cache *redis.Client
//	@return Cron
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func NewBlobUploadRetryCron(db *gorm.DB, cache *redis.Client) Cron {
	return &BlobUploadRetryCron{
		cron:   cron.New(cron.WithLogger(newCronLoggerAdapter(constant.CronModuleBlobUploadRetry))),
		db:     db,
		store:  objectstore.NewFromConfig(),
		locker: lock.NewLocker(cache),
	}
}

// Stop 停止调度并等待运行中任务完成
func (c *BlobUploadRetryCron) Stop() {
	if c.cron != nil {
		<-c.cron.Stop().Done()
	}
}

// StopGracefully 仅停止调度，不等待运行中任务完成
func (c *BlobUploadRetryCron) StopGracefully() {
	if c.cron != nil {
		c.cron.Stop()
	}
}

// Start 启动定时任务
func (c *BlobUploadRetryCron) Start(spec string) error {
	c.lockKey = fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleBlobUploadRetry)
	entryID, err := c.cron.AddFunc(spec, wrapCronFunc(constant.CronModuleBlobUploadRetry, c.locker, c.lockKey, LockOptions{}, c.retry, constant.CronTriggerSourceScheduled))
	if err != nil {
		logger.Logger().Error("[BlobUploadRetryCron] Add func error", zap.Error(err))
		return err
	}
	logger.Logger().Info("[BlobUploadRetryCron] Add func success", zap.Int("entryID", int(entryID)))
	c.cron.Start()
	return nil
}

// Trigger 手动触发一次
func (c *BlobUploadRetryCron) Trigger() bool {
	return TriggerWithLock(constant.CronModuleBlobUploadRetry, c.locker, c.lockKey, LockOptions{}, c.retry)
}

func (c *BlobUploadRetryCron) retry(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
	stats, err := blobstore.RetryUploads(ctx, c.db, c.store, time.Now().UTC())
	if err != nil {
		logger.WithCtx(ctx).Error("[BlobUploadRetryCron] Retry uploads failed", zap.Error(err))
		return nil, err
	}
	return &commonmodel.CronCallAuditMetadata{
		UploadedBlobs: stats.Uploaded,
		PendingBlobs:  stats.Pending,
		FailedBlobs:   stats.Failed,
	}, nil
}
