package blobstore

import (
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/test/unit/sqlitetest"
)

type migrateEnv struct {
	*testEnv
	migrator *blobstore.Migrator
}

func newMigrateEnv(t *testing.T) *migrateEnv {
	t.Helper()
	e := newEnv(t, true)
	if err := e.db.AutoMigrate(&dbmodel.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &migrateEnv{testEnv: e, migrator: blobstore.NewMigrator(e.db, e.ext)}
}

func seedMessage(t *testing.T, db *gorm.DB, msg *vo.UnifiedMessage, checksum string, deletedAt int64) *dbmodel.Message {
	t.Helper()
	row := &dbmodel.Message{Message: msg, CheckSum: checksum}
	row.DeletedAt = deletedAt
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}
	return row
}

func reloadMessage(t *testing.T, db *gorm.DB, id uint) *dbmodel.Message {
	t.Helper()
	var row dbmodel.Message
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatalf("reload message: %v", err)
	}
	return &row
}

func runMigrate(t *testing.T, m *blobstore.Migrator, opts blobstore.MigrateOptions) blobstore.MigrateStats {
	t.Helper()
	stats, err := m.Run(t.Context(), opts)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return stats
}

func TestMigrateReplacesAndRecomputesChecksum(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	raw := smallPNG(t, 21)
	row := seedMessage(t, e.db, imageMsg(dataURI(raw)), "old-1", 0)

	stats := runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 10})

	got := reloadMessage(t, e.db, row.ID)
	if got.Message.Content.Parts[1].ImageURL != vo.ImageRefOf(vo.SHA256Hex(raw)) {
		t.Fatalf("not replaced: %+v", got.Message.Content.Parts[1])
	}
	if got.CheckSum != vo.ComputeMessageChecksum(got.Message, got.ModelID, nil) {
		t.Fatalf("checksum not recomputed: %s", got.CheckSum)
	}
	if stats.Scanned != 1 || stats.Migrated != 1 || stats.Blobs.Uploaded != 1 || stats.LastID != row.ID {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	seedMessage(t, e.db, imageMsg(dataURI(smallPNG(t, 22))), "old", 0)
	runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 10})
	stats := runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 10})
	var blobs int64
	e.db.Model(&dbmodel.Blob{}).Count(&blobs)
	if stats.Scanned != 0 || blobs != 1 {
		t.Fatalf("stats=%+v blobs=%d", stats, blobs)
	}
}

func TestMigrateIncludesSoftDeleted(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	row := seedMessage(t, e.db, imageMsg(dataURI(smallPNG(t, 23))), "old", 123)
	runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 10})
	if got := reloadMessage(t, e.db, row.ID); strings.HasPrefix(got.Message.Content.Parts[1].ImageURL, "data:") {
		t.Fatal("soft-deleted row not migrated")
	}
}

func TestMigrateChecksumConflictKeepsOld(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	raw := smallPNG(t, 24)
	row := seedMessage(t, e.db, imageMsg(dataURI(raw)), "old", 0)
	ref := imageMsg(vo.ImageRefOf(vo.SHA256Hex(raw)))
	seedMessage(t, e.db, ref, vo.ComputeMessageChecksum(ref, "", nil), 0)

	stats := runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 10})

	got := reloadMessage(t, e.db, row.ID)
	if got.CheckSum != "old" || strings.HasPrefix(got.Message.Content.Parts[1].ImageURL, "data:") || stats.ChecksumKept != 1 {
		t.Fatalf("checksum=%s part=%+v stats=%+v", got.CheckSum, got.Message.Content.Parts[1], stats)
	}
}

func TestMigrateToolCallsKeepsChecksum(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	msg := imageMsg(dataURI(smallPNG(t, 25)))
	msg.Role = enum.RoleAssistant
	msg.ToolCalls = []*vo.UnifiedToolCall{{ID: "c1", Name: "search", Arguments: `{"q":"x"}`}}
	row := seedMessage(t, e.db, msg, "old-tc", 0)
	runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 10})
	got := reloadMessage(t, e.db, row.ID)
	if got.CheckSum != "old-tc" || strings.HasPrefix(got.Message.Content.Parts[1].ImageURL, "data:") {
		t.Fatalf("checksum=%s part=%+v", got.CheckSum, got.Message.Content.Parts[1])
	}
}

func TestMigratePutFailureStillReplaces(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	e.store.failPut = true
	raw := smallPNG(t, 26)
	row := seedMessage(t, e.db, imageMsg(dataURI(raw)), "old", 0)
	runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 10})
	if got := reloadMessage(t, e.db, row.ID); got.Message.Content.Parts[1].ImageURL != vo.ImageRefOf(vo.SHA256Hex(raw)) {
		t.Fatalf("part=%+v", got.Message.Content.Parts[1])
	}
	if b := getBlob(t, e.testEnv, raw); b.Status != enum.BlobStatusPending {
		t.Fatalf("status=%s", b.Status)
	}
}

func TestMigrateBlobWriteFailureRetriedLater(t *testing.T) {
	t.Parallel()
	db := sqlitetest.OpenPrivateMemoryDB(t)
	if err := db.AutoMigrate(&dbmodel.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	m := blobstore.NewMigrator(db, blobstore.NewExternalizer(db, blobstore.NewSyncUploader(db, newFakeStore())))
	url := dataURI(smallPNG(t, 27))
	row := seedMessage(t, db, imageMsg(url), "old", 0)

	runMigrate(t, m, blobstore.MigrateOptions{BatchSize: 10})
	if got := reloadMessage(t, db, row.ID); got.Message.Content.Parts[1].ImageURL != url {
		t.Fatal("message must keep base64 when blobs write fails")
	}
	if err := db.AutoMigrate(&dbmodel.Blob{}); err != nil {
		t.Fatalf("migrate blob: %v", err)
	}
	runMigrate(t, m, blobstore.MigrateOptions{BatchSize: 10})
	if got := reloadMessage(t, db, row.ID); got.Message.Content.Parts[1].ImageURL == url {
		t.Fatal("rerun did not migrate")
	}
}

func TestMigrateParseFailureSkipped(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	now := time.Now()
	if err := e.db.Exec(`INSERT INTO messages (model_id, message, check_sum, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, ?, 0)`,
		"", `{"role":"user","content":[{"type":"image_url","image_url":"data:image/png;base64,AAAA"`, "broken", now, now).Error; err != nil {
		t.Fatalf("insert broken: %v", err)
	}
	row := seedMessage(t, e.db, imageMsg(dataURI(smallPNG(t, 28))), "ok", 0)
	stats := runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 1})
	if stats.ParseFailed != 1 || stats.Migrated != 1 {
		t.Fatalf("stats=%+v", stats)
	}
	if got := reloadMessage(t, e.db, row.ID); strings.HasPrefix(got.Message.Content.Parts[1].ImageURL, "data:") {
		t.Fatal("valid row not migrated")
	}
}

func TestMigrateDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	url := dataURI(smallPNG(t, 29))
	row := seedMessage(t, e.db, imageMsg(url), "old", 0)
	stats := runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 10, DryRun: true})
	var blobs int64
	e.db.Model(&dbmodel.Blob{}).Count(&blobs)
	got := reloadMessage(t, e.db, row.ID)
	if got.Message.Content.Parts[1].ImageURL != url || got.CheckSum != "old" || blobs != 0 || e.store.putCalls != 0 {
		t.Fatalf("dry run wrote data: blobs=%d puts=%d", blobs, e.store.putCalls)
	}
	if stats.Migrated != 1 || stats.Blobs.SavedBytes != int64(len(url)) {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestMigrateStartIDAndLimit(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	rows := make([]*dbmodel.Message, 3)
	for i := range rows {
		rows[i] = seedMessage(t, e.db, imageMsg(dataURI(smallPNG(t, uint8(30+i)))), "old-"+string(rune('a'+i)), 0)
	}
	stats := runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 1, StartID: rows[0].ID, Limit: 1})
	if stats.Scanned != 1 || stats.LastID != rows[1].ID {
		t.Fatalf("stats=%+v", stats)
	}
	if got := reloadMessage(t, e.db, rows[0].ID); !strings.HasPrefix(got.Message.Content.Parts[1].ImageURL, "data:") {
		t.Fatal("row before start id migrated")
	}
	if got := reloadMessage(t, e.db, rows[2].ID); !strings.HasPrefix(got.Message.Content.Parts[1].ImageURL, "data:") {
		t.Fatal("row beyond limit migrated")
	}
}

func TestMigrateTextOnlyUntouched(t *testing.T) {
	t.Parallel()
	e := newMigrateEnv(t)
	row := seedMessage(t, e.db, &vo.UnifiedMessage{Role: enum.RoleUser, Content: &vo.UnifiedContent{Text: "hi"}}, "text", 0)
	before := reloadMessage(t, e.db, row.ID)
	stats := runMigrate(t, e.migrator, blobstore.MigrateOptions{BatchSize: 10})
	after := reloadMessage(t, e.db, row.ID)
	if stats.Scanned != 0 || !after.UpdatedAt.Equal(before.UpdatedAt) || after.CheckSum != "text" {
		t.Fatalf("stats=%+v", stats)
	}
}
