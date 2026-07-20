package trace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/traceclient"
)

func TestTraceClientArtifactResolver_AllowsOnlySupportedTargets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := []string{
		"aris-darwin-amd64",
		"aris-darwin-arm64",
		"aris-linux-amd64",
		"aris-linux-arm64",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	resolver := traceclient.NewArtifactResolver(dir)

	for _, tc := range []struct {
		osName string
		arch   string
		ok     bool
	}{
		{osName: "darwin", arch: "amd64", ok: true},
		{osName: "darwin", arch: "arm64", ok: true},
		{osName: "linux", arch: "amd64", ok: true},
		{osName: "linux", arch: "arm64", ok: true},
		{osName: "../../etc", arch: "passwd", ok: false},
		{osName: "windows", arch: "amd64", ok: false},
	} {
		t.Run(tc.osName+"/"+tc.arch, func(t *testing.T) {
			t.Parallel()
			artifact, err := resolver.Resolve(tc.osName, tc.arch)
			if tc.ok && (err != nil || artifact.Size == 0 || artifact.Filename != "aris") {
				t.Fatalf("resolve = %+v, %v", artifact, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("unsupported target resolved to %+v", artifact)
			}
		})
	}
}
