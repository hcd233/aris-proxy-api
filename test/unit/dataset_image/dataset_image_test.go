package dataset_image

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/hcd233/aris-mass/internal/application/dataset/converter"
	datasetquery "github.com/hcd233/aris-mass/internal/application/dataset/query"
	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/domain/session"
)

var (
	shaA = strings.Repeat("a", 64)
	shaB = strings.Repeat("b", 64)
)

func imagePart(url string) *vo.UnifiedContentPart {
	return &vo.UnifiedContentPart{Type: enum.ContentPartTypeImageURL, ImageURL: url, ImageMime: "image/png", ImageSize: 2048}
}

func msgWith(role string, parts ...*vo.UnifiedContentPart) *session.MessageDetailProjection {
	return &session.MessageDetailProjection{Message: &vo.UnifiedMessage{Role: role, Content: &vo.UnifiedContent{Parts: parts}}}
}

func textPart(s string) *vo.UnifiedContentPart {
	return &vo.UnifiedContentPart{Type: enum.ContentPartTypeText, Text: s}
}

func passThrough(p *vo.UnifiedContentPart) (string, bool) { return p.ImageURL, true }

func TestNilResolverOutputUnchanged(t *testing.T) {
	t.Parallel()
	msgs := []*session.MessageDetailProjection{msgWith(enum.RoleUser, textPart("看图"), imagePart(vo.ImageRefOf(shaA)))}
	conv := converter.ConvertSession(msgs, nil)
	line, err := converter.MarshalJSONLine(conv)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(line) != `{"conversations":[{"from":"user","value":"看图"}]}` {
		t.Fatalf("line=%s", line)
	}
}

func TestPlaceholderMatchesImages(t *testing.T) {
	t.Parallel()
	msgs := []*session.MessageDetailProjection{
		msgWith(enum.RoleUser, textPart("比较"), imagePart("https://x/a.png"), textPart("和"), imagePart("https://x/b.png")),
	}
	conv := converter.ConvertSessionWithImages(msgs, nil, passThrough)
	if conv.Conversations[0].Value != "比较\n<image>\n和\n<image>" {
		t.Fatalf("value=%q", conv.Conversations[0].Value)
	}
	if len(conv.Images) != 2 || conv.Images[0] != "https://x/a.png" || conv.Images[1] != "https://x/b.png" {
		t.Fatalf("images=%v", conv.Images)
	}
}

func TestResolverFalseSkipsBoth(t *testing.T) {
	t.Parallel()
	msgs := []*session.MessageDetailProjection{msgWith(enum.RoleUser, imagePart("keep"), imagePart("drop"))}
	conv := converter.ConvertSessionWithImages(msgs, nil, func(p *vo.UnifiedContentPart) (string, bool) {
		return p.ImageURL, p.ImageURL == "keep"
	})
	if conv.Conversations[0].Value != "<image>" || len(conv.Images) != 1 {
		t.Fatalf("value=%q images=%v", conv.Conversations[0].Value, conv.Images)
	}
}

func TestToolRoleImageIncluded(t *testing.T) {
	t.Parallel()
	m := msgWith(enum.RoleTool, textPart("截图"), imagePart("https://x/c.png"))
	m.Message.ToolCallID = "call_1"
	conv := converter.ConvertSessionWithImages([]*session.MessageDetailProjection{m}, nil, passThrough)
	if conv.Conversations[0].Value != "截图\n<image>" || len(conv.Images) != 1 || conv.Conversations[0].Name != "call_1" {
		t.Fatalf("conv=%+v", conv)
	}
}

func TestTextOnlyHasNoImagesField(t *testing.T) {
	t.Parallel()
	msgs := []*session.MessageDetailProjection{{Message: &vo.UnifiedMessage{Role: enum.RoleUser, Content: &vo.UnifiedContent{Text: "hi"}}}}
	line, _ := converter.MarshalJSONLine(converter.ConvertSessionWithImages(msgs, nil, passThrough))
	if strings.Contains(string(line), "images") {
		t.Fatalf("line=%s", line)
	}
}

type fakeReader struct {
	data  map[string][]byte
	reads map[string]int
}

func (f *fakeReader) IsReferenced(context.Context, []uint, string) (bool, error) { return true, nil }

func (f *fakeReader) Locate(context.Context, string) (*blob.ImageView, error) { return nil, nil }

func (f *fakeReader) ReadBytes(_ context.Context, sha string) (mime string, data []byte, err error) {
	f.reads[sha]++
	d, ok := f.data[sha]
	if !ok {
		return "", nil, ierr.New(ierr.ErrDataNotExists, "missing")
	}
	return "image/png", d, nil
}

func newReader(data map[string][]byte) *fakeReader {
	return &fakeReader{data: data, reads: map[string]int{}}
}

func TestExportResolverRef(t *testing.T) {
	t.Parallel()
	r := datasetquery.NewExportImageResolver(t.Context(), newReader(map[string][]byte{shaA: []byte("png")}))
	got, ok := r.Resolve(imagePart(vo.ImageRefOf(shaA)))
	if !ok || got != "data:image/png;base64,cG5n" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}
}

func TestExportResolverDataURIAndExternalPassThrough(t *testing.T) {
	t.Parallel()
	r := datasetquery.NewExportImageResolver(t.Context(), newReader(nil))
	for _, url := range []string{"data:image/png;base64,AAAA", "https://x/a.png"} {
		if got, ok := r.Resolve(imagePart(url)); !ok || got != url {
			t.Errorf("%s: got=%q ok=%v", url, got, ok)
		}
	}
	if _, ok := r.Resolve(imagePart("")); ok {
		t.Error("empty url must be skipped")
	}
}

func TestExportResolverCachesAndCaps(t *testing.T) {
	t.Parallel()
	big := bytes.Repeat([]byte{1}, 30<<20)
	reader := newReader(map[string][]byte{shaA: big, shaB: big})
	r := datasetquery.NewExportImageResolver(t.Context(), reader)
	for range 2 {
		r.Resolve(imagePart(vo.ImageRefOf(shaA)))
	}
	if reader.reads[shaA] != 1 {
		t.Fatalf("same image read %d times", reader.reads[shaA])
	}
	r.Resolve(imagePart(vo.ImageRefOf(shaB)))
	r.Resolve(imagePart(vo.ImageRefOf(shaA)))
	if reader.reads[shaA] != 2 {
		t.Fatalf("cache must be cleared after exceeding cap, reads=%d", reader.reads[shaA])
	}
	if r.CachedBytes() > 64<<20 {
		t.Fatalf("cache bytes %d exceed cap", r.CachedBytes())
	}
}

func TestExportResolverReadFailure(t *testing.T) {
	t.Parallel()
	r := datasetquery.NewExportImageResolver(t.Context(), newReader(nil))
	if _, ok := r.Resolve(imagePart(vo.ImageRefOf(shaA))); ok || r.Failed() != 1 {
		t.Fatalf("ok=%v failed=%d", ok, r.Failed())
	}
}

func TestPreviewResolverNoFetch(t *testing.T) {
	t.Parallel()
	got, ok := datasetquery.PreviewImageResolver(imagePart(vo.ImageRefOf(shaA)))
	if !ok || got != "data:image/png;base64,<已省略 2 KB>" {
		t.Fatalf("got=%q", got)
	}
	raw := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 3000))
	got, _ = datasetquery.PreviewImageResolver(&vo.UnifiedContentPart{Type: enum.ContentPartTypeImageURL, ImageURL: "data:image/jpeg;base64," + raw})
	if got != "data:image/jpeg;base64,<已省略 3 KB>" {
		t.Fatalf("got=%q", got)
	}
	if got, _ := datasetquery.PreviewImageResolver(imagePart("https://x/a.png")); got != "https://x/a.png" {
		t.Fatalf("got=%q", got)
	}
}
