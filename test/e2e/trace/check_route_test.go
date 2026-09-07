package trace_e2e

import (
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

// stubCheckClientHandler 嵌入接口空结构体：注册期满足方法集、不解引用（cross_tenant 惯例）。
type stubCheckClientHandler struct{ handler.ClientHandler }

// TestCheckRoute_LegacyPathServesOldBinaries check 路由新旧双路径回归。
//
// #175 将 /api/cli/v1/trace/client/check 更名为 /aris/client/check（破坏性），
// 旧路径保留为 deprecated 兼容路由——存量客户端二进制的 init/status 硬编码旧路径，
// 404 会让其误判服务不可用；新路径为唯一推荐路径。
func TestCheckRoute_LegacyPathServesOldBinaries(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.User{}, &dbmodel.ProxyAPIKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// github/google_bind_id 各自参与 (bind_id, deleted_at) 唯一索引，须逐个区分
	user := &dbmodel.User{
		Name: "check-e2e", Permission: enum.PermissionUser,
		GithubBindID: "gh-check-e2e", GoogleBindID: "gg-check-e2e",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&dbmodel.ProxyAPIKey{UserID: user.ID, Name: "k", Key: "sk-check-route-e2e"}).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}

	app := fiber.New()
	humaAPI := humafiber.New(app, huma.DefaultConfig("check-route", "1.0"))
	router.RegisterCLIAPIRoutes(huma.NewGroup(humaAPI, constant.CLIAPIPrefix), router.APIRouterDependencies{
		DB:            db,
		ClientHandler: &stubCheckClientHandler{},
		TraceHandler:  handler.NewTraceHandler(handler.TraceDependencies{}),
	})

	paths := []string{
		constant.CLIAPIPrefix + constant.ArisClientCheckRoutePath,
		constant.CLIAPIPrefix + constant.TraceClientCheckLegacyRoutePath,
	}
	for _, path := range paths {
		// 有效 key：204（check 只校验 API Key，无响应体）
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://e2e.local"+path, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+"sk-check-route-e2e")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("GET %s with valid key: status = %d, want 204", path, resp.StatusCode)
		}

		// 无 key：401（路由存在且鉴权生效；404 意味着路由缺失）
		req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, "http://e2e.local"+path, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err = app.Test(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("GET %s without key: status = %d, want 401", path, resp.StatusCode)
		}
	}
}
