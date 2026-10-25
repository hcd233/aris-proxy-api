package status

import (
	"bytes"
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
		ServerOK:      true,
		ServerLatency: 42 * time.Millisecond,
		AuthOK:        true,
		AuthMaskedKey: "••••5678",
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
◆ Providers
  ! agent configs · none configured — run aris model export
`
	if out.String() != want {
		t.Fatalf("Render =\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestRenderNotInitialized(t *testing.T) {
	t.Parallel()
	report := &status.Report{}
	var out bytes.Buffer
	if err := status.Render(&out, report); err != nil {
		t.Fatal(err)
	}
	want := `aris status

◆ Server
  ! not initialized · run ` + "`aris init`" + ` to configure
◆ Auth
  ! not configured
◆ Providers
  ! agent configs · none configured — run aris model export
`
	if out.String() != want {
		t.Fatalf("Render =\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestRenderJSONSchema(t *testing.T) {
	t.Parallel()
	report := &status.Report{
		ConfigFound:      true,
		Host:             "https://aris.example.com",
		ServerOK:         true,
		ServerLatency:    42 * time.Millisecond,
		AuthOK:           true,
		AuthMaskedKey:    "••••5678",
		ProvidersFound:   []string{"opencode"},
		ProvidersMissing: []string{"codex"},
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
		"configFound", "host", "serverOk", "serverLatencyMs",
		"authOk", "authMaskedKey", "providersFound", "providersMissing",
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
