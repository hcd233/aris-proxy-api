package i18n

import (
	"context"
	"embed"
	"strings"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"go.uber.org/zap"
)

//go:embed locales/*.json
var localeFiles embed.FS

var (
	translations = make(map[enum.Locale]map[string]string)
	loadOnce     sync.Once
	loadErr      error
)

func loadLocales() {
	entries, err := localeFiles.ReadDir(constant.LocaleEmbedDir)
	if err != nil {
		loadErr = err
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), constant.LocaleFileExt) {
			continue
		}
		locale := enum.Locale(strings.TrimSuffix(entry.Name(), constant.LocaleFileExt))
		data, err := localeFiles.ReadFile(constant.LocaleEmbedDir + "/" + entry.Name())
		if err != nil {
			loadErr = err
			return
		}
		var m map[string]string
		if err := sonic.Unmarshal(data, &m); err != nil {
			loadErr = err
			return
		}
		translations[locale] = m
	}
}

func init() {
	loadOnce.Do(loadLocales)
}

func DetectLocale(acceptLanguage string) enum.Locale {
	raw := strings.TrimSpace(acceptLanguage)
	if raw == "" {
		return enum.LocaleEN
	}
	parts := strings.SplitN(raw, ",", 2)
	primary := strings.TrimSpace(parts[0])
	if idx := strings.IndexAny(primary, constant.LocaleAcceptSeparator); idx > 0 {
		primary = primary[:idx]
	}
	primary = strings.ToLower(primary)
	switch primary {
	case constant.LocalePrimaryZH:
		return enum.LocaleZH
	case constant.LocalePrimaryJA:
		return enum.LocaleJA
	default:
		return enum.LocaleEN
	}
}

func Translate(locale enum.Locale, key, fallback string) string {
	if m, ok := translations[locale]; ok {
		if msg, ok := m[key]; ok {
			return msg
		}
	}
	if m, ok := translations[enum.LocaleEN]; ok {
		if msg, ok := m[key]; ok {
			return msg
		}
	}
	return fallback
}

func WithLocale(ctx context.Context, locale enum.Locale) context.Context {
	return context.WithValue(ctx, constant.CtxKeyLocale, locale)
}

func FromCtx(ctx context.Context) enum.Locale {
	if v, ok := ctx.Value(constant.CtxKeyLocale).(enum.Locale); ok {
		return v
	}
	return enum.LocaleEN
}

func LoadErr() error {
	return loadErr
}

var L = zap.L()
