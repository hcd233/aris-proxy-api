package cmd

import (
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
		lo.Must0(database.AutoMigrate(cmd.Context()))
		// 在 AutoMigrate 之后跑标准 BTREE 复合索引 + message_count/tool_count 存量回填。
		// 全部 SQL 幂等可重入；任意一条失败 panic，让 K8s migrate Job 直接红灯，
		// 避免半完成的 schema 进入流量层（参考 75658e5 / 11e4602 的事故）。
		lo.Must0(database.PostMigrate(cmd.Context()))
	},
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	rootCmd.AddCommand(databaseCmd)
}
