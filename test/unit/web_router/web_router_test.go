package web_router

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
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
	router.RegisterWebRouter(app, testWebFS(t))

	rsp := doRequest(t, app, "/web", "")
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
	webFS := &trackingFS{base: testWebFS(t)}
	app := fiber.New()
	router.RegisterWebRouter(app, webFS)

	// Registration reads index.html.gz once
	if len(webFS.opened) != 1 || webFS.opened[0] != "dist/index.html.gz" {
		t.Fatalf("after registration opened %v, want [dist/index.html.gz]", webFS.opened)
	}

	rsp := doRequest(t, app, "/web", "")
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("GET /web status = %d, want %d", rsp.StatusCode, http.StatusMovedPermanently)
	}
	// No additional opens during the redirect request
	if len(webFS.opened) != 1 {
		t.Fatalf("after redirect request opened %v, want still [dist/index.html.gz]", webFS.opened)
	}
}

func TestRegisterWebRouter_FallbackAndStaticNotFound(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	router.RegisterWebRouter(app, testWebFS(t))

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
			rsp := doRequest(t, app, tc.path, "")
			defer rsp.Body.Close()
			if rsp.StatusCode != tc.wantStatus {
				t.Fatalf("GET %s status = %d, want %d", tc.path, rsp.StatusCode, tc.wantStatus)
			}
		})
	}
}

// TestRegisterWebRouter_FallsBackToRawAssets 回归用例：构建产物未 gzip 预压缩（.gz 缺失）时，
// 路由仍须正常服务——直接读原始文件发送，不得导致 /web/* 全部 404。
//
// 背景：CI 曾遗漏 gzip 步骤，线上 embed 只有 index.html 无 index.html.gz，
// 旧实现读 index.html.gz 失败后不注册路由，整个 Web 控制台 404。
//
//	@author centonhuang
//	@update 2026-08-05 01:30:00
func TestRegisterWebRouter_FallsBackToRawAssets(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	router.RegisterWebRouter(app, fstest.MapFS{
		"dist/index.html": {
			Data: []byte("dashboard-raw"),
		},
		"dist/_next/static/chunks/app.js": {
			Data: []byte("console.log('raw')"),
		},
	})

	cases := []struct {
		name     string
		path     string
		wantBody string
	}{
		{name: "index fallback", path: "/web/", wantBody: "dashboard-raw"},
		{name: "client route fallback", path: "/web/sessions", wantBody: "dashboard-raw"},
		{name: "raw static asset", path: "/web/_next/static/chunks/app.js", wantBody: "console.log('raw')"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rsp := doRequest(t, app, tc.path, "")
			defer rsp.Body.Close()
			if rsp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tc.path, rsp.StatusCode, http.StatusOK)
			}
			if enc := rsp.Header.Get("Content-Encoding"); enc != "" {
				t.Fatalf("GET %s Content-Encoding = %q, want empty (raw asset)", tc.path, enc)
			}
			body, err := io.ReadAll(rsp.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if string(body) != tc.wantBody {
				t.Fatalf("GET %s body = %q, want %q", tc.path, string(body), tc.wantBody)
			}
		})
	}
}

func TestRegisterWebRouter_ServesPrecompressedByNegotiation(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	router.RegisterWebRouter(app, testWebFS(t))

	cases := []struct {
		name           string
		path           string
		acceptEncoding string
		wantEncoding   string
	}{
		{name: "gzip client gets precompressed index", path: "/web/", acceptEncoding: "gzip", wantEncoding: "gzip"},
		{name: "gzip client gets precompressed asset", path: "/web/_next/static/chunks/app.js", acceptEncoding: "gzip", wantEncoding: "gzip"},
		{name: "plain client gets decompressed index", path: "/web/", acceptEncoding: "", wantEncoding: ""},
		{name: "plain client gets decompressed asset", path: "/web/_next/static/chunks/app.js", acceptEncoding: "", wantEncoding: ""},
		{name: "br-only client gets decompressed body", path: "/web/", acceptEncoding: "br", wantEncoding: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rsp := doRequest(t, app, tc.path, tc.acceptEncoding)
			defer rsp.Body.Close()

			if rsp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tc.path, rsp.StatusCode, http.StatusOK)
			}
			if encoding := rsp.Header.Get("Content-Encoding"); encoding != tc.wantEncoding {
				t.Fatalf("GET %s Content-Encoding = %q, want %q", tc.path, encoding, tc.wantEncoding)
			}
			if vary := rsp.Header.Get("Vary"); vary != "Accept-Encoding" {
				t.Fatalf("GET %s Vary = %q, want %q", tc.path, vary, "Accept-Encoding")
			}

			body, err := io.ReadAll(rsp.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if tc.wantEncoding == "gzip" {
				zr, err := gzip.NewReader(bytes.NewReader(body))
				if err != nil {
					t.Fatalf("gzip.NewReader() error = %v", err)
				}
				body, err = io.ReadAll(zr)
				if err != nil {
					t.Fatalf("ReadAll(gzip) error = %v", err)
				}
				if err := zr.Close(); err != nil {
					t.Fatalf("gzip.Close() error = %v", err)
				}
			}

			wantBody := "dashboard"
			if tc.path == "/web/_next/static/chunks/app.js" {
				wantBody = "console.log('ok')"
			}
			if string(body) != wantBody {
				t.Fatalf("GET %s body = %q, want %q", tc.path, string(body), wantBody)
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

func testWebFS(t *testing.T) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		"dist/index.html.gz": {
			Data: gzipData(t, []byte("dashboard")),
		},
		"dist/_next/static/chunks/app.js.gz": {
			Data: gzipData(t, []byte("console.log('ok')")),
		},
	}
}

func gzipData(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		t.Fatalf("gzip.Write() error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip.Close() error = %v", err)
	}
	return buf.Bytes()
}

func doRequest(t *testing.T, app *fiber.App, path, acceptEncoding string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rsp, err := app.Test(req)
	if err != nil {
		t.Fatalf("App.Test(%s) error = %v", path, err)
	}
	return rsp
}
