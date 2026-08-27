// Package client_models 验证客户端模型列表接口行为。
//
// 离线单测直连 handler（fake 端口），验证响应结构与字段裁剪；
// 在线 e2e 由 BASE_URL / API_KEY 环境变量门控。
//
// 本文件补充路由组装回归：客户端 SDK 请求的路径必须出现在服务端真实注册的
// 路由表里，否则 aris model export 会以 404 失败。
package client_models

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

// routeTestAPIKey 回归用例使用的 API Key 明文
const routeTestAPIKey = "sk-route-regression"

// RegisterAPIRouter 只为注册路由而持有各 handler 的方法值，方法本身不会被调用。
// 用「嵌入接口」的空结构体即可满足所有接口：nil 嵌入接口满足方法集，
// 注册期不会解引用，只有真正请求该路由才会 panic（本用例只请求模型分发路由）。
type (
	stubPingHandler      struct{ handler.PingHandler }
	stubTraceHandler     struct{ handler.TraceHandler }
	stubTokenHandler     struct{ handler.TokenHandler }
	stubOauth2Handler    struct{ handler.Oauth2Handler }
	stubUserHandler      struct{ handler.UserHandler }
	stubDemoHandler      struct{ handler.DemoHandler }
	stubAPIKeyHandler    struct{ handler.APIKeyHandler }
	stubSessionHandler   struct{ handler.SessionHandler }
	stubEndpointHandler  struct{ handler.EndpointHandler }
	stubModelHandler     struct{ handler.ModelHandler }
	stubUpstreamHandler  struct{ handler.UpstreamHandler }
	stubAuditHandler     struct{ handler.AuditHandler }
	stubCronHandler      struct{ handler.CronHandler }
	stubTriggerHandler   struct{ handler.TriggerHandler }
	stubOpenAIHandler    struct{ handler.OpenAIHandler }
	stubAnthropicHandler struct{ handler.AnthropicHandler }
	stubMetricsHandler   struct{ handler.MetricsHandler }
	stubDatasetHandler   struct{ handler.DatasetHandler }
)

// newRouteTestDB 建 sqlite 内存库并迁移 API Key 中间件所需表
func newRouteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.ProxyAPIKey{}, &dbmodel.User{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

// TestClientModelsRouteMatchesSDKPath 客户端 SDK 常量路径必须在服务端真实注册。
//
// 回归背景：commit d80f04d5 把客户端模型分发接口更名为 /api/v1/model/list，
// 只同步了 SDK 常量与组内路径，但 clientGroup 仍挂载在 /api/v1/client 下，
// 实际注册路径是 /api/v1/client/model/list，二者不一致导致 aris model export 报
// "list models rejected with status 404"。
//
// 本用例直接调用生产装配入口 router.RegisterAPIRouter（而非复刻分组），
// 因此 group 前缀的任何改动都会被捕获。断言 200 而非 401：预置可用 API Key，
// 这样 401 表示鉴权失败、404 表示路由缺失，二者可区分。
func TestClientModelsRouteMatchesSDKPath(t *testing.T) {
	t.Parallel()
	db := newRouteTestDB(t)
	user := dbmodel.User{Name: "route-tester"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&dbmodel.ProxyAPIKey{UserID: user.ID, Name: "k", Key: routeTestAPIKey}).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("client models route", "1.0"))
	router.RegisterAPIRouter(api, router.APIRouterDependencies{
		DB:               db,
		PingHandler:      &stubPingHandler{},
		TraceHandler:     &stubTraceHandler{},
		TokenHandler:     &stubTokenHandler{},
		Oauth2Handler:    &stubOauth2Handler{},
		UserHandler:      &stubUserHandler{},
		DemoHandler:      &stubDemoHandler{},
		APIKeyHandler:    &stubAPIKeyHandler{},
		SessionHandler:   &stubSessionHandler{},
		EndpointHandler:  &stubEndpointHandler{},
		ModelHandler:     &stubModelHandler{},
		UpstreamHandler:  &stubUpstreamHandler{},
		AuditHandler:     &stubAuditHandler{},
		CronHandler:      &stubCronHandler{},
		TriggerHandler:   &stubTriggerHandler{},
		OpenAIHandler:    &stubOpenAIHandler{},
		AnthropicHandler: &stubAnthropicHandler{},
		MetricsHandler:   &stubMetricsHandler{},
		DatasetHandler:   &stubDatasetHandler{},
		ClientHandler:    handler.NewClientHandler(handler.ClientDependencies{List: &fakeListClientModels{}}),
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, constant.ClientModelsListPath, http.NoBody)
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+routeTestAPIKey)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SDK path %q must be registered by RegisterAPIRouter, got status %d (404 = route missing)",
			constant.ClientModelsListPath, resp.StatusCode)
	}
}
