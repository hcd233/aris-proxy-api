package router

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"mime"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

var commonMIME = map[string]string{
	".html":  "text/html",
	".js":    "application/javascript",
	".css":   "text/css",
	".woff2": "font/woff2",
	".woff":  "font/woff",
	".ico":   "image/x-icon",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".json":  "application/json",
	".txt":   "text/plain",
}

// RegisterWebRouter 注册前端静态资源路由
//
// dist 目录内的文件在构建时（make web-build）已 gzip 预压缩为 <原名>.gz。
// 支持 gzip 的客户端直接返回预压缩内容，否则实时解压回退。
//
//	@param app fiber.App
//	@param webFS fs.FS 嵌入的前端静态文件系统
//	@author centonhuang
//	@update 2026-08-04 17:30:00
func RegisterWebRouter(app *fiber.App, webFS fs.FS) {
	subFS, err := fs.Sub(webFS, "dist")
	if err != nil {
		logger.Logger().Error("failed to create sub filesystem from embedded dist/", zap.Error(err))
		return
	}

	indexContent, err := fs.ReadFile(subFS, "index.html.gz")
	if err != nil {
		logger.Logger().Error("failed to read index.html.gz from embedded dist/", zap.Error(err))
		return
	}

	app.Get("/web/*", func(c fiber.Ctx) error {
		path := c.Path()

		if path == "/web" {
			return c.Redirect().Status(fiber.StatusMovedPermanently).To("/web/")
		}

		filePath := strings.TrimPrefix(path, "/web")
		filePath = strings.TrimPrefix(filePath, "/")

		if filePath == "" || strings.HasSuffix(filePath, "/") {
			filePath = pathpkg.Join(filePath, "index.html")
		}

		data, err := fs.ReadFile(subFS, filePath+".gz")
		if err == nil {
			setStaticContentType(c, filepath.Ext(filePath))
			return sendPrecompressed(c, data)
		}

		ext := filepath.Ext(filePath)
		// 静态资源文件（如 js, css, 图片等）如果找不到，应直接返回 404，而不是 fallback 到 index.html
		// 这可以避免浏览器缓存错误的 200 HTML 响应（当作 JS/CSS 解析报错）导致页面彻底崩溃无法恢复
		if (ext != "" && ext != ".html") || strings.Contains(filePath, "_next/") || strings.Contains(filePath, "static/") {
			return c.Status(fiber.StatusNotFound).SendString("404 Not Found")
		}

		c.Set("Content-Type", "text/html; charset=utf-8")
		return sendPrecompressed(c, indexContent)
	})
}

func setStaticContentType(c fiber.Ctx, ext string) {
	if ct, ok := commonMIME[ext]; ok {
		c.Set("Content-Type", ct)
		return
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		c.Set("Content-Type", ct)
	}
}

// sendPrecompressed 发送已 gzip 压缩的内容，客户端不支持 gzip 时实时解压回退
func sendPrecompressed(c fiber.Ctx, gzData []byte) error {
	c.Set(fiber.HeaderVary, fiber.HeaderAcceptEncoding)
	if acceptsGzip(c) {
		c.Set(fiber.HeaderContentEncoding, "gzip")
		return c.Send(gzData)
	}

	zr, err := gzip.NewReader(bytes.NewReader(gzData))
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }() //nolint:errcheck // gzip reader close errors are best-effort

	raw, err := io.ReadAll(zr)
	if err != nil {
		return err
	}
	return c.Send(raw)
}

// acceptsGzip 显式检查客户端是否声明支持 gzip 编码（忽略 q=0）。
// 不能用 c.AcceptsEncodings：RFC 7231 语义下无 Accept-Encoding 头的请求被视为
// 接受任意编码，而裸 curl 等客户端收到 gzip 内容并不会自动解压
func acceptsGzip(c fiber.Ctx) bool {
	for offer := range strings.SplitSeq(c.Get(fiber.HeaderAcceptEncoding), ",") {
		token, params, _ := strings.Cut(strings.TrimSpace(offer), ";")
		if token != "gzip" {
			continue
		}
		q := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(params), "q="))
		if q == "0" || q == "0.0" || q == "0.00" || q == "0.000" {
			continue
		}
		return true
	}
	return false
}
