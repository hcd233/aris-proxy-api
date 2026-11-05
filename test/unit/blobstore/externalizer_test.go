package blobstore

import (
	"bytes"
	"encoding/base64"
	"math/rand/v2"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/test/unit/sqlitetest"
)

func smallPNG(t *testing.T, seed uint8) []byte {
	t.Helper()
	img := flatRGBA(4, 4)
	img.Pix[0] = seed
	return encodePNG(t, img)
}

func dataURI(raw []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
}

func imageMsg(urls ...string) *vo.UnifiedMessage {
	parts := []*vo.UnifiedContentPart{{Type: enum.ContentPartTypeText, Text: "看图"}}
	for _, u := range urls {
		parts = append(parts, &vo.UnifiedContentPart{Type: enum.ContentPartTypeImageURL, ImageURL: u, ImageDetail: "high"})
	}
	return &vo.UnifiedMessage{Role: enum.RoleUser, Content: &vo.UnifiedContent{Parts: parts}}
}

func getBlob(t *testing.T, e *testEnv, raw []byte) *dbmodel.Blob {
	t.Helper()
	row, err := dao.GetBlobDAO().GetBySHA256(e.db, vo.SHA256Hex(raw))
	if err != nil || row == nil {
		t.Fatalf("blob row missing: %v", err)
	}
	return row
}

type testEnv struct {
	db    *gorm.DB
	store *fakeStore
	ext   *blobstore.Externalizer
}

func newEnv(t *testing.T, withStore bool) *testEnv {
	t.Helper()
	db := openBlobDB(t)
	store := newFakeStore()
	var ext *blobstore.Externalizer
	if withStore {
		ext = blobstore.NewExternalizer(db, store)
	} else {
		ext = blobstore.NewExternalizer(db, nil)
	}
	return &testEnv{db: db, store: store, ext: ext}
}

func TestExternalizeUploadsAndReplaces(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	raw := smallPNG(t, 1)
	url := dataURI(raw)
	in := []*vo.UnifiedMessage{imageMsg(url)}

	out, stats := e.ext.Externalize(t.Context(), in)

	row := getBlob(t, e, raw)
	if row.Status != enum.BlobStatusUploaded || row.Data != nil || stats.Uploaded != 1 {
		t.Fatalf("row=%+v stats=%+v", row, stats)
	}
	if !bytes.Equal(e.store.objects[row.StorageKey], raw) {
		t.Fatal("object content mismatch")
	}
	part := out[0].Content.Parts[1]
	if part.ImageURL != vo.ImageRefOf(row.SHA256) || part.ImageMime != "image/png" || part.ImageSize != int64(len(raw)) || part.ImageDetail != "high" {
		t.Fatalf("part not replaced: %+v", part)
	}
	if in[0].Content.Parts[1].ImageURL != url {
		t.Fatal("input mutated")
	}
	if stats.SavedBytes != int64(len(url)) {
		t.Fatalf("saved bytes = %d, want %d", stats.SavedBytes, len(url))
	}
}

func TestExternalizeDedupWithinTask(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	url := dataURI(smallPNG(t, 2))
	_, stats := e.ext.Externalize(t.Context(), []*vo.UnifiedMessage{imageMsg(url), imageMsg(url)})
	if e.store.totalPuts() != 1 || stats.Uploaded != 1 {
		t.Fatalf("puts=%d stats=%+v", e.store.totalPuts(), stats)
	}
}

func TestExternalizeReusesExisting(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	raw := smallPNG(t, 3)
	insertBlob(t, e.db, &dbmodel.Blob{SHA256: vo.SHA256Hex(raw), Status: enum.BlobStatusUploaded, StorageKey: "k", Mime: "image/png", Size: 9})
	out, stats := e.ext.Externalize(t.Context(), []*vo.UnifiedMessage{imageMsg(dataURI(raw))})
	if e.store.totalPuts() != 0 || stats.Reused != 1 {
		t.Fatalf("puts=%d stats=%+v", e.store.totalPuts(), stats)
	}
	if got := out[0].Content.Parts[1]; got.ImageSize != 9 {
		t.Fatalf("expected db meta, got %+v", got)
	}
}

func TestExternalizeRepeatedImageKeepsDetail(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	url := dataURI(smallPNG(t, 4))
	msg := imageMsg(url, url)
	msg.Content.Parts[1].ImageDetail, msg.Content.Parts[2].ImageDetail = "low", "high"
	out, _ := e.ext.Externalize(t.Context(), []*vo.UnifiedMessage{msg})
	a, b := out[0].Content.Parts[1], out[0].Content.Parts[2]
	if a.ImageURL != b.ImageURL || a.ImageDetail != "low" || b.ImageDetail != "high" {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
}

func TestExternalizeSkipsNonData(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	in := []*vo.UnifiedMessage{
		imageMsg("https://example.com/a.png"),
		imageMsg(vo.ImageRefOf(shaOf('a'))),
		imageMsg(""),
		{Role: enum.RoleUser, Content: &vo.UnifiedContent{Text: "hi"}},
		{Role: enum.RoleAssistant},
	}
	out, stats := e.ext.Externalize(t.Context(), in)
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("message %d not returned as original pointer", i)
		}
	}
	if stats != (blobstore.ExternalizeStats{}) {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestExternalizeInvalidBase64Kept(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	url := "data:image/png;base64,@@@"
	out, stats := e.ext.Externalize(t.Context(), []*vo.UnifiedMessage{imageMsg(url)})
	if out[0].Content.Parts[1].ImageURL != url || stats.Failed != 1 {
		t.Fatalf("part=%+v stats=%+v", out[0].Content.Parts[1], stats)
	}
}

func TestExternalizePutFailureGoesPending(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.store.failPut = true
	raw := smallPNG(t, 5)
	before := time.Now()
	out, stats := e.ext.Externalize(t.Context(), []*vo.UnifiedMessage{imageMsg(dataURI(raw))})
	row := getBlob(t, e, raw)
	wantNext := before.Add(5 * time.Minute)
	if row.Status != enum.BlobStatusPending || !bytes.Equal(row.Data, raw) || row.Attempts != 1 || row.LastError == "" ||
		row.NextRetryAt == nil || row.NextRetryAt.Before(wantNext.Add(-time.Second)) || row.NextRetryAt.After(wantNext.Add(time.Minute)) {
		t.Fatalf("row=%+v", row)
	}
	if out[0].Content.Parts[1].ImageURL != vo.ImageRefOf(row.SHA256) || stats.Pending != 1 {
		t.Fatalf("part=%+v stats=%+v", out[0].Content.Parts[1], stats)
	}
}

func TestExternalizeNoStoreGoesPending(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	raw := smallPNG(t, 6)
	_, stats := e.ext.Externalize(t.Context(), []*vo.UnifiedMessage{imageMsg(dataURI(raw))})
	if row := getBlob(t, e, raw); row.Status != enum.BlobStatusPending || stats.Pending != 1 {
		t.Fatalf("row=%+v stats=%+v", row, stats)
	}
}

func TestExternalizeUnprocessable(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	raw := make([]byte, 3<<20)
	rng := rand.New(rand.NewPCG(7, 8))
	for i := range raw {
		raw[i] = byte(rng.UintN(256))
	}
	_, stats := e.ext.Externalize(t.Context(), []*vo.UnifiedMessage{imageMsg(dataURI(raw))})
	row := getBlob(t, e, raw)
	if row.Status != enum.BlobStatusUnprocessable || !bytes.Equal(row.Data, raw) || e.store.totalPuts() != 0 || stats.Unprocessable != 1 {
		t.Fatalf("status=%s puts=%d stats=%+v", row.Status, e.store.totalPuts(), stats)
	}
}

func TestExternalizeDBFailureKeepsBase64(t *testing.T) {
	t.Parallel()
	db := sqlitetest.OpenPrivateMemoryDB(t) // 不建 blobs 表
	ext := blobstore.NewExternalizer(db, newFakeStore())
	raw := smallPNG(t, 9)
	url := dataURI(raw)
	out, stats := ext.Externalize(t.Context(), []*vo.UnifiedMessage{imageMsg(url)})
	if out[0].Content.Parts[1].ImageURL != url || stats.Failed != 1 {
		t.Fatalf("part=%+v stats=%+v", out[0].Content.Parts[1], stats)
	}
	ref := imageMsg(vo.ImageRefOf(vo.SHA256Hex(raw)))
	if vo.ComputeMessageChecksum(out[0], "", nil) != vo.ComputeMessageChecksum(ref, "", nil) {
		t.Fatal("kept base64 must share checksum with reference form")
	}
}

func TestExternalizeTaskTimeoutGoesPending(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.store.blockPut = true
	e.ext.TaskTimeout = 10 * time.Millisecond
	a, b := smallPNG(t, 10), smallPNG(t, 11)
	_, stats := e.ext.Externalize(t.Context(), []*vo.UnifiedMessage{imageMsg(dataURI(a), dataURI(b))})
	if getBlob(t, e, a).Status != enum.BlobStatusPending || getBlob(t, e, b).Status != enum.BlobStatusPending {
		t.Fatal("expected both pending")
	}
	if e.store.putCalls > 1 || stats.Pending != 2 {
		t.Fatalf("putCalls=%d stats=%+v", e.store.putCalls, stats)
	}
}
