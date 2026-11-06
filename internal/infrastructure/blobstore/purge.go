package blobstore

import (
	"context"
	"strings"

	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/config"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-mass/internal/logger"
)

// CollectImageRefs 收集指定 message 中引用的图片 sha256（去重）
//
//	在硬删除 message 之前调用，删除后再用 DeleteUnreferenced 清理不再被引用的图片。
//
//	@param ctx context.Context
//	@param db *gorm.DB
//	@param messageIDs []uint
//	@return []string
//	@return error
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func CollectImageRefs(ctx context.Context, db *gorm.DB, messageIDs []uint) ([]string, error) {
	db = db.WithContext(ctx)
	var shas []string
	for _, chunk := range lo.Chunk(messageIDs, config.SQLBatchSize) {
		var rows []messageRow
		if err := db.Table(constant.TableMessages).Select(constant.MessageRawFields).
			Where(constant.DBConditionIDInAndMessageLike, chunk, constant.BlobAnyRefLikePattern).
			Find(&rows).Error; err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "collect image refs")
		}
		for _, row := range rows {
			shas = append(shas, parseImageRefs(ctx, row)...)
		}
	}
	return lo.Uniq(shas), nil
}

// DeleteUnreferenced 删除 shas 中已不被任何 message（含软删行）引用的 blobs 行与对象
//
//	对象删除失败的行保留，避免留下"行已删、对象还在"无从追踪的孤儿对象；
//	对象存储未配置时 uploaded 行同样保留。
//
//	ponytail: 与在线写入之间没有锁，扫描与删除之间若有新 message 复用了同一张图，这张图会丢失；
//	清理每周一次、窗口为秒级，需要严格时改为删除前对候选行加 FOR UPDATE 并复查。
//
//	@param ctx context.Context
//	@param db *gorm.DB
//	@param store blob.ObjectStore 可为 nil
//	@param shas []string 候选 sha256（通常来自 CollectImageRefs）
//	@return int64 删除的 blobs 行数
//	@return error
//	@author centonhuang
//	@update 2026-11-06 10:00:00
func DeleteUnreferenced(ctx context.Context, db *gorm.DB, store blob.ObjectStore, shas []string) (int64, error) {
	if len(shas) == 0 {
		return 0, nil
	}
	log := logger.WithCtx(ctx)
	db = db.WithContext(ctx)
	referenced, err := referencedSHAs(db, shas)
	if err != nil {
		return 0, err
	}
	candidates := lo.Filter(shas, func(sha string, _ int) bool { return !referenced[sha] })
	rows, err := dao.GetBlobDAO().FindMetaBySHA256s(db, candidates)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrDBQuery, err, "load unreferenced blobs")
	}
	var ids []uint
	for _, row := range rows {
		if row.Status == enum.BlobStatusUploaded {
			if store == nil {
				continue
			}
			if err := store.Delete(ctx, row.StorageKey); err != nil {
				log.Error("[BlobPurge] Failed to delete object, blob row kept", zap.String("key", row.StorageKey), zap.Error(err))
				continue
			}
		}
		ids = append(ids, row.ID)
	}
	n, err := dao.GetBlobDAO().DeleteByIDs(db, ids)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrDBDelete, err, "delete unreferenced blobs")
	}
	return n, nil
}

// referencedSHAs 全表扫描含图片引用的 message，返回 candidates 中仍出现在 message 文本里的 sha256
//
//	这里只做子串匹配不解析 JSON：多匹配只会多保留，解析失败的行也不会导致误删。
func referencedSHAs(db *gorm.DB, candidates []string) (map[string]bool, error) {
	referenced := map[string]bool{}
	var batch []messageRow
	err := db.Table(constant.TableMessages).Select(constant.FieldID, constant.FieldMessage).
		Where(constant.DBConditionMessageLike, constant.BlobAnyRefLikePattern).
		FindInBatches(&batch, config.SQLBatchSize, func(_ *gorm.DB, _ int) error {
			for _, row := range batch {
				for _, sha := range candidates {
					if !referenced[sha] && strings.Contains(row.Message, sha) {
						referenced[sha] = true
					}
				}
			}
			return nil
		}).Error
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "scan referenced images")
	}
	return referenced, nil
}
