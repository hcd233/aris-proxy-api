package main

import (
	"context"
	"time"

	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	"github.com/hcd233/aris-mass/internal/infrastructure/database"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-mass/internal/infrastructure/objectstore"
	"github.com/hcd233/aris-mass/internal/infrastructure/storage"
	objdao "github.com/hcd233/aris-mass/internal/infrastructure/storage/obj_dao"
	"github.com/hcd233/aris-mass/internal/logger"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var objectCmd = &cobra.Command{
	Use:   "object",
	Short: "Object Storage Command Group",
	Long:  `Object storage command group for managing and operating object storage, including creating buckets, creating directories, uploading files, etc.`,
}

var bucketCmd = &cobra.Command{
	Use:   "bucket",
	Short: "Bucket Command Group",
	Long:  `Bucket command group for managing and operating buckets, including creating buckets, deleting buckets, etc.`,
}

var createBucketCmd = &cobra.Command{
	Use:   "create",
	Short: "Create Bucket",
	Long:  `Create a bucket.`,
	Run: func(_ *cobra.Command, _ []string) {
		ctx := context.Background()
		logger := logger.Logger()
		storage.InitObjectStorage()

		audioObjDAO := objdao.GetAudioObjDAO()
		lo.Must0(audioObjDAO.CreateBucket(ctx))

		logger.Info("[Object Storage] Bucket created",
			zap.String("bucket", audioObjDAO.GetBucketName(ctx)))
	},
}

var migrateOpts blobstore.MigrateOptions

var migrateRetryFailed bool

var migrateObjectCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate Message Images",
	Long: `Move base64 images stored in messages into blobs and object storage, replacing them with references.
Safe to re-run: rows already migrated are skipped. Use --start-id to resume from the last logged ID.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runMigrateImages(cmd.Context())
	},
}

// runMigrateImages 执行存量图片迁移；--retry-failed 只把 failed 的 blob 重置为 pending
func runMigrateImages(ctx context.Context) error {
	log := logger.Logger()
	db := database.InitDatabase()
	defer func() { _ = database.CloseDatabase(db) }() //nolint:errcheck // 进程即将退出，关闭失败无副作用

	if migrateRetryFailed {
		n, err := dao.GetBlobDAO().ResetFailed(db.WithContext(ctx), time.Now().UTC())
		if err != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, err, "reset failed blobs")
		}
		log.Info("[BlobMigrate] Reset failed blobs to pending", zap.Int64("count", n))
		return nil
	}

	store := objectstore.NewFromConfig()
	if store == nil && !migrateOpts.DryRun {
		return ierr.New(ierr.ErrValidation, "object storage is not configured, set OBJECT_STORAGE_* or use --dry-run")
	}
	stats, err := blobstore.NewMigrator(db, blobstore.NewExternalizer(db, store)).Run(ctx, migrateOpts)
	log.Info("[BlobMigrate] Migration finished",
		zap.Bool("dryRun", migrateOpts.DryRun), zap.Uint("lastID", stats.LastID),
		zap.Int64("scanned", stats.Scanned), zap.Int64("migrated", stats.Migrated),
		zap.Int64("parseFailed", stats.ParseFailed), zap.Int64("checksumKept", stats.ChecksumKept),
		zap.Int("uploaded", stats.Blobs.Uploaded), zap.Int("pending", stats.Blobs.Pending),
		zap.Int("unprocessable", stats.Blobs.Unprocessable), zap.Int("reused", stats.Blobs.Reused),
		zap.Int("failed", stats.Blobs.Failed), zap.Int64("savedBytes", stats.Blobs.SavedBytes))
	return err
}

func init() {
	migrateObjectCmd.Flags().IntVar(&migrateOpts.BatchSize, "batch-size", constant.BlobMigrateDefaultBatchSize, "message rows per batch")
	migrateObjectCmd.Flags().UintVar(&migrateOpts.StartID, "start-id", 0, "only process messages with id greater than this")
	migrateObjectCmd.Flags().IntVar(&migrateOpts.Limit, "limit", 0, "max message rows to process, 0 for no limit")
	migrateObjectCmd.Flags().BoolVar(&migrateOpts.DryRun, "dry-run", false, "count rows and estimated savings without writing")
	migrateObjectCmd.Flags().BoolVar(&migrateRetryFailed, "retry-failed", false, "reset failed blobs to pending and exit")
	migrateObjectCmd.MarkFlagsMutuallyExclusive("dry-run", "retry-failed")

	bucketCmd.AddCommand(createBucketCmd)
	objectCmd.AddCommand(bucketCmd)
	objectCmd.AddCommand(migrateObjectCmd)
	rootCmd.AddCommand(objectCmd)
}
