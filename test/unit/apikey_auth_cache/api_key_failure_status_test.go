package apikey_auth_cache

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// TestAPIKeyAuthFailureStatus 区分「Key 不存在」与「DB 不可用」：
// 前者 401（客户端问题），后者 503（服务端问题，SDK 应重试）。
func TestAPIKeyAuthFailureStatus(t *testing.T) {
	t.Parallel()

	if got := middleware.APIKeyAuthFailureStatus(gorm.ErrRecordNotFound); got != fiber.StatusUnauthorized {
		t.Errorf("missing key must map to 401, got %d", got)
	}
	dbErr := ierr.New(ierr.ErrDBQuery, "FATAL: sorry, too many clients already")
	if got := middleware.APIKeyAuthFailureStatus(dbErr); got != fiber.StatusServiceUnavailable {
		t.Errorf("db failure must map to 503, got %d", got)
	}
}
