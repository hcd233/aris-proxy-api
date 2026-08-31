// client_sdk_e2e_test.go 真实 SDK ↔ 生产路由装配的贯通验证（CR R2）。
//
// client_route_test.go 用常量直接拼请求 URL，若 SDK 内部拼路径与常量脱节则抓不到；
// 本文件用 internal/client/api 的真实 Client（与 aris 二进制同一条拼路径代码）
// 经生产 RegisterAPIRouter 装配的 fiber app 发起请求，任何一侧路径漂移都会以
// 404 暴露——这是 client models 链路（本周修复 5 次）的最终贯通守护。
package client_models

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	clientapi "github.com/hcd233/aris-proxy-api/internal/client/api"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

// fiberTestTransport 把真实 fiber app 适配为 http.Client RoundTripper
// （fiber v3 的 App 未实现 http.Handler，无法直接挂到 httptest.NewServer）。
type fiberTestTransport struct{ app *fiber.App }

func (t fiberTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.app.Test(req)
}

// TestClientSDKListModelsAgainstProductionRoutes SDK 拼路径必须命中生产注册路由。
func TestClientSDKListModelsAgainstProductionRoutes(t *testing.T) {
	t.Parallel()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(gormsqlite.Open("file:"+dbName+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.ProxyAPIKey{}, &dbmodel.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	user := dbmodel.User{Name: "sdk-e2e-user"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&dbmodel.ProxyAPIKey{UserID: user.ID, Name: "sdk-e2e-key", Key: routeTestAPIKey}).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("client sdk e2e", "1.0"))
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

	hc := &http.Client{Transport: fiberTestTransport{app: app}}
	client := clientapi.New("http://sdk-e2e.local", routeTestAPIKey, hc)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("SDK ListModels against production routes must succeed, got %v (404 = SDK path and server registration diverged)", err)
	}
	if len(models) != 1 || models[0].Alias != "gpt-4o" {
		t.Fatalf("unexpected models: %+v", models)
	}
}
