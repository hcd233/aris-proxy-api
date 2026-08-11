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
func runMigrate(ctx context.Context) {
	lo.Must0(database.AutoMigrate(ctx))
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	rootCmd.AddCommand(databaseCmd)
}
