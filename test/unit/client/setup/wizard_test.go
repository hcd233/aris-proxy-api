package setup

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/client/setup"
)

func TestNormalizeHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "https ok", raw: "https://aris.example.com", want: "https://aris.example.com"},
		{name: "http ok", raw: "http://localhost:8080", want: "http://localhost:8080"},
		{name: "trim space and slash", raw: "  https://aris.example.com/  ", want: "https://aris.example.com"},
		{name: "missing scheme", raw: "aris.example.com", wantErr: true},
		{name: "empty", raw: "   ", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := setup.NormalizeHost(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeHost(%q) should error", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeHost(%q) error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeHost(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestResolveAPIKey(t *testing.T) {
	t.Parallel()
	if got := setup.ResolveAPIKey("new-key", "old-key"); got != "new-key" {
		t.Fatalf("input should win, got %q", got)
	}
	if got := setup.ResolveAPIKey("", "old-key"); got != "old-key" {
		t.Fatalf("empty input should keep existing, got %q", got)
	}
	if got := setup.ResolveAPIKey("", ""); got != "" {
		t.Fatalf("both empty should be empty, got %q", got)
	}
}
