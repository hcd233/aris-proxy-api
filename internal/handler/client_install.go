package handler

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"text/template"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

//go:embed install_aris_client.sh.tmpl
var installScriptTemplate string

var installScriptTmpl = template.Must(template.New(constant.ArisClientInstallScriptTmplName).Parse(installScriptTemplate))

type installScriptData struct {
	Host string
}

// writeInstallScriptError 返回一个将错误输出到 stderr 并退出的 bash 脚本。
// 用于 curl|bash 模式下服务端出错时给出可读提示，避免 bash 把 JSON 当命令执行。
func writeInstallScriptError(humaCtx huma.Context, message string) {
	humaCtx.SetStatus(fiber.StatusOK)
	humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeTextPlain)
	humaCtx.SetHeader(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoStore)
	if _, writeErr := io.WriteString(humaCtx.BodyWriter(), "#!/usr/bin/env bash\necho '"+message+"' >&2\nexit 1\n"); writeErr != nil {
		logger.WithCtx(humaCtx.Context()).Warn(
			"[ClientHandler] Failed to write install error script",
			zap.Error(writeErr),
		)
	}
}

// HandleInstallScript 返回自包含的安装脚本（无票据，host 从请求头推导）。
func (h *clientHandler) HandleInstallScript(
	_ context.Context,
	_ *dto.InstallScriptReq,
) (*huma.StreamResponse, error) {
	return &huma.StreamResponse{Body: func(humaCtx huma.Context) {
		scheme := humaCtx.Header(constant.HTTPHeaderXForwardedProto)
		host := humaCtx.Header(constant.HTTPHeaderHost)

		// scheme 严格白名单 + 原始 Host 头校验 + 只嵌入已验证分量（防 shell 注入）。
		origin, ok := util.SafeInstallOrigin(scheme, host)
		if !ok {
			logger.WithCtx(humaCtx.Context()).Warn(
				"[ClientHandler] Invalid origin for install script",
				zap.String("origin", scheme+"://"+host),
			)
			writeInstallScriptError(humaCtx, constant.ArisClientInstallOriginErrorMessage)
			return
		}

		var buf bytes.Buffer
		if err := installScriptTmpl.Execute(&buf, installScriptData{Host: origin}); err != nil {
			logger.WithCtx(humaCtx.Context()).Warn(
				"[ClientHandler] Failed to execute install script template",
				zap.Error(err),
			)
			writeInstallScriptError(humaCtx, constant.ArisClientInstallGenErrorMessage)
			return
		}

		humaCtx.SetStatus(fiber.StatusOK)
		humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeTextPlain)
		humaCtx.SetHeader(constant.HTTPHeaderCacheControl, constant.HTTPCacheControlNoStore)
		if _, writeErr := io.WriteString(humaCtx.BodyWriter(), buf.String()); writeErr != nil {
			logger.WithCtx(humaCtx.Context()).Warn(
				"[ClientHandler] Failed to write install script",
				zap.Error(writeErr),
			)
		}
	}}, nil
}
