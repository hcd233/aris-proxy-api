package lintconv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintconv"
)

type fixtureCase struct {
	Name    string            `json:"name"`
	Files   []fixtureFile     `json:"files"`
	Want    []fixtureExpected `json:"want"`
	NotWant []fixtureExpected `json:"notWant"`
}

type fixtureFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type fixtureExpected struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
}

func TestRunReportsExpectedDiagnostics(t *testing.T) {
	cases := loadFixtureCases(t)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			root := t.TempDir()
			writeModule(t, root)
			for _, file := range tc.Files {
				writeFixtureFile(t, root, file)
			}

			result := lintconv.Run([]string{root})
			for _, want := range tc.Want {
				if !hasDiagnostic(result.Diagnostics, want.Rule, enum.Severity(want.Severity)) {
					t.Fatalf("missing diagnostic rule=%s severity=%s in %#v", want.Rule, want.Severity, result.Diagnostics)
				}
			}
			for _, nw := range tc.NotWant {
				if hasDiagnostic(result.Diagnostics, nw.Rule, enum.Severity(nw.Severity)) {
					t.Fatalf("unexpected diagnostic rule=%s severity=%s in %#v", nw.Rule, nw.Severity, result.Diagnostics)
				}
			}
		})
	}
}

func TestResultCountsBySeverity(t *testing.T) {
	result := lintconv.Result{Diagnostics: []lintconv.Diagnostic{
		{Rule: "a", Severity: enum.SeverityError},
		{Rule: "b", Severity: enum.SeverityWarning},
		{Rule: "c", Severity: enum.SeverityWarning},
	}}
	if result.ErrorCount() != 1 {
		t.Fatalf("ErrorCount() = %d, want 1", result.ErrorCount())
	}
	if result.WarningCount() != 2 {
		t.Fatalf("WarningCount() = %d, want 2", result.WarningCount())
	}
}

func loadFixtureCases(t *testing.T) []fixtureCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("fixtures", "cases.json"))
	if err != nil {
		t.Fatalf("read cases fixture: %v", err)
	}
	var cases []fixtureCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("unmarshal cases fixture: %v", err)
	}
	return cases
}

func writeModule(t *testing.T, root string) {
	t.Helper()
	content := "module github.com/hcd233/aris-proxy-api\n\ngo 1.25.1\n\nrequire go.uber.org/zap v1.27.0\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(content), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}

func writeFixtureFile(t *testing.T, root string, file fixtureFile) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(file.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(file.Content), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
}

func hasDiagnostic(diagnostics []lintconv.Diagnostic, rule string, severity enum.Severity) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Rule == rule && diagnostic.Severity == severity {
			return true
		}
	}
	return false
}
