package main

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
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

// runMigrate 执行数据库结构迁移：仅 AutoMigrate 建表/建列/建索引。
//
// 注意：AutoMigrate 不修改已有同名索引的列组合。涉及索引变更的迁移
// （如多租户化 user_id 复合唯一索引）需部署后手工执行 SQL 重建，见 PR #162 描述。
func runMigrate(ctx context.Context) {
	lo.Must0(database.AutoMigrate(ctx))
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	rootCmd.AddCommand(databaseCmd)
}
