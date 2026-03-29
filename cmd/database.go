package cmd

import (
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const migrateBatchSize = 500

var databaseCmd = &cobra.Command{
	Use:   "database",
	Short: "Database Command Group",
	Long:  `Database command group for managing and operating database, including migration, backup and recovery, etc.`,
}

var migrateDatabaseCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate Database",
	Long:  `Execute database migration operation, update the database structure to the latest mode.`,
	Run: func(cmd *cobra.Command, _ []string) {
		database.InitDatabase()
		db := database.GetDBInstance(cmd.Context())
		lo.Must0(db.AutoMigrate(dbmodel.Models...))
		repairDeletedMessageReferences(db)
	},
}

// repairDeletedMessageReferences 修复 session 中引用已软删除 message 的问题
//
// 对每个 session 的 message_ids，检查是否有已删除的 message，
// 如果有则按 checksum+model 查找存活的替代记录进行替换。
//
//	@param db *gorm.DB
//	@author centonhuang
//	@update 2026-03-29 18:00:00
func repairDeletedMessageReferences(db *gorm.DB) {
	log := logger.Logger()

	if !db.Migrator().HasTable(&dbmodel.Session{}) {
		log.Info("[Migrate] Session table not found, skip repair")
		return
	}

	var (
		offset        int
		repairedCount int
	)

	for {
		var sessions []*dbmodel.Session
		if err := db.Select([]string{"id", "message_ids"}).
			Where("deleted_at = 0").
			Order("id ASC").
			Offset(offset).Limit(migrateBatchSize).
			Find(&sessions).Error; err != nil {
			log.Error("[Migrate] Failed to read sessions", zap.Error(err))
			return
		}

		if len(sessions) == 0 {
			break
		}

		for _, s := range sessions {
			if repairSession(db, log, s) {
				repairedCount++
			}
		}

		offset += migrateBatchSize
	}

	log.Info("[Migrate] Repair complete", zap.Int("repairedSessions", repairedCount))
}

// repairSession 修复单个 session 中引用已删除 message 的问题
//
//	@param db *gorm.DB
//	@param log *zap.Logger
//	@param s *dbmodel.Session
//	@return bool 是否有修复
//	@author centonhuang
//	@update 2026-03-29 18:00:00
func repairSession(db *gorm.DB, log *zap.Logger, s *dbmodel.Session) bool {
	if len(s.MessageIDs) == 0 {
		return false
	}

	// 批量查出这些 message 的存活状态
	var aliveMessages []*dbmodel.Message
	if err := db.Select([]string{"id"}).
		Where("id IN ? AND deleted_at = 0", s.MessageIDs).
		Find(&aliveMessages).Error; err != nil {
		log.Error("[Migrate] Failed to check alive messages",
			zap.Uint("sessionID", s.ID), zap.Error(err))
		return false
	}

	aliveSet := lo.SliceToMap(aliveMessages, func(m *dbmodel.Message) (uint, bool) {
		return m.ID, true
	})

	// 找出已删除的 message IDs
	deadIDs := lo.Filter(s.MessageIDs, func(id uint, _ int) bool {
		return !aliveSet[id]
	})

	if len(deadIDs) == 0 {
		return false
	}

	// 批量查出已删除 message 的 checksum+model
	replacementMap := buildReplacementMap(db, log, deadIDs)
	if len(replacementMap) == 0 {
		return false
	}

	// 替换 message_ids
	newIDs := make([]uint, len(s.MessageIDs))
	changed := false
	for i, mid := range s.MessageIDs {
		if replacement, ok := replacementMap[mid]; ok {
			newIDs[i] = replacement
			changed = true
		} else {
			newIDs[i] = mid
		}
	}

	if !changed {
		return false
	}

	s.MessageIDs = newIDs
	if err := db.Model(s).Select("message_ids").Updates(s).Error; err != nil {
		log.Error("[Migrate] Failed to update session message_ids",
			zap.Uint("sessionID", s.ID), zap.Error(err))
		return false
	}

	log.Info("[Migrate] Repaired session",
		zap.Uint("sessionID", s.ID),
		zap.Int("replacedCount", len(deadIDs)))
	return true
}

// buildReplacementMap 对已删除的 message，按 checksum+model 查找存活的替代记录
//
//	@param db *gorm.DB
//	@param log *zap.Logger
//	@param deadIDs []uint 已删除的 message IDs
//	@return map[uint]uint 旧ID → 替代ID 的映射
//	@author centonhuang
//	@update 2026-03-29 18:00:00
func buildReplacementMap(db *gorm.DB, log *zap.Logger, deadIDs []uint) map[uint]uint {
	// 查出已删除 message 的 checksum+model（不过滤 deleted_at，因为它们已被软删除）
	var deadMessages []*dbmodel.Message
	if err := db.Select([]string{"id", "check_sum", "model"}).
		Where("id IN ?", deadIDs).
		Find(&deadMessages).Error; err != nil {
		log.Error("[Migrate] Failed to read dead messages", zap.Error(err))
		return nil
	}

	replacementMap := make(map[uint]uint, len(deadMessages))
	for _, dm := range deadMessages {
		// 按 checksum+model 查找存活的替代记录
		var alive dbmodel.Message
		if err := db.Select([]string{"id"}).
			Where("check_sum = ? AND model = ? AND deleted_at = 0", dm.CheckSum, dm.Model).
			First(&alive).Error; err != nil {
			continue
		}
		replacementMap[dm.ID] = alive.ID
	}

	return replacementMap
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	rootCmd.AddCommand(databaseCmd)
}
