package web_router

import (
	"context"
	"io/fs"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

func TestRegisterWebRouter_RedirectsWebRootToTrailingSlash(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	router.RegisterWebRouter(app, testWebFS())

	rsp := doRequest(t, app, "/web")
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("GET /web status = %d, want %d", rsp.StatusCode, http.StatusMovedPermanently)
	}
	if location := rsp.Header.Get("Location"); location != "/web/" {
		t.Fatalf("GET /web Location = %q, want %q", location, "/web/")
	}
}

func TestRegisterWebRouter_RedirectsWithoutOpeningFiles(t *testing.T) {
	t.Parallel()
	webFS := &trackingFS{base: testWebFS()}
	app := fiber.New()
	router.RegisterWebRouter(app, webFS)

	// Registration reads index.html once
	if len(webFS.opened) != 1 || webFS.opened[0] != "dist/index.html" {
		t.Fatalf("after registration opened %v, want [dist/index.html]", webFS.opened)
	}

	rsp := doRequest(t, app, "/web")
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("GET /web status = %d, want %d", rsp.StatusCode, http.StatusMovedPermanently)
	}
	// No additional opens during the redirect request
	if len(webFS.opened) != 1 {
		t.Fatalf("after redirect request opened %v, want still [dist/index.html]", webFS.opened)
	}
}

func TestRegisterWebRouter_FallbackAndStaticNotFound(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	router.RegisterWebRouter(app, testWebFS())

	cases := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "slash web root", path: "/web/", wantStatus: http.StatusOK},
		{name: "client route fallback", path: "/web/sessions", wantStatus: http.StatusOK},
		{name: "missing static asset", path: "/web/_next/static/chunks/missing.js", wantStatus: http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rsp := doRequest(t, app, tc.path)
			defer rsp.Body.Close()
			if rsp.StatusCode != tc.wantStatus {
				t.Fatalf("GET %s status = %d, want %d", tc.path, rsp.StatusCode, tc.wantStatus)
			}
		})
	}
}

type trackingFS struct {
	base   fstest.MapFS
	opened []string
}

func (tfs *trackingFS) Open(name string) (fs.File, error) {
	tfs.opened = append(tfs.opened, name)
	return tfs.base.Open(name)
}

func testWebFS() fstest.MapFS {
	return fstest.MapFS{
		"dist/index.html": {
			Data: []byte("dashboard"),
		},
		"dist/_next/static/chunks/app.js": {
			Data: []byte("console.log('ok')"),
		},
	}
}

func doRequest(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	rsp, err := app.Test(req)
	if err != nil {
		t.Fatalf("App.Test(%s) error = %v", path, err)
	}
	return rsp
}
