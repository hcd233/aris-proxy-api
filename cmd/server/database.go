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

// runMigrate 执行数据库迁移：先删/改旧列（ManualMigrations），再 AutoMigrate 建新列，最后回填存量数据。
//
// 顺序关键：旧 model_call_audits.model_id 为 uint 类型，必须先删旧列并 rename model→model_id，
// AutoMigrate 才能正确新建 text 列；回填依赖新列已存在，放最后。
func runMigrate(ctx context.Context) {
	lo.Must0(database.ManualMigrations(ctx))
	lo.Must0(database.AutoMigrate(ctx))
	lo.Must0(database.BackfillModelIDs(ctx))
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	rootCmd.AddCommand(databaseCmd)
}
