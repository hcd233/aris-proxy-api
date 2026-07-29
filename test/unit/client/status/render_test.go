package status

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/hcd233/aris-proxy-api/internal/client/status"
)

func init() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

func TestRenderHealthyReport(t *testing.T) {
	t.Parallel()
	report := &status.Report{
		ConfigFound:   true,
		Host:          "https://aris.example.com",
		Agent:         "codex",
		ServerOK:      true,
		ServerLatency: 42 * time.Millisecond,
		AuthOK:        true,
		AuthMaskedKey: "••••5678",
		HooksFound:    10,
		HooksTotal:    10,
		LogDir:        "/home/u/.aris/trace/logs",
	}
	var out bytes.Buffer
	if err := status.Render(&out, report); err != nil {
		t.Fatal(err)
	}
	want := `aris status

◆ Server
  ✓ https://aris.example.com · reachable (42ms)
◆ Auth
  ✓ API key valid · ••••5678
◆ Agent
  ✓ codex · hooks 10/10 registered
◆ Local queue
  ✓ no pending records
◆ Diagnostics
  ✓ no recent errors
`
	if out.String() != want {
		t.Fatalf("Render =\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestRenderNotInitialized(t *testing.T) {
	t.Parallel()
	report := &status.Report{
		HooksTotal:   10,
		HooksMissing: []string{"SessionStart", "PostCompact"},
		LogDir:       "/home/u/.aris/trace/logs",
	}
	var out bytes.Buffer
	if err := status.Render(&out, report); err != nil {
		t.Fatal(err)
	}
	want := `aris status

◆ Server
  ! not initialized · run ` + "`aris init`" + ` to configure
◆ Auth
  ! not configured
◆ Agent
  ! codex · hooks 0/10 registered, missing: SessionStart, PostCompact
◆ Local queue
  ✓ no pending records
◆ Diagnostics
  ✓ no recent errors
`
	if out.String() != want {
		t.Fatalf("Render =\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestRenderQueueBacklog(t *testing.T) {
	t.Parallel()
	report := &status.Report{
		ConfigFound:   true,
		Host:          "https://aris.example.com",
		Agent:         "codex",
		ServerOK:      true,
		ServerLatency: 42 * time.Millisecond,
		AuthOK:        true,
		AuthMaskedKey: "••••5678",
		HooksFound:    10,
		HooksTotal:    10,
		PendingCount:  3,
		PendingBytes:  12700,
		RejectedCount: 1,
		RecentErrors:  2,
		LogDir:        "/home/u/.aris/trace/logs",
	}
	var out bytes.Buffer
	if err := status.Render(&out, report); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, fragment := range []string{
		"! 3 pending (12.4 KB) · 1 rejected",
		"! 2 recent error(s) · see /home/u/.aris/trace/logs",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("Render missing %q:\n%s", fragment, text)
		}
	}
}

func TestRenderJSONSchema(t *testing.T) {
	t.Parallel()
	report := &status.Report{
		ConfigFound:   true,
		Host:          "https://aris.example.com",
		Agent:         "codex",
		ServerOK:      true,
		ServerLatency: 42 * time.Millisecond,
		AuthOK:        true,
		AuthMaskedKey: "••••5678",
		HooksFound:    7,
		HooksTotal:    10,
		HooksMissing:  []string{"Stop"},
		PendingCount:  3,
		PendingBytes:  12700,
		RejectedCount: 1,
		RecentErrors:  2,
	}
	var out bytes.Buffer
	if err := status.RenderJSON(&out, report); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("RenderJSON output is not valid JSON: %v", err)
	}
	for _, key := range []string{
		"configFound", "host", "agent", "serverOk", "serverLatencyMs",
		"authOk", "authMaskedKey", "hooksFound", "hooksTotal", "hooksMissing",
		"pendingCount", "pendingBytes", "rejectedCount", "recentErrors",
	} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("RenderJSON missing key %q:\n%s", key, out.String())
		}
	}
	var latency int64
	if err := sonic.Unmarshal(decoded["serverLatencyMs"], &latency); err != nil || latency != 42 {
		t.Fatalf("serverLatencyMs = %s, want 42", decoded["serverLatencyMs"])
	}
}
