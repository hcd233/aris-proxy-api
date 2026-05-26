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
		filePath := strings.TrimPrefix(c.Path(), "/web/")
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

		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(indexContent)
	})
}
