package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/i18n"
)

func LocaleMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		locale := i18n.DetectLocale(c.Get(constant.HTTPHeaderAcceptLanguage))
		c.Locals(constant.CtxKeyLocale, locale)
		return c.Next()
	}
}
