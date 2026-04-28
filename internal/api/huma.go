package api

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
)

// NewHumaAPI 创建 Huma API 实例
//
//	@param app *fiber.App
//	@return huma.API
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func NewHumaAPI(app *fiber.App) huma.API {
	return humafiber.New(app, huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI: constant.OpenAPIVersion,
			Info: &huma.Info{
				Title:       constant.APITitle,
				Description: constant.APIDescription,
				Version:     constant.APIVersion,
				Contact: &huma.Contact{
					Name:  constant.ContactName,
					Email: constant.ContactEmail,
					URL:   constant.ContactURL,
				},
				License: &huma.License{
					Name: constant.LicenseName,
					URL:  constant.LicenseURL,
				},
			},
			Components: &huma.Components{
				Schemas: huma.NewMapRegistry(constant.OpenAPISchemasPrefix, huma.DefaultSchemaNamer),
				SecuritySchemes: map[string]*huma.SecurityScheme{
					constant.SecuritySchemeJWT: {
						Type:        constant.SecurityTypeAPIKey,
						Name:        constant.HeaderAuthorization,
						In:          constant.SecurityInHeader,
						Description: constant.JWTDescription,
					},
					constant.SecuritySchemeAPIKey: {
						Type:        constant.SecurityTypeHTTP,
						Scheme:      constant.SecuritySchemeBearer,
						Description: constant.APIKeyDescription,
					},
				},
			},
		},
		OpenAPIPath:   lo.If(config.Env != enum.EnvProduction, constant.OpenAPIDocsPath).Else(""),
		DocsPath:      "",
		SchemasPath:   lo.If(config.Env != enum.EnvProduction, constant.OpenAPISchemasPath).Else(""),
		Formats:       huma.DefaultFormats,
		DefaultFormat: constant.DefaultFormatJSON,
	})
}
