package blobstore

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color/palette"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"strings"
	"testing"

	_ "golang.org/x/image/webp" // 注册 webp 解码器，供 DecodeConfig 校验

	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
)

const maxStored = 2 << 20

// tinyWebP 1x1 无损 WebP
const tinyWebP = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="

func noiseRGBA(w, h int) *image.RGBA {
	rng := rand.New(rand.NewPCG(1, 2))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = byte(rng.UintN(256))
	}
	return img
}

func flatRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 30, 120, 200, 255
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image, quality int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func decodeSize(t *testing.T, data []byte) (width, height int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output not decodable: %v", err)
	}
	return cfg.Width, cfg.Height
}

func TestSmallImageKeptVerbatim(t *testing.T) {
	t.Parallel()
	raw := encodePNG(t, flatRGBA(10, 8))
	res := blobstore.Compress(raw)
	if !bytes.Equal(res.Data, raw) || res.Mime != "image/png" || res.Width != 10 || res.Height != 8 || res.Unprocessable {
		t.Fatalf("unexpected result: mime=%s w=%d h=%d unprocessable=%v same=%v", res.Mime, res.Width, res.Height, res.Unprocessable, bytes.Equal(res.Data, raw))
	}
}

func TestLargePNGUnder2MB(t *testing.T) {
	t.Parallel()
	raw := encodePNG(t, noiseRGBA(1200, 1200))
	if len(raw) <= maxStored {
		t.Fatalf("fixture too small: %d", len(raw))
	}
	res := blobstore.Compress(raw)
	if len(res.Data) > maxStored || res.Unprocessable {
		t.Fatalf("size=%d unprocessable=%v", len(res.Data), res.Unprocessable)
	}
	w, h := decodeSize(t, res.Data)
	if w != res.Width || h != res.Height {
		t.Fatalf("reported %dx%d, actual %dx%d", res.Width, res.Height, w, h)
	}
}

func TestLargeJPEGUnder2MB(t *testing.T) {
	t.Parallel()
	raw := encodeJPEG(t, noiseRGBA(2000, 2000), 100)
	if len(raw) <= maxStored {
		t.Fatalf("fixture too small: %d", len(raw))
	}
	res := blobstore.Compress(raw)
	if len(res.Data) > maxStored || res.Mime != "image/jpeg" {
		t.Fatalf("size=%d mime=%s", len(res.Data), res.Mime)
	}
	decodeSize(t, res.Data)
}

func TestOversizeEdgeDownscaled(t *testing.T) {
	t.Parallel()
	res := blobstore.Compress(encodePNG(t, flatRGBA(5000, 100)))
	w, _ := decodeSize(t, res.Data)
	if res.Width != 4096 || w != 4096 || res.Mime != "image/png" {
		t.Fatalf("width=%d decoded=%d mime=%s", res.Width, w, res.Mime)
	}
}

func TestAnimatedGIFFirstFrame(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(3, 4))
	anim := &gif.GIF{}
	for range 2 {
		frame := image.NewPaletted(image.Rect(0, 0, 1500, 1500), palette.Plan9)
		for i := range frame.Pix {
			frame.Pix[i] = uint8(rng.UintN(256))
		}
		anim.Image = append(anim.Image, frame)
		anim.Delay = append(anim.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, anim); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	if buf.Len() <= maxStored {
		t.Fatalf("fixture too small: %d", buf.Len())
	}
	res := blobstore.Compress(buf.Bytes())
	if len(res.Data) > maxStored || res.Mime == "image/gif" || res.Unprocessable {
		t.Fatalf("size=%d mime=%s", len(res.Data), res.Mime)
	}
	decodeSize(t, res.Data)
}

func TestWebPDecoded(t *testing.T) {
	t.Parallel()
	raw, err := base64.StdEncoding.DecodeString(tinyWebP)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	res := blobstore.Compress(raw)
	if !bytes.Equal(res.Data, raw) || res.Mime != "image/webp" || res.Width != 1 {
		t.Fatalf("mime=%s width=%d same=%v", res.Mime, res.Width, bytes.Equal(res.Data, raw))
	}
}

// withExifOrientation 在 SOI 后插入只含 Orientation 标签的 APP1 段
func withExifOrientation(jpg []byte, orientation uint16) []byte {
	var tiff bytes.Buffer
	tiff.WriteString("MM\x00\x2a")
	_ = binary.Write(&tiff, binary.BigEndian, uint32(8))
	_ = binary.Write(&tiff, binary.BigEndian, uint16(1))
	_ = binary.Write(&tiff, binary.BigEndian, []uint16{0x0112, 3})
	_ = binary.Write(&tiff, binary.BigEndian, uint32(1))
	_ = binary.Write(&tiff, binary.BigEndian, []uint16{orientation, 0})
	_ = binary.Write(&tiff, binary.BigEndian, uint32(0))
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

func TestExifOrientationApplied(t *testing.T) {
	t.Parallel()
	raw := withExifOrientation(encodeJPEG(t, flatRGBA(5000, 1000), 80), 6)
	res := blobstore.Compress(raw)
	w, h := decodeSize(t, res.Data)
	if h <= w || res.Height != h {
		t.Fatalf("expected portrait after orientation, got %dx%d (reported %dx%d)", w, h, res.Width, res.Height)
	}
}

// withPNGSize 改写 PNG 的 IHDR 宽高并重算 CRC，只改头部声明
func withPNGSize(t *testing.T, pngBytes []byte, w, h uint32) []byte {
	t.Helper()
	out := bytes.Clone(pngBytes)
	const ihdrData = 16
	binary.BigEndian.PutUint32(out[ihdrData:], w)
	binary.BigEndian.PutUint32(out[ihdrData+4:], h)
	crc := crc32.ChecksumIEEE(out[12 : ihdrData+13])
	binary.BigEndian.PutUint32(out[ihdrData+13:], crc)
	if cfg, err := png.DecodeConfig(bytes.NewReader(out)); err != nil || cfg.Width != int(w) {
		t.Fatalf("patched header invalid: %v", err)
	}
	return out
}

func TestPixelBombNotDecoded(t *testing.T) {
	t.Parallel()
	raw := withPNGSize(t, encodePNG(t, flatRGBA(4, 4)), 20000, 20000)
	res := blobstore.Compress(raw)
	if !bytes.Equal(res.Data, raw) || res.Width != 0 || res.Unprocessable {
		t.Fatalf("width=%d unprocessable=%v same=%v", res.Width, res.Unprocessable, bytes.Equal(res.Data, raw))
	}
}

func TestUndecodableLargeIsUnprocessable(t *testing.T) {
	t.Parallel()
	raw := make([]byte, 3<<20)
	rng := rand.New(rand.NewPCG(5, 6))
	for i := range raw {
		raw[i] = byte(rng.UintN(256))
	}
	res := blobstore.Compress(raw)
	if !res.Unprocessable || !bytes.Equal(res.Data, raw) || res.Mime != "application/octet-stream" {
		t.Fatalf("unprocessable=%v mime=%s", res.Unprocessable, res.Mime)
	}
}

func TestTruncatedImageKept(t *testing.T) {
	t.Parallel()
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgo=")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	res := blobstore.Compress(raw)
	if !bytes.Equal(res.Data, raw) || res.Mime != "image/png" || res.Unprocessable {
		t.Fatalf("mime=%s unprocessable=%v", res.Mime, res.Unprocessable)
	}
}

func TestSVGNotTrusted(t *testing.T) {
	t.Parallel()
	res := blobstore.Compress([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	if res.Mime != "application/octet-stream" {
		t.Fatalf("mime=%s", res.Mime)
	}
}

func TestStorageKey(t *testing.T) {
	t.Parallel()
	sha := "abcd" + strings.Repeat("e", 60)
	cases := map[string]string{
		"image/jpeg":               "images/ab/cd/" + sha + ".jpg",
		"image/png":                "images/ab/cd/" + sha + ".png",
		"image/gif":                "images/ab/cd/" + sha + ".gif",
		"image/webp":               "images/ab/cd/" + sha + ".webp",
		"application/octet-stream": "images/ab/cd/" + sha + ".bin",
	}
	for mime, want := range cases {
		if got := blobstore.StorageKey(sha, mime); got != want {
			t.Errorf("%s: got %s, want %s", mime, got, want)
		}
	}
}
