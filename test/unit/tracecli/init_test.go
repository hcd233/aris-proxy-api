package tracecli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	client "github.com/hcd233/aris-proxy-api/internal/tracecli"
)

type fakeTerminal struct {
	interactive bool
	lines       []string
	secrets     []string
	output      []string
}

func (f *fakeTerminal) Interactive() bool { return f.interactive }
func (f *fakeTerminal) ReadLine(_ string) (string, error) {
	line := f.lines[0]
	f.lines = f.lines[1:]
	return line, nil
}
func (f *fakeTerminal) ReadSecret(_ string) (string, error) {
	secret := f.secrets[0]
	f.secrets = f.secrets[1:]
	return secret, nil
}
func (f *fakeTerminal) WriteLine(line string) { f.output = append(f.output, line) }

type fakeCodexInstaller struct {
	commandPath string
}

func (f *fakeCodexInstaller) Install(_ context.Context, commandPath string) (string, error) {
	f.commandPath = commandPath
	return "/tmp/hooks.json.bak", nil
}

func TestInitRunner_CompletesFourSteps(t *testing.T) {
	t.Parallel()
	order := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			order = append(order, "health")
			w.WriteHeader(http.StatusOK)
		case "/api/v1/trace/client/check":
			order = append(order, "key")
			if r.Header.Get("Authorization") != "Bearer proxy-key" {
				t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	terminal := &fakeTerminal{interactive: true, lines: []string{""}, secrets: []string{"proxy-key"}}
	installer := &fakeCodexInstaller{}
	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	runner := client.InitRunner{
		Terminal: terminal,
		Config:   client.NewConfigStore(paths),
		Codex:    installer,
		HTTP:     client.NewHTTPClient(server.Client()),
	}
	if err := runner.Run(context.Background(), client.InitOptions{
		Host:        server.URL + "/",
		CommandPath: "/home/user/.aris/bin/aris",
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "health,key" {
		t.Fatalf("request order = %v", order)
	}
	if installer.commandPath != "/home/user/.aris/bin/aris" {
		t.Fatalf("command path = %q", installer.commandPath)
	}
	config, err := client.NewConfigStore(paths).Load(context.Background())
	if err != nil || config.Host != server.URL || config.Agent != "codex" || config.APIKey != "proxy-key" {
		t.Fatalf("config = %+v, %v", config, err)
	}
	output := strings.Join(terminal.output, "\n")
	for _, want := range []string{"[1/4]", "[2/4]", "[3/4]", "[4/4]", "/hooks"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "proxy-key") {
		t.Fatal("output leaked API key")
	}
}

func TestInitRunner_RejectsNonInteractiveTerminal(t *testing.T) {
	t.Parallel()
	runner := client.InitRunner{Terminal: &fakeTerminal{}}
	if err := runner.Run(context.Background(), client.InitOptions{Host: "https://example.com"}); err == nil {
		t.Fatal("expected non-interactive init to fail")
	}
}
