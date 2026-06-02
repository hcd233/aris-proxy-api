package router

import (
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

func RegisterWebRouter(app *fiber.App, webFS fs.FS) {
	subFS, err := fs.Sub(webFS, "dist")
	if err != nil {
		logger.Logger().Error("failed to create sub filesystem from embedded dist/", zap.Error(err))
		return
	}

	indexContent, err := fs.ReadFile(subFS, "index.html")
	if err != nil {
		logger.Logger().Error("failed to read index.html from embedded dist/", zap.Error(err))
		return
	}

	app.Get("/web/*", func(c fiber.Ctx) error {
		filePath := strings.TrimPrefix(c.Path(), "/web")
		filePath = strings.TrimPrefix(filePath, "/")

		if filePath == "" || strings.HasSuffix(filePath, "/") {
			filePath = pathpkg.Join(filePath, "index.html")
		}

		data, err := fs.ReadFile(subFS, filePath)
		if err == nil {
			ext := filepath.Ext(filePath)
			if ct, ok := commonMIME[ext]; ok {
				c.Set("Content-Type", ct)
			} else if ct := mime.TypeByExtension(ext); ct != "" {
				c.Set("Content-Type", ct)
			}
			return c.Send(data)
		}

		ext := filepath.Ext(filePath)
		// 静态资源文件（如 js, css, 图片等）如果找不到，应直接返回 404，而不是 fallback 到 index.html
		// 这可以避免浏览器缓存错误的 200 HTML 响应（当作 JS/CSS 解析报错）导致页面彻底崩溃无法恢复
		if (ext != "" && ext != ".html") || strings.Contains(filePath, "_next/") || strings.Contains(filePath, "static/") {
			return c.Status(fiber.StatusNotFound).SendString("404 Not Found")
		}

		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(indexContent)
	})
}
