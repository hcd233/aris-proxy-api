package session_image

import (
	"context"
	"errors"
	"strings"
	"testing"

	sessionport "github.com/hcd233/aris-mass/internal/application/session/port"
	sessionquery "github.com/hcd233/aris-mass/internal/application/session/query"
	"github.com/hcd233/aris-mass/internal/common/ierr"
	"github.com/hcd233/aris-mass/internal/domain/blob"
)

var validSHA = strings.Repeat("ab", 32)

type stubMeta struct {
	got  sessionport.GetSessionMetaByUserQuery
	err  error
	view *sessionport.SessionMetaView
}

func (s *stubMeta) Handle(_ context.Context, q sessionport.GetSessionMetaByUserQuery) (*sessionport.SessionMetaView, error) {
	s.got = q
	return s.view, s.err
}

type fakeReader struct {
	referenced bool
	view       *blob.ImageView
	calls      int
}

func (f *fakeReader) IsReferenced(context.Context, []uint, string) (bool, error) {
	f.calls++
	return f.referenced, nil
}

func (f *fakeReader) Locate(context.Context, string) (*blob.ImageView, error) {
	f.calls++
	return f.view, nil
}

func (f *fakeReader) ReadBytes(context.Context, string) (mime string, data []byte, err error) {
	return "", nil, nil
}

func newHandler(meta *stubMeta, reader *fakeReader) sessionport.GetSessionImageHandler {
	return sessionquery.NewGetSessionImageHandler(meta, reader)
}

func okMeta() *stubMeta {
	return &stubMeta{view: &sessionport.SessionMetaView{ID: 1, MessageIDs: []uint{10, 11}}}
}

func TestPassesDemoScopeToMetaQuery(t *testing.T) {
	t.Parallel()
	meta := okMeta()
	reader := &fakeReader{referenced: true, view: &blob.ImageView{Mime: "image/png", Data: []byte("x")}}
	q := sessionport.GetSessionImageQuery{UserID: 7, IsAdmin: true, IsDemo: true, AllowedSessionIDs: []uint{1}, SessionID: 1, SHA256: validSHA}
	if _, err := newHandler(meta, reader).Handle(t.Context(), q); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !meta.got.IsDemo || len(meta.got.AllowedSessionIDs) != 1 || meta.got.UserID != 7 || meta.got.SessionID != 1 || !meta.got.IsAdmin {
		t.Fatalf("meta query = %+v", meta.got)
	}
}

func TestMetaRejectionPropagates(t *testing.T) {
	t.Parallel()
	meta := &stubMeta{err: ierr.New(ierr.ErrNoPermission, "no permission")}
	reader := &fakeReader{referenced: true}
	_, err := newHandler(meta, reader).Handle(t.Context(), sessionport.GetSessionImageQuery{SessionID: 1, SHA256: validSHA})
	if !errors.Is(err, ierr.ErrNoPermission) || reader.calls != 0 {
		t.Fatalf("err=%v readerCalls=%d", err, reader.calls)
	}
}

func TestUnreferencedIsNotFound(t *testing.T) {
	t.Parallel()
	reader := &fakeReader{view: &blob.ImageView{Mime: "image/png", Data: []byte("x")}}
	_, err := newHandler(okMeta(), reader).Handle(t.Context(), sessionport.GetSessionImageQuery{SessionID: 1, SHA256: validSHA})
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("err=%v", err)
	}
}

func TestInvalidSHAIsValidationError(t *testing.T) {
	t.Parallel()
	_, err := newHandler(okMeta(), &fakeReader{referenced: true}).Handle(t.Context(), sessionport.GetSessionImageQuery{SessionID: 1, SHA256: "ABC"})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("err=%v", err)
	}
}

func TestNonImageMimeHasNoDataURL(t *testing.T) {
	t.Parallel()
	reader := &fakeReader{referenced: true, view: &blob.ImageView{Mime: "application/octet-stream", Data: []byte("<svg/>")}}
	view, err := newHandler(okMeta(), reader).Handle(t.Context(), sessionport.GetSessionImageQuery{SessionID: 1, SHA256: validSHA})
	if err != nil || view.DataURL != "" || view.Mime != "application/octet-stream" {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestDataURLBuilt(t *testing.T) {
	t.Parallel()
	reader := &fakeReader{referenced: true, view: &blob.ImageView{Mime: "image/png", Data: []byte("png")}}
	view, err := newHandler(okMeta(), reader).Handle(t.Context(), sessionport.GetSessionImageQuery{SessionID: 1, SHA256: validSHA})
	if err != nil || view.DataURL != "data:image/png;base64,cG5n" || view.URL != "" {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestMissingBlobIsNotFound(t *testing.T) {
	t.Parallel()
	_, err := newHandler(okMeta(), &fakeReader{referenced: true}).Handle(t.Context(), sessionport.GetSessionImageQuery{SessionID: 1, SHA256: validSHA})
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("err=%v", err)
	}
}
