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

// runMigrate 执行数据库结构迁移：仅 AutoMigrate 建表/建列。
// 存量数据迁移（旧列改名、model_id 回填）已拆分到 `database migrate-data` 命令，部署后手动执行。
func runMigrate(ctx context.Context) {
	lo.Must0(database.AutoMigrate(ctx))
}

var migrateDataDatabaseCmd = &cobra.Command{
	Use:   "migrate-data",
	Short: "Migrate Existing Data",
	Long:  `Execute manual data migration: rename legacy columns (model→model_id etc.) and backfill models.model_id. Run once after deployment, idempotent and re-runnable.`,
	Run: func(cmd *cobra.Command, _ []string) {
		runMigrateData(cmd.Context())
	},
}

// runMigrateData 执行存量数据迁移：旧列改名（ManualMigrations）+ models.model_id 回填。
// 必须在服务部署（AutoMigrate 已建新列）后手动执行一次，幂等可重入。
func runMigrateData(ctx context.Context) {
	lo.Must0(database.ManualMigrations(ctx))
	lo.Must0(database.BackfillModelIDs(ctx))
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	databaseCmd.AddCommand(migrateDataDatabaseCmd)
	rootCmd.AddCommand(databaseCmd)
}
