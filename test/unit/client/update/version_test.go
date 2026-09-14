package update

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestIsNewer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{name: "patch bump", latest: "v0.2.2", current: "v0.2.1", expected: true},
		{name: "minor bump", latest: "v0.3.0", current: "v0.2.9", expected: true},
		{name: "double digit minor is not lexical", latest: "v0.10.0", current: "v0.9.9", expected: true},
		{name: "missing patch segment", latest: "v1", current: "v0.9.9", expected: true},
		{name: "equal versions", latest: "v0.2.2", current: "v0.2.2", expected: false},
		{name: "older latest", latest: "v0.2.1", current: "v0.2.2", expected: false},
		{name: "dev build", latest: "v0.2.2", current: constant.ArisClientDevVersion, expected: false},
		{name: "empty latest", latest: "", current: "v0.2.2", expected: false},
		{name: "prerelease not comparable", latest: "v0.3.0-rc.1", current: "v0.2.2", expected: false},
		{name: "non numeric segment", latest: "v0.3.x", current: "v0.2.2", expected: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := update.IsNewer(testCase.latest, testCase.current); got != testCase.expected {
				t.Fatalf("IsNewer(%q, %q) = %v, want %v", testCase.latest, testCase.current, got, testCase.expected)
			}
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{name: "equal versions", latest: "v0.2.2", current: "v0.2.2", expected: true},
		{name: "current ahead", latest: "v0.2.2", current: "v0.3.0", expected: true},
		{name: "current behind", latest: "v0.2.2", current: "v0.2.1", expected: false},
		{name: "dev build is never up to date", latest: "v0.2.2", current: constant.ArisClientDevVersion, expected: false},
		{name: "empty current", latest: "v0.2.2", current: "", expected: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := update.IsUpToDate(testCase.latest, testCase.current); got != testCase.expected {
				t.Fatalf("IsUpToDate(%q, %q) = %v, want %v", testCase.latest, testCase.current, got, testCase.expected)
			}
		})
	}
}

//nolint:paralleltest // t.Setenv 与 t.Parallel 互斥
func TestShouldCheck(t *testing.T) {
	cases := []struct {
		name        string
		current     string
		interactive bool
		env         string
		expected    bool
	}{
		{name: "release build on tty", current: "v0.2.2", interactive: true, expected: true},
		{name: "dev build", current: constant.ArisClientDevVersion, interactive: true, expected: false},
		{name: "empty version", current: "", interactive: true, expected: false},
		{name: "unparsable version", current: "v0.2.2-rc.1", interactive: true, expected: false},
		{name: "not interactive", current: "v0.2.2", interactive: false, expected: false},
		{name: "opt out with 1", current: "v0.2.2", interactive: true, env: "1", expected: false},
		{name: "opt out with true", current: "v0.2.2", interactive: true, env: "true", expected: false},
		{name: "opt out with yes", current: "v0.2.2", interactive: true, env: "YES", expected: false},
		{name: "opt in value with zero", current: "v0.2.2", interactive: true, env: "0", expected: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(constant.ArisClientNoUpdateCheckEnv, testCase.env)
			if got := update.ShouldCheck(testCase.current, testCase.interactive); got != testCase.expected {
				t.Fatalf("ShouldCheck(%q, %v) with %s=%q = %v, want %v",
					testCase.current, testCase.interactive, constant.ArisClientNoUpdateCheckEnv, testCase.env, got, testCase.expected)
			}
		})
	}
}

func TestShouldCheckCommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		commandPath []string
		expected    bool
	}{
		{name: "status participates", commandPath: []string{"status"}, expected: true},
		{name: "init participates", commandPath: []string{"init"}, expected: true},
		{name: "model export participates", commandPath: []string{"model", "export"}, expected: true},
		{name: "trace ingest skipped", commandPath: []string{constant.ArisClientCommandTrace, "ingest"}, expected: false},
		{name: "trace install skipped", commandPath: []string{constant.ArisClientCommandTrace, "install"}, expected: false},
		{name: "update skipped", commandPath: []string{constant.ArisClientCommandUpdate}, expected: false},
		{name: "version skipped", commandPath: []string{constant.ArisClientCommandVersion}, expected: false},
		{name: "empty path skipped", commandPath: nil, expected: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := update.ShouldCheckCommand(testCase.commandPath); got != testCase.expected {
				t.Fatalf("ShouldCheckCommand(%v) = %v, want %v", testCase.commandPath, got, testCase.expected)
			}
		})
	}
}
