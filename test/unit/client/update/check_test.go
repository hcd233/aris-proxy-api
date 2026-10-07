package update

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
)

// testBudget 放宽的检查/等待预算：生产预算（检查 1.5s / 提示 0.4s）在全量并行负载下
// 会被调度延迟吃掉，导致取版本超时或 wait() 提前放弃（抖动）
var testBudget = update.CheckOptions{CheckTimeout: 5 * time.Second, NoticeWait: 5 * time.Second}

func TestStartCheckPrintsNoticeForNewerRelease(t *testing.T) {
	t.Parallel()
	server, _ := newReleaseServer(t, "v9.9.9", nil, "")
	var out bytes.Buffer

	opts := testBudget
	opts.Current = "v0.1.0"
	opts.Out = &out
	opts.BaseURL = server.URL
	wait := update.StartCheck(t.Context(), opts)
	wait()

	if !strings.Contains(out.String(), "v9.9.9") || !strings.Contains(out.String(), "v0.1.0") {
		t.Fatalf("notice = %q, want latest and current version", out.String())
	}
}

func TestStartCheckIsSilentWhenUpToDate(t *testing.T) {
	t.Parallel()
	server, _ := newReleaseServer(t, "v0.2.2", nil, "")
	var out bytes.Buffer

	opts := testBudget
	opts.Current = "v0.2.2"
	opts.Out = &out
	opts.BaseURL = server.URL
	wait := update.StartCheck(t.Context(), opts)
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

	opts := testBudget
	opts.Current = "v0.1.0"
	opts.Out = &out
	opts.BaseURL = server.URL
	wait := update.StartCheck(t.Context(), opts)
	wait()

	if out.Len() != 0 {
		t.Fatalf("notice = %q, want empty output", out.String())
	}
}
