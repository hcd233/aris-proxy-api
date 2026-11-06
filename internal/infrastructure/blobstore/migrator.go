package blobstore

import (
	"cmp"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/logger"
)

// MigrateOptions 存量迁移参数
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type MigrateOptions struct {
	BatchSize int
	// StartID 从 id > StartID 的行开始（断点续跑）
	StartID uint
	// Limit 最多处理的行数，0 表示不限
	Limit  int
	DryRun bool
}

// MigrateStats 存量迁移统计
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type MigrateStats struct {
	Scanned, Migrated, ParseFailed, ChecksumKept int64
	// LastID 最后处理的 message ID，可作为下次 --start-id
	LastID uint
	Blobs  ExternalizeStats
}

// Migrator 把 messages 表中存量 base64 图片外置为引用
//
//	@author centonhuang
//	@update 2026-11-05 10:00:00
type Migrator struct {
	db           *gorm.DB
	externalizer *Externalizer
}

// messageRow 按原始字符串读取 message 行，绕开 GORM 字段 serializer
type messageRow struct {
	ID       uint
	ModelID  string
	Message  string
	CheckSum string
}

// parseImageRefs 解析 message 行中 image_url 部分引用的图片；解析失败记日志并返回 nil
func parseImageRefs(ctx context.Context, row messageRow) []string {
	var msg vo.UnifiedMessage
	if err := sonic.UnmarshalString(row.Message, &msg); err != nil {
		logger.WithCtx(ctx).Warn("[BlobStore] Failed to parse message", zap.Uint("id", row.ID), zap.Error(err))
		return nil
	}
	return vo.ImageRefSHAs(&msg)
}

// NewMigrator 构造存量迁移器
//
//	@param db *gorm.DB
//	@param externalizer *Externalizer
//	@return *Migrator
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func NewMigrator(db *gorm.DB, externalizer *Externalizer) *Migrator {
	return &Migrator{db: db, externalizer: externalizer}
}

// Run 按 id 升序分批迁移含 data URI 图片的 message（含已软删的行）
//
//	@receiver m *Migrator
//	@param ctx context.Context
//	@param opts MigrateOptions
//	@return MigrateStats
//	@return error
//	@author centonhuang
//	@update 2026-11-05 10:00:00
func (m *Migrator) Run(ctx context.Context, opts MigrateOptions) (MigrateStats, error) {
	log := logger.WithCtx(ctx)
	batch := cmp.Or(max(opts.BatchSize, 0), constant.BlobMigrateDefaultBatchSize)
	stats := MigrateStats{LastID: opts.StartID}
	db := m.db.WithContext(ctx)
	for {
		var ids []uint
		if err := db.Table(constant.TableMessages).
			Where(constant.DBConditionBlobMigrateCandidate, stats.LastID, constant.BlobMigrateLikePattern).
			Order(constant.FieldID).Limit(batch).
			Pluck(constant.FieldID, &ids).Error; err != nil {
			return stats, ierr.Wrap(ierr.ErrDBQuery, err, "list migrate candidates")
		}
		for _, id := range ids {
			if opts.Limit > 0 && stats.Scanned >= int64(opts.Limit) {
				return stats, nil
			}
			if err := ctx.Err(); err != nil {
				return stats, err
			}
			m.migrateRow(ctx, db, id, opts.DryRun, &stats)
			stats.Scanned++
			stats.LastID = id
		}
		log.Info("[BlobMigrate] Batch done",
			zap.Uint("lastID", stats.LastID), zap.Int64("scanned", stats.Scanned), zap.Int64("migrated", stats.Migrated),
			zap.Int("uploaded", stats.Blobs.Uploaded), zap.Int("pending", stats.Blobs.Pending),
			zap.Int("unprocessable", stats.Blobs.Unprocessable), zap.Int("reused", stats.Blobs.Reused),
			zap.Int("failed", stats.Blobs.Failed), zap.Int64("savedBytes", stats.Blobs.SavedBytes))
		if len(ids) < batch {
			return stats, nil
		}
	}
}

func (m *Migrator) migrateRow(ctx context.Context, db *gorm.DB, id uint, dryRun bool, stats *MigrateStats) {
	log := logger.WithCtx(ctx)
	var row messageRow
	if err := db.Table(constant.TableMessages).Select(constant.MessageRawFields).
		Where(constant.DBConditionWhereIDIn, []uint{id}).Take(&row).Error; err != nil {
		log.Error("[BlobMigrate] Failed to load message", zap.Uint("id", id), zap.Error(err))
		return
	}
	var msg vo.UnifiedMessage
	if err := sonic.UnmarshalString(row.Message, &msg); err != nil {
		log.Error("[BlobMigrate] Failed to parse message, skipped", zap.Uint("id", id), zap.Error(err))
		stats.ParseFailed++
		return
	}
	if dryRun {
		stats.Blobs.SavedBytes += dataURIBytes(&msg)
		stats.Migrated++
		return
	}

	out, blobStats := m.externalizer.Externalize(ctx, []*vo.UnifiedMessage{&msg})
	stats.Blobs.add(blobStats)
	if out[0] == &msg {
		return
	}
	payload, err := sonic.MarshalString(out[0])
	if err != nil {
		log.Error("[BlobMigrate] Failed to marshal message", zap.Uint("id", id), zap.Error(err))
		return
	}
	updates := map[string]any{constant.FieldMessage: payload, constant.FieldUpdatedAt: time.Now().UTC()}
	if len(out[0].ToolCalls) == 0 {
		updates[constant.FieldCheckSum] = vo.ComputeMessageChecksum(out[0], row.ModelID, nil)
	}
	err = db.Table(constant.TableMessages).Where(constant.DBConditionWhereIDIn, []uint{id}).Updates(updates).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		delete(updates, constant.FieldCheckSum)
		stats.ChecksumKept++
		err = db.Table(constant.TableMessages).Where(constant.DBConditionWhereIDIn, []uint{id}).Updates(updates).Error
	}
	if err != nil {
		log.Error("[BlobMigrate] Failed to update message", zap.Uint("id", id), zap.Error(err))
		return
	}
	stats.Migrated++
}

// dataURIBytes 预估可从 message 中移走的 data URI 字节数（dry-run 用）
func dataURIBytes(msg *vo.UnifiedMessage) int64 {
	if msg.Content == nil {
		return 0
	}
	var n int64
	for _, p := range msg.Content.Parts {
		if p != nil && p.Type == enum.ContentPartTypeImageURL && strings.HasPrefix(p.ImageURL, constant.DataURLPrefix) {
			n += int64(len(p.ImageURL))
		}
	}
	return n
}
