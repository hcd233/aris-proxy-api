package cmd

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const recomputeBatchSize = 500

var databaseCmd = &cobra.Command{
	Use:   "database",
	Short: "Database Command Group",
	Long:  `Database command group for managing and operating database, including migration, backup and recovery, etc.`,
}

var migrateDatabaseCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate Database",
	Long:  `Execute database migration operation, update the database structure to the latest mode, then recompute message checksums and deduplicate.`,
	Run: func(cmd *cobra.Command, _ []string) {
		database.InitDatabase()
		db := database.GetDBInstance(cmd.Context())
		lo.Must0(db.AutoMigrate(dbmodel.Models...))
		recomputeAndDeduplicateMessages(db)
	},
}

// recomputeAndDeduplicateMessages 重算所有消息的 checksum 并去重
//
// 流程：
//
//  1. 分批读取所有 message，用新算法（nil schema）重算 checksum
//
//  2. 更新变化了的 checksum
//
//  3. 按 checksum+model 分组找出重复记录
//
//  4. 保留最小 ID，更新 session 表中的引用，软删除重复记录
//
//     @param db *gorm.DB
//     @author centonhuang
//     @update 2026-03-29 10:00:00
func recomputeAndDeduplicateMessages(db *gorm.DB) {
	log := logger.Logger()

	if !hasMessageTable(db) {
		log.Info("[Migrate] Message table not found, skip checksum recompute")
		return
	}

	updatedCount := recomputeChecksums(db, log)
	log.Info("[Migrate] Checksum recompute complete", zap.Int("updatedCount", updatedCount))

	deduplicatedCount := deduplicateMessages(db, log)
	log.Info("[Migrate] Deduplication complete", zap.Int("deduplicatedCount", deduplicatedCount))
}

// hasMessageTable 检查 messages 表是否存在
//
//	@param db *gorm.DB
//	@return bool
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func hasMessageTable(db *gorm.DB) bool {
	return db.Migrator().HasTable(&dbmodel.Message{})
}

// recomputeChecksums 按 Session 遍历，从 Tool 表查出 schema，用 schema-aware 算法重算消息 checksum
//
//	@param db *gorm.DB
//	@param log *zap.Logger
//	@return int 更新的记录数
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func recomputeChecksums(db *gorm.DB, log *zap.Logger) int {
	var (
		offset       int
		updatedCount int
	)

	for {
		var sessions []*dbmodel.Session
		if err := db.Select([]string{"id", "message_ids", "tool_ids"}).
			Where("deleted_at = 0").
			Order("id ASC").
			Offset(offset).Limit(recomputeBatchSize).
			Find(&sessions).Error; err != nil {
			log.Error("[Migrate] Failed to read sessions", zap.Error(err))
			return updatedCount
		}

		if len(sessions) == 0 {
			break
		}

		for _, s := range sessions {
			schemas := buildToolSchemaMap(db, log, s.ToolIDs)
			updatedCount += recomputeSessionMessages(db, log, s.MessageIDs, schemas)
		}

		log.Info("[Migrate] Processed session batch",
			zap.Int("offset", offset), zap.Int("batchSize", len(sessions)))
		offset += recomputeBatchSize
	}

	return updatedCount
}

// buildToolSchemaMap 从 Tool 表查出指定 ID 的工具，构建 ToolSchemaMap
//
//	@param db *gorm.DB
//	@param log *zap.Logger
//	@param toolIDs []uint
//	@return util.ToolSchemaMap
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func buildToolSchemaMap(db *gorm.DB, log *zap.Logger, toolIDs []uint) util.ToolSchemaMap {
	schemas := util.ToolSchemaMap{}
	if len(toolIDs) == 0 {
		return schemas
	}

	var tools []*dbmodel.Tool
	if err := db.Select([]string{"id", "tool"}).
		Where("id IN ? AND deleted_at = 0", toolIDs).
		Find(&tools).Error; err != nil {
		log.Error("[Migrate] Failed to read tools", zap.Error(err))
		return schemas
	}

	for _, t := range tools {
		if t.Tool != nil && t.Tool.Parameters != nil {
			schemas[t.Tool.Name] = t.Tool.Parameters
		}
	}

	return schemas
}

// recomputeSessionMessages 用给定的 ToolSchemaMap 重算指定消息的 checksum
//
//	@param db *gorm.DB
//	@param log *zap.Logger
//	@param messageIDs []uint
//	@param schemas util.ToolSchemaMap
//	@return int 更新的记录数
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func recomputeSessionMessages(db *gorm.DB, log *zap.Logger, messageIDs []uint, schemas util.ToolSchemaMap) int {
	if len(messageIDs) == 0 {
		return 0
	}

	var messages []*dbmodel.Message
	if err := db.Select([]string{"id", "model", "message", "check_sum"}).
		Where("id IN ? AND deleted_at = 0", messageIDs).
		Find(&messages).Error; err != nil {
		log.Error("[Migrate] Failed to read messages", zap.Error(err))
		return 0
	}

	updatedCount := 0
	for _, m := range messages {
		newCS := util.ComputeMessageChecksum(m.Message, schemas)
		if newCS == m.CheckSum {
			continue
		}
		if err := db.Model(&dbmodel.Message{}).Where("id = ?", m.ID).
			Update("check_sum", newCS).Error; err != nil {
			log.Error("[Migrate] Failed to update checksum",
				zap.Uint("messageID", m.ID), zap.Error(err))
			continue
		}
		updatedCount++
	}

	return updatedCount
}

// dupGroup 重复消息分组
type dupGroup struct {
	CheckSum string `gorm:"column:check_sum"`
	Model    string `gorm:"column:model"`
	MinID    uint   `gorm:"column:min_id"`
}

// deduplicateMessages 找出重复的 checksum+model 组合，保留最小 ID，更新 session 引用，软删除重复记录
//
//	@param db *gorm.DB
//	@param log *zap.Logger
//	@return int 删除的重复记录数
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func deduplicateMessages(db *gorm.DB, log *zap.Logger) int {
	var dups []dupGroup
	if err := db.Model(&dbmodel.Message{}).
		Select("check_sum, model, MIN(id) as min_id").
		Where("deleted_at = 0").
		Group("check_sum, model").
		Having("COUNT(*) > 1").
		Find(&dups).Error; err != nil {
		log.Error("[Migrate] Failed to find duplicates", zap.Error(err))
		return 0
	}

	if len(dups) == 0 {
		log.Info("[Migrate] No duplicates found")
		return 0
	}

	log.Info("[Migrate] Found duplicate groups", zap.Int("groups", len(dups)))

	totalRemoved := 0
	for _, dup := range dups {
		removed := deduplicateGroup(db, log, dup)
		totalRemoved += removed
	}

	return totalRemoved
}

// deduplicateGroup 对单个重复组执行去重：保留最小 ID，更新 session 引用，软删除其余
//
//	@param db *gorm.DB
//	@param log *zap.Logger
//	@param dup dupGroup
//	@return int 删除的记录数
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func deduplicateGroup(db *gorm.DB, log *zap.Logger, dup dupGroup) int {
	var ids []uint
	if err := db.Model(&dbmodel.Message{}).
		Where("check_sum = ? AND model = ? AND deleted_at = 0", dup.CheckSum, dup.Model).
		Order("id ASC").
		Pluck("id", &ids).Error; err != nil {
		log.Error("[Migrate] Failed to get duplicate IDs",
			zap.String("checksum", dup.CheckSum), zap.Error(err))
		return 0
	}

	if len(ids) <= 1 {
		return 0
	}

	keepID := ids[0]
	removeIDs := ids[1:]

	idReplaceMap := make(map[uint]uint, len(removeIDs))
	for _, rid := range removeIDs {
		idReplaceMap[rid] = keepID
	}

	updateSessionReferences(db, log, idReplaceMap)

	now := time.Now().UTC().Unix()
	if err := db.Model(&dbmodel.Message{}).
		Where("id IN ?", removeIDs).
		Update("deleted_at", now).Error; err != nil {
		log.Error("[Migrate] Failed to soft delete duplicates",
			zap.Uint("keepID", keepID), zap.Error(err))
		return 0
	}

	log.Info("[Migrate] Deduplicated group",
		zap.String("checksum", dup.CheckSum),
		zap.String("model", dup.Model),
		zap.Uint("keepID", keepID),
		zap.Int("removedCount", len(removeIDs)))

	return len(removeIDs)
}

// updateSessionReferences 更新 session 表中引用了被删除 message ID 的记录
//
//	@param db *gorm.DB
//	@param log *zap.Logger
//	@param idReplaceMap map[uint]uint 旧ID → 新ID 的映射
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func updateSessionReferences(db *gorm.DB, log *zap.Logger, idReplaceMap map[uint]uint) {
	var offset int

	for {
		var sessions []*dbmodel.Session
		if err := db.Select([]string{"id", "message_ids"}).
			Where("deleted_at = 0").
			Order("id ASC").
			Offset(offset).Limit(recomputeBatchSize).
			Find(&sessions).Error; err != nil {
			log.Error("[Migrate] Failed to read sessions", zap.Error(err))
			return
		}

		if len(sessions) == 0 {
			break
		}

		for _, s := range sessions {
			updated := false
			newIDs := make([]uint, len(s.MessageIDs))
			for i, mid := range s.MessageIDs {
				if replacement, shouldReplace := idReplaceMap[mid]; shouldReplace {
					newIDs[i] = replacement
					updated = true
				} else {
					newIDs[i] = mid
				}
			}

			if !updated {
				continue
			}

			s.MessageIDs = newIDs
			if err := db.Save(s).Error; err != nil {
				log.Error("[Migrate] Failed to update session message_ids",
					zap.Uint("sessionID", s.ID), zap.Error(err))
			}
		}

		offset += recomputeBatchSize
	}
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	rootCmd.AddCommand(databaseCmd)
}
