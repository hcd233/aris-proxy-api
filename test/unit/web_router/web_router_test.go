package web_router

import (
	"io"
	"io/fs"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

func TestRegisterWebRouter_ServesWebWithoutRedirect(t *testing.T) {
	app := fiber.New()
	router.RegisterWebRouter(app, testWebFS())

	rsp := doRequest(t, app, "/web")
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		t.Fatalf("GET /web status = %d, want %d", rsp.StatusCode, http.StatusOK)
	}
	if location := rsp.Header.Get("Location"); location != "" {
		t.Fatalf("GET /web Location = %q, want empty", location)
	}
	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "dashboard" {
		t.Fatalf("GET /web body = %q, want %q", string(body), "dashboard")
	}
}

func TestRegisterWebRouter_MapsWebPathToIndex(t *testing.T) {
	webFS := &trackingFS{base: testWebFS()}
	app := fiber.New()
	router.RegisterWebRouter(app, webFS)

	rsp := doRequest(t, app, "/web")
	defer rsp.Body.Close()

	if webFS.openCount("dist/web") != 0 {
		t.Fatalf("GET /web opened dist/web, want dist/index.html")
	}
	if webFS.openCount("dist/index.html") < 2 {
		t.Fatalf("GET /web opened dist/index.html %d times, want at least 2", webFS.openCount("dist/index.html"))
	}
}

func TestRegisterWebRouter_FallbackAndStaticNotFound(t *testing.T) {
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

func (tfs *trackingFS) openCount(name string) int {
	count := 0
	for _, opened := range tfs.opened {
		if opened == name {
			count++
		}
	}
	return count
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
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	rsp, err := app.Test(req)
	if err != nil {
		t.Fatalf("App.Test(%s) error = %v", path, err)
	}
	return rsp
}
