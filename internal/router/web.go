package router

import (
	"io/fs"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

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

	app.Use("/web", static.New("", static.Config{
		FS:         subFS,
		IndexNames: []string{"index.html"},
	}))

	app.Get("/web/*", func(c fiber.Ctx) error {
		c.Type("html", "utf-8")
		return c.Send(indexContent)
	})
}
