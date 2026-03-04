package cmd

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/storage"
	objdao "github.com/hcd233/aris-proxy-api/internal/infrastructure/storage/obj_dao"
	"github.com/hcd233/aris-proxy-api/internal/logger"
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

func init() {
	bucketCmd.AddCommand(createBucketCmd)
	objectCmd.AddCommand(bucketCmd)
	rootCmd.AddCommand(objectCmd)
}
