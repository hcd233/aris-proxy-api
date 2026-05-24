package router

import (
	"io/fs"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func RegisterWebRouter(app *fiber.App, webFS fs.FS) {
	subFS, _ := fs.Sub(webFS, "dist")

	app.Use("/web", static.New("", static.Config{
		FS:         subFS,
		IndexNames: []string{"index.html"},
	}))

	app.Use("/web/*", static.New("", static.Config{
		FS:         subFS,
		IndexNames: []string{"index.html"},
	}))
}
