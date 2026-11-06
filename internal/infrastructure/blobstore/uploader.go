package blobstore

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alitto/pond/v2"
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

// Uploader 新图片的压缩、上传与 blobs 行写入
//
//	异步模式（服务进程）：图片在独立协程池的内存队列里排队，压缩并上传成功后写 uploaded 行，
//	上传失败才把压缩后的字节写成 pending 行；队列满、内存预算超限或进程退出排空时，
//	直接把原图写成 pending 行，由重试任务压缩补传。消息落库因此不再等待压缩与上传。
//	同步模式（CLI 迁移）：在调用方协程里处理完再返回，保证 message 替换为引用前 blobs 行已存在。
//
//	ponytail: 进程被强杀（OOM / SIGKILL）时内存中尚未处理的图片会丢失，引用变成悬空；
//	要做到零丢失需改回"先落 pending 行再异步上传"。
//
//	@author centonhuang
//	@update 2026-11-06 10:00:00
type Uploader struct {
	db    *gorm.DB
	store blob.ObjectStore
	pool  pond.Pool
	// inflight 正在排队或处理中的 sha256，同一张图并发提交时只处理一次
	inflight      sync.Map
	inflightBytes atomic.Int64
	draining      atomic.Bool
}

// NewAsyncUploader 构造带独立协程池的上传器（服务进程使用）
//
//	@param db *gorm.DB
//	@param store blob.ObjectStore 为 nil 时一律写 pending，由重试任务在配置后补传
//	@param workers int
//	@param queueSize int
//	@return *Uploader
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func NewAsyncUploader(db *gorm.DB, store blob.ObjectStore, workers, queueSize int) *Uploader {
	return &Uploader{db: db, store: store, pool: pond.NewPool(workers, pond.WithQueueSize(queueSize))}
}

// NewSyncUploader 构造同步上传器（CLI 迁移使用）
//
//	@param db *gorm.DB
//	@param store blob.ObjectStore
//	@return *Uploader
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func NewSyncUploader(db *gorm.DB, store blob.ObjectStore) *Uploader {
	return &Uploader{db: db, store: store}
}

// Submit 提交一张 blobs 表中尚不存在的新图
//
//	返回 error 表示这张图既没有入队也没有写入 blobs 行，调用方应保留 data URI。
//
//	@receiver u *Uploader
//	@param ctx context.Context 已脱离请求生命周期的上下文
//	@param sha string 原始字节 sha256
//	@param raw []byte 原始字节
//	@param stats *ExternalizeStats
//	@return error
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func (u *Uploader) Submit(ctx context.Context, sha string, raw []byte, stats *ExternalizeStats) error {
	if _, dup := u.inflight.LoadOrStore(sha, struct{}{}); dup {
		stats.Queued++
		return nil
	}
	if u.pool != nil && u.tryEnqueue(ctx, sha, raw) {
		stats.Queued++
		return nil
	}
	defer u.inflight.Delete(sha)
	// 同步模式就地处理；异步模式入队失败时直接落原图 pending 行
	status := enum.BlobStatusPending
	var err error
	if u.pool == nil {
		status, err = u.process(ctx, sha, raw)
	} else {
		err = u.persistRaw(ctx, sha, raw)
	}
	if err == nil {
		stats.count(status)
	}
	return err
}

// Stop 停止接收新图，并把队列中剩余的图片直接写成 pending 行后返回
//
//	@receiver u *Uploader
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func (u *Uploader) Stop() {
	u.draining.Store(true)
	if u.pool != nil {
		u.pool.StopAndWait()
	}
}

// tryEnqueue 内存预算内且队列未满时入队
func (u *Uploader) tryEnqueue(ctx context.Context, sha string, raw []byte) bool {
	if u.draining.Load() {
		return false
	}
	size := int64(len(raw))
	if u.inflightBytes.Add(size) > constant.BlobPoolMaxInflightBytes {
		u.inflightBytes.Add(-size)
		return false
	}
	_, ok := u.pool.TrySubmit(func() {
		defer u.inflight.Delete(sha)
		defer u.inflightBytes.Add(-size)
		u.run(ctx, sha, raw)
	})
	if !ok {
		u.inflightBytes.Add(-size)
	}
	return ok
}

// run 协程池中处理一张图；排空阶段跳过压缩与上传，直接落 pending
func (u *Uploader) run(ctx context.Context, sha string, raw []byte) {
	var err error
	if u.draining.Load() {
		err = u.persistRaw(ctx, sha, raw)
	} else {
		_, err = u.process(ctx, sha, raw)
	}
	if err != nil {
		logger.WithCtx(ctx).Error("[BlobUploader] Image lost: failed to write blob row", zap.String("sha256", sha), zap.Error(err))
	}
}

// process 压缩、尽力上传一次并写入 blobs 行
func (u *Uploader) process(ctx context.Context, sha string, raw []byte) (enum.BlobStatus, error) {
	res := Compress(raw)
	row := &dbmodel.Blob{
		SHA256:       sha,
		StorageKey:   StorageKey(sha, res.Mime),
		Mime:         res.Mime,
		Size:         int64(len(res.Data)),
		Width:        res.Width,
		Height:       res.Height,
		OriginalSize: int64(len(raw)),
	}
	switch {
	case res.Unprocessable:
		row.Status, row.Data = enum.BlobStatusUnprocessable, res.Data
	case u.tryPut(ctx, res, row):
		row.Status = enum.BlobStatusUploaded
	default:
		next := time.Now().UTC().Add(constant.BlobRetryBackoff[0])
		row.Status, row.Data, row.Attempts, row.NextRetryAt = enum.BlobStatusPending, res.Data, 1, &next
	}
	if err := dao.GetBlobDAO().InsertIgnoreConflict(u.db.WithContext(ctx), row); err != nil {
		return "", ierr.Wrap(ierr.ErrDBCreate, err, "insert blob")
	}
	return row.Status, nil
}

// persistRaw 不压缩不上传，把原图写成立即可重试的 pending 行
func (u *Uploader) persistRaw(ctx context.Context, sha string, raw []byte) error {
	mime := detectMime(raw)
	now := time.Now().UTC()
	row := &dbmodel.Blob{
		SHA256:       sha,
		Status:       enum.BlobStatusPending,
		Data:         raw,
		StorageKey:   StorageKey(sha, mime),
		Mime:         mime,
		Size:         int64(len(raw)),
		OriginalSize: int64(len(raw)),
		NextRetryAt:  &now,
	}
	if err := dao.GetBlobDAO().InsertIgnoreConflict(u.db.WithContext(ctx), row); err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "insert pending blob")
	}
	return nil
}

// tryPut 对象存储已配置时上传一次；失败原因记入 row.LastError
func (u *Uploader) tryPut(ctx context.Context, res CompressResult, row *dbmodel.Blob) bool {
	if u.store == nil {
		return false
	}
	putCtx, cancel := context.WithTimeout(ctx, constant.BlobPutTimeout)
	defer cancel()
	if err := u.store.Put(putCtx, row.StorageKey, res.Mime, res.Data); err != nil {
		logger.WithCtx(ctx).Warn("[BlobUploader] Upload failed, kept pending", zap.String("key", row.StorageKey), zap.Error(err))
		row.LastError = TruncateError(err)
		return false
	}
	return true
}
