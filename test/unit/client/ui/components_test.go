package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	clientui "github.com/hcd233/aris-proxy-api/internal/client/ui"
)

func init() {
	// 强制无色彩渲染，保证 golden 输出确定
	lipgloss.SetColorProfile(termenv.Ascii)
}

func TestStepHeader(t *testing.T) {
	t.Parallel()
	got := clientui.StepHeader(1, 4, "Connect to server")
	want := "[1/4] Connect to server"
	if got != want {
		t.Fatalf("StepHeader = %q, want %q", got, want)
	}
}

func TestSectionTitle(t *testing.T) {
	t.Parallel()
	got := clientui.SectionTitle("Server")
	want := "◆ Server"
	if got != want {
		t.Fatalf("SectionTitle = %q, want %q", got, want)
	}
}

func TestCheckRowWithDetail(t *testing.T) {
	t.Parallel()
	got := clientui.CheckRowOK("https://aris.example.com", "reachable (42ms)")
	want := "✓ https://aris.example.com · reachable (42ms)"
	if got != want {
		t.Fatalf("CheckRowOK = %q, want %q", got, want)
	}
}

func TestCheckRowWithoutDetail(t *testing.T) {
	t.Parallel()
	got := clientui.CheckRowFail("Server", "")
	want := "✗ Server"
	if got != want {
		t.Fatalf("CheckRowFail = %q, want %q", got, want)
	}
}

func TestCheckRowWarn(t *testing.T) {
	t.Parallel()
	got := clientui.CheckRowWarn("Hooks", "7/10 registered")
	want := "! Hooks · 7/10 registered"
	if got != want {
		t.Fatalf("CheckRowWarn = %q, want %q", got, want)
	}
}

func TestSummaryPanel(t *testing.T) {
	t.Parallel()
	got := clientui.SummaryPanel(
		"Trace configuration completed",
		"Config: /tmp/x",
	)
	want := "╭───────────────────────────────╮\n" +
		"│ Trace configuration completed │\n" +
		"│ Config: /tmp/x                │\n" +
		"╰───────────────────────────────╯"
	if got != want {
		t.Fatalf("SummaryPanel =\n%s\nwant:\n%s", got, want)
	}
}

func TestHuhThemeNotNil(t *testing.T) {
	t.Parallel()
	if clientui.HuhTheme() == nil {
		t.Fatal("HuhTheme returned nil")
	}
}
