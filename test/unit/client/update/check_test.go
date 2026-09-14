package update

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
)

func TestStartCheckPrintsNoticeForNewerRelease(t *testing.T) {
	t.Parallel()
	server, _ := newReleaseServer(t, "v9.9.9", nil, "")
	var out bytes.Buffer

	wait := update.StartCheck(t.Context(), update.CheckOptions{
		Current: "v0.1.0",
		Out:     &out,
		BaseURL: server.URL,
	})
	wait()

	if !strings.Contains(out.String(), "v9.9.9") || !strings.Contains(out.String(), "v0.1.0") {
		t.Fatalf("notice = %q, want latest and current version", out.String())
	}
}

func TestStartCheckIsSilentWhenUpToDate(t *testing.T) {
	t.Parallel()
	server, _ := newReleaseServer(t, "v0.2.2", nil, "")
	var out bytes.Buffer

	wait := update.StartCheck(t.Context(), update.CheckOptions{
		Current: "v0.2.2",
		Out:     &out,
		BaseURL: server.URL,
	})
	wait()

	if out.Len() != 0 {
		t.Fatalf("notice = %q, want empty output", out.String())
	}
}

func TestStartCheckIsSilentOnFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	var out bytes.Buffer

	wait := update.StartCheck(t.Context(), update.CheckOptions{
		Current: "v0.1.0",
		Out:     &out,
		BaseURL: server.URL,
	})
	wait()

	if out.Len() != 0 {
		t.Fatalf("notice = %q, want empty output", out.String())
	}
}
