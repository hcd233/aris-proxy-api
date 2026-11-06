package blobstore

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/test/unit/sqlitetest"
)

func openBlobDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := sqlitetest.OpenPrivateMemoryDB(t)
	if err := db.AutoMigrate(&dbmodel.Blob{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func shaOf(c byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func insertBlob(t *testing.T, db *gorm.DB, b *dbmodel.Blob) *dbmodel.Blob {
	t.Helper()
	if err := dao.GetBlobDAO().InsertIgnoreConflict(db, b); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := dao.GetBlobDAO().GetBySHA256(db, b.SHA256)
	if err != nil || got == nil {
		t.Fatalf("get after insert: %v", err)
	}
	return got
}

func pendingBlob(sha string, next time.Time) *dbmodel.Blob {
	return &dbmodel.Blob{
		SHA256: sha, Status: enum.BlobStatusPending, Data: []byte("img"), StorageKey: "k/" + sha,
		Mime: "image/png", Size: 3, Attempts: 1, NextRetryAt: &next,
	}
}

func TestInsertIgnoreConflictKeepsFirst(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	insertBlob(t, db, &dbmodel.Blob{SHA256: shaOf('a'), Status: enum.BlobStatusUploaded, StorageKey: "k", Mime: "image/png"})
	got := insertBlob(t, db, pendingBlob(shaOf('a'), time.Now()))
	if got.Status != enum.BlobStatusUploaded || got.Data != nil {
		t.Fatalf("second insert overwrote first: %+v", got)
	}
}

func TestFindMetaOmitsData(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	insertBlob(t, db, pendingBlob(shaOf('a'), time.Now()))
	rows, err := dao.GetBlobDAO().FindMetaBySHA256s(db, []string{shaOf('a'), shaOf('b')})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	if rows[0].Data != nil || rows[0].Mime != "image/png" || rows[0].Size != 3 || rows[0].StorageKey == "" {
		t.Fatalf("unexpected meta: %+v", rows[0])
	}
}

func TestGetMissingReturnsNil(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	if got, err := dao.GetBlobDAO().GetBySHA256(db, shaOf('z')); got != nil || err != nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if got, err := dao.GetBlobDAO().GetByID(db, 999); got != nil || err != nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestMarkUploadedClearsData(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	b := insertBlob(t, db, pendingBlob(shaOf('a'), time.Now()))
	meta := dao.BlobUploadedMeta{StorageKey: "images/aa/aa/x.jpg", Mime: "image/jpeg", Size: 7, Width: 3, Height: 2}
	if err := dao.GetBlobDAO().MarkUploaded(db, b.ID, meta); err != nil {
		t.Fatalf("mark: %v", err)
	}
	got, _ := dao.GetBlobDAO().GetByID(db, b.ID)
	if got.Status != enum.BlobStatusUploaded || got.Data != nil {
		t.Fatalf("unexpected: status=%s data=%v", got.Status, got.Data)
	}
	if got.StorageKey != meta.StorageKey || got.Mime != meta.Mime || got.Size != meta.Size || got.Width != 3 || got.Height != 2 {
		t.Fatalf("meta not written back: %+v", got)
	}
}

func TestMarkRetryCannotOverwriteUploaded(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	b := insertBlob(t, db, &dbmodel.Blob{SHA256: shaOf('a'), Status: enum.BlobStatusUploaded, StorageKey: "k"})
	blobDAO := dao.GetBlobDAO()
	if err := blobDAO.MarkRetry(db, b.ID, 2, "boom", time.Now()); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if err := blobDAO.MarkFailed(db, b.ID, 10, "boom"); err != nil {
		t.Fatalf("failed: %v", err)
	}
	got, _ := blobDAO.GetByID(db, b.ID)
	if got.Status != enum.BlobStatusUploaded || got.Attempts != 0 {
		t.Fatalf("uploaded row overwritten: %+v", got)
	}
}

func TestMarkRetryAndFailed(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	blobDAO := dao.GetBlobDAO()
	b := insertBlob(t, db, pendingBlob(shaOf('a'), time.Now()))
	next := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if err := blobDAO.MarkRetry(db, b.ID, 2, "boom", next); err != nil {
		t.Fatalf("retry: %v", err)
	}
	got, _ := blobDAO.GetByID(db, b.ID)
	if got.Status != enum.BlobStatusPending || got.Attempts != 2 || got.LastError != "boom" || !got.NextRetryAt.Equal(next) {
		t.Fatalf("after retry: %+v", got)
	}
	if err := blobDAO.MarkFailed(db, b.ID, 10, "final"); err != nil {
		t.Fatalf("failed: %v", err)
	}
	got, _ = blobDAO.GetByID(db, b.ID)
	if got.Status != enum.BlobStatusFailed || got.Attempts != 10 || got.Data == nil {
		t.Fatalf("after failed: %+v", got)
	}
}

func TestListDueRetryIDs(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	now := time.Now().UTC()
	late := insertBlob(t, db, pendingBlob(shaOf('a'), now.Add(-time.Minute)))
	early := insertBlob(t, db, pendingBlob(shaOf('b'), now.Add(-time.Hour)))
	insertBlob(t, db, pendingBlob(shaOf('c'), now.Add(time.Hour)))
	failed := pendingBlob(shaOf('d'), now.Add(-time.Hour))
	failed.Status = enum.BlobStatusFailed
	insertBlob(t, db, failed)

	ids, err := dao.GetBlobDAO().ListDueRetryIDs(db, now, 10)
	if err != nil || len(ids) != 2 || ids[0] != early.ID || ids[1] != late.ID {
		t.Fatalf("ids=%v err=%v want [%d %d]", ids, err, early.ID, late.ID)
	}
	ids, _ = dao.GetBlobDAO().ListDueRetryIDs(db, now, 1)
	if len(ids) != 1 {
		t.Fatalf("limit not applied: %v", ids)
	}
}

func TestResetFailed(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	failed := pendingBlob(shaOf('a'), time.Now().Add(time.Hour))
	failed.Status, failed.Attempts = enum.BlobStatusFailed, 10
	b := insertBlob(t, db, failed)
	insertBlob(t, db, pendingBlob(shaOf('b'), time.Now().Add(time.Hour)))

	now := time.Now().UTC().Truncate(time.Second)
	n, err := dao.GetBlobDAO().ResetFailed(db, now)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got, _ := dao.GetBlobDAO().GetByID(db, b.ID)
	if got.Status != enum.BlobStatusPending || got.Attempts != 0 || !got.NextRetryAt.Equal(now) {
		t.Fatalf("after reset: %+v", got)
	}
	if c, _ := dao.GetBlobDAO().CountByStatus(db, enum.BlobStatusPending); c != 2 {
		t.Fatalf("pending count=%d", c)
	}
}
