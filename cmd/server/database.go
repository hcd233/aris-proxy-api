package main

import (
	"context"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

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
		runMigrate(cmd.Context())
	},
}

// runMigrate 执行数据库结构迁移：AutoMigrate 建表/建列/建索引，随后幂等回填归属 ID。
//
// 注意：AutoMigrate 不修改已有同名索引的列组合。涉及索引变更的迁移
// （如多租户化 user_id 复合唯一索引）需部署后手工执行 SQL 重建，见 PR #162 描述。
//
// 回填必须先于新版本滚动部署：新版本按 api_key_id 过滤，未回填会让存量
// 会话对普通用户短时全不可见。回填幂等，重复执行安全。
func runMigrate(ctx context.Context) {
	db := database.InitDatabase()
	defer func() { _ = database.CloseDatabase(db) }() //nolint:errcheck // 进程即将退出，关闭失败无副作用

	lo.Must0(database.AutoMigrate(ctx, db))

	sessions, traces := lo.Must2(repository.BackfillSessionAPIKeyID(ctx, db))
	logger.Logger().Info("[Migrate] Backfilled api_key_id",
		zap.Int64("sessions", sessions), zap.Int64("traces", traces))
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	rootCmd.AddCommand(databaseCmd)
}
