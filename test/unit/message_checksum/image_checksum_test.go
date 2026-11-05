package message_checksum

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\nfake-image-bytes-for-checksum")

func pngSHA() string {
	sum := sha256.Sum256(pngBytes)
	return hex.EncodeToString(sum[:])
}

func imageMessage(url, mime string, size int64) *vo.UnifiedMessage {
	return &vo.UnifiedMessage{Role: enum.RoleUser, Content: &vo.UnifiedContent{Parts: []*vo.UnifiedContentPart{
		{Type: enum.ContentPartTypeText, Text: "看图"},
		{Type: enum.ContentPartTypeImageURL, ImageURL: url, ImageDetail: "high", ImageMime: mime, ImageSize: size},
	}}}
}

func TestChecksumImageFormsEqual(t *testing.T) {
	t.Parallel()
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	dataURI := vo.ComputeMessageChecksum(imageMessage("data:image/png;base64,"+b64, "", 0), "", nil)
	otherMime := vo.ComputeMessageChecksum(imageMessage("data:image/jpeg;base64,"+b64, "", 0), "", nil)
	ref := vo.ComputeMessageChecksum(imageMessage(vo.ImageRefOf(pngSHA()), "image/png", 1234), "", nil)
	if dataURI != otherMime || dataURI != ref {
		t.Fatalf("checksums differ: dataURI=%s otherMime=%s ref=%s", dataURI, otherMime, ref)
	}
}

func TestChecksumDifferentImagesDiffer(t *testing.T) {
	t.Parallel()
	a := vo.ComputeMessageChecksum(imageMessage("data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("a")), "", 0), "", nil)
	b := vo.ComputeMessageChecksum(imageMessage("data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("b")), "", 0), "", nil)
	if a == b {
		t.Fatalf("different images must not share checksum: %s", a)
	}
}

func TestChecksumExternalURLKeepsLiteral(t *testing.T) {
	t.Parallel()
	a := vo.ComputeMessageChecksum(imageMessage("https://example.com/a.png", "", 0), "", nil)
	b := vo.ComputeMessageChecksum(imageMessage("https://example.com/b.png", "", 0), "", nil)
	if a == b {
		t.Fatalf("different external urls must not share checksum: %s", a)
	}
	ref := vo.ComputeMessageChecksum(imageMessage(vo.ImageRefOf(pngSHA()), "", 0), "", nil)
	if a == ref {
		t.Fatal("external url must not be treated as image hash")
	}
}

func TestChecksumDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	msg := imageMessage(url, "image/png", 9)
	_ = vo.ComputeMessageChecksum(msg, "", nil)
	part := msg.Content.Parts[1]
	if part.ImageURL != url || part.ImageMime != "image/png" || part.ImageSize != 9 {
		t.Fatalf("input mutated: %+v", part)
	}
}

func TestChecksumInvalidBase64Stable(t *testing.T) {
	t.Parallel()
	first := vo.ComputeMessageChecksum(imageMessage("data:image/png;base64,@@@", "", 0), "", nil)
	second := vo.ComputeMessageChecksum(imageMessage("data:image/png;base64,@@@", "", 0), "", nil)
	other := vo.ComputeMessageChecksum(imageMessage("data:image/png;base64,###", "", 0), "", nil)
	if first != second || first == other {
		t.Fatalf("invalid base64 must keep literal and stay stable: first=%s second=%s other=%s", first, second, other)
	}
}

func TestParseImageDataURI(t *testing.T) {
	t.Parallel()
	std := base64.StdEncoding.EncodeToString(pngBytes)
	raw := base64.RawStdEncoding.EncodeToString(pngBytes)
	valid := []struct{ name, url string }{
		{"canonical", "data:image/png;base64," + std},
		{"charset param", "data:image/png;charset=utf-8;base64," + std},
		{"no padding", "data:image/png;base64," + raw},
		{"with newlines", "data:image/png;base64," + std[:10] + "\n" + std[10:20] + "\r\n" + std[20:]},
	}
	for _, tc := range valid {
		mime, data, ok := vo.ParseImageDataURI(tc.url)
		if !ok || mime != "image/png" || vo.SHA256Hex(data) != pngSHA() {
			t.Errorf("%s: ok=%v mime=%q sha=%s", tc.name, ok, mime, vo.SHA256Hex(data))
		}
	}
	invalid := []string{"data:image/svg+xml,<svg/>", "https://x/a.png", "", "data:image/png;base64,@@@"}
	for _, url := range invalid {
		if _, _, ok := vo.ParseImageDataURI(url); ok {
			t.Errorf("%q: expected ok=false", url)
		}
	}
}

func TestParseImageRef(t *testing.T) {
	t.Parallel()
	sha := pngSHA()
	if got, ok := vo.ParseImageRef(vo.ImageRefOf(sha)); !ok || got != sha {
		t.Fatalf("valid ref: got=%q ok=%v", got, ok)
	}
	invalid := []string{
		vo.ImageRefOf(sha[:63]),
		vo.ImageRefOf(sha + "0"),
		vo.ImageRefOf("A" + sha[1:]),
		vo.ImageRefOf("g" + sha[1:]),
		sha,
		"",
	}
	for _, url := range invalid {
		if _, ok := vo.ParseImageRef(url); ok {
			t.Errorf("%q: expected ok=false", url)
		}
	}
}
