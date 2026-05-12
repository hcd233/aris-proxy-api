package cmd

import (
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
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
		db := database.InitDatabase().WithContext(cmd.Context())
		lo.Must0(db.AutoMigrate(model.Models...))
	},
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	rootCmd.AddCommand(databaseCmd)
}
