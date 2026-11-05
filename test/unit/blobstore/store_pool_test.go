package blobstore

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/dto"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/internal/infrastructure/pool"
	"github.com/hcd233/aris-mass/internal/util"
)

// TestStoreTaskPersistsReference 落库路径：message 行只存引用，图片进 blobs
func TestStoreTaskPersistsReference(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	if err := db.AutoMigrate(&dbmodel.Session{}, &dbmodel.Message{}, &dbmodel.Tool{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	persisted := make(chan struct{}, 1)
	if err := db.Callback().Create().After("gorm:create").Register("test:session_created", func(tx *gorm.DB) {
		if tx.Statement.Table != "sessions" {
			return
		}
		select {
		case persisted <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}

	pm := pool.NewPoolManager(db, nil, nil, nil, blobstore.NewExternalizer(db, newFakeStore()))
	raw := smallPNG(t, 42)
	if err := pm.SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:      util.CopyContextValues(context.Background()),
		Messages: []*vo.UnifiedMessage{imageMsg(dataURI(raw))},
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	select {
	case <-persisted:
	case <-time.After(5 * time.Second):
		t.Fatal("session not persisted within timeout")
	}

	var stored []string
	if err := db.Table("messages").Pluck("message", &stored).Error; err != nil || len(stored) != 1 {
		t.Fatalf("messages=%v err=%v", stored, err)
	}
	if strings.Contains(stored[0], "base64") || !strings.Contains(stored[0], vo.ImageRefOf(vo.SHA256Hex(raw))) {
		t.Fatalf("message not externalized: %s", stored[0])
	}
	var blobs int64
	db.Model(&dbmodel.Blob{}).Count(&blobs)
	if blobs != 1 {
		t.Fatalf("blobs=%d", blobs)
	}
}
