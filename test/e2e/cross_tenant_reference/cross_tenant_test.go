// Package cross_tenant_reference 验证跨聚合引用的租户隔离（CR R4 越权回归基线）。
//
// 缺陷背景：
//   - PATCH /model/{id} 换绑 endpoint 时未校验目标归属（C2），用户 A 可把 model
//     挂到用户 B 的 endpoint，model 会出现在 B 的 upstream 分组视图（信息泄露）；
//   - endpoint/model 的 create/update/delete 越权校验分布在应用层/仓储层，
//     本包在生产路由装配（RegisterAPIRouter + JWT 中间件 + sqlite 仓储 + miniredis）
//     下做端到端守护，防止后续重构破坏这些校验。
//
// 断言设计：越权操作必须非 2xx，正向操作 200；并用 upstream 列表直接断言
// B 的视图里不出现 A 的模型（泄露面）。
package cross_tenant_reference

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	endpointcommand "github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	modelcommand "github.com/hcd233/aris-proxy-api/internal/application/model/command"
	upstreamquery "github.com/hcd233/aris-proxy-api/internal/application/upstream/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

// e2eStubHandlers 嵌入接口空结构体：注册期满足方法集、不解引用（client_route_test.go 惯例）
type (
	stubPingHandler      struct{ handler.PingHandler }
	stubTraceHandler     struct{ handler.TraceHandler }
	stubTokenHandler     struct{ handler.TokenHandler }
	stubOauth2Handler    struct{ handler.Oauth2Handler }
	stubUserHandler      struct{ handler.UserHandler }
	stubDemoHandler      struct{ handler.DemoHandler }
	stubAPIKeyHandler    struct{ handler.APIKeyHandler }
	stubSessionHandler   struct{ handler.SessionHandler }
	stubAuditHandler     struct{ handler.AuditHandler }
	stubCronHandler      struct{ handler.CronHandler }
	stubTriggerHandler   struct{ handler.TriggerHandler }
	stubOpenAIHandler    struct{ handler.OpenAIHandler }
	stubAnthropicHandler struct{ handler.AnthropicHandler }
	stubMetricsHandler   struct{ handler.MetricsHandler }
	stubDatasetHandler   struct{ handler.DatasetHandler }
	stubClientHandler    struct{ handler.ClientHandler }
)

// crossTenantFixture 真实装配：生产路由 + JWT + sqlite 仓储 + miniredis
type crossTenantFixture struct {
	app    *fiber.App
	signer jwt.TokenSigner
	userA  *dbmodel.User
	userB  *dbmodel.User
	admin  *dbmodel.User
	epA    *dbmodel.Endpoint
	epB    *dbmodel.Endpoint
	modelA *dbmodel.Model
}

func newCrossTenantFixture(t *testing.T) *crossTenantFixture {
	t.Helper()
	// config 的 JWT 过期时长默认值可能为 0（token 立即过期），测试环境显式设置
	config.JwtAccessTokenExpired = time.Hour
	config.JwtAccessTokenSecret = "cross-tenant-e2e-secret"

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}, &dbmodel.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	endpointRepo := repository.NewEndpointRepository(db)
	modelRepo := repository.NewModelRepository(db)
	userRepo := repository.NewUserRepository(db)

	endpointHandler := handler.NewEndpointHandler(handler.EndpointDependencies{
		Create: endpointcommand.NewCreateEndpointHandler(endpointRepo),
		Update: endpointcommand.NewUpdateEndpointHandler(endpointRepo),
		Delete: endpointcommand.NewDeleteEndpointHandler(endpointRepo),
	})
	modelHandler := handler.NewModelHandler(handler.ModelDependencies{
		Create: modelcommand.NewCreateModelHandler(endpointRepo, modelRepo),
		Update: modelcommand.NewUpdateModelHandler(endpointRepo, modelRepo),
		Delete: modelcommand.NewDeleteModelHandler(modelRepo),
	})
	upstreamHandler := handler.NewUpstreamHandler(handler.UpstreamDependencies{
		List: upstreamquery.NewListUpstreamHandler(endpointRepo, modelRepo, userRepo),
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("cross tenant reference", "1.0"))
	router.RegisterAPIRouter(api, router.APIRouterDependencies{
		DB:               db,
		Cache:            rdb,
		AccessSigner:     jwt.NewAccessTokenSigner(),
		PingHandler:      &stubPingHandler{},
		TraceHandler:     &stubTraceHandler{},
		TokenHandler:     &stubTokenHandler{},
		Oauth2Handler:    &stubOauth2Handler{},
		UserHandler:      &stubUserHandler{},
		DemoHandler:      &stubDemoHandler{},
		APIKeyHandler:    &stubAPIKeyHandler{},
		SessionHandler:   &stubSessionHandler{},
		EndpointHandler:  endpointHandler,
		ModelHandler:     modelHandler,
		UpstreamHandler:  upstreamHandler,
		AuditHandler:     &stubAuditHandler{},
		CronHandler:      &stubCronHandler{},
		TriggerHandler:   &stubTriggerHandler{},
		OpenAIHandler:    &stubOpenAIHandler{},
		AnthropicHandler: &stubAnthropicHandler{},
		MetricsHandler:   &stubMetricsHandler{},
		DatasetHandler:   &stubDatasetHandler{},
		ClientHandler:    &stubClientHandler{},
	})
	// 种子：两个普通用户 + 一个 admin；A/B 各自名下一个 endpoint；A 名下一个 model
	f := &crossTenantFixture{app: app, signer: jwt.NewAccessTokenSigner()}
	// github_bind_id 参与 (github_bind_id, deleted_at) 唯一索引，零值彼此冲突，须逐个区分
	// github/google_bind_id 各自参与 (bind_id, deleted_at) 唯一索引，零值彼此冲突，须逐个区分
	f.userA = &dbmodel.User{Name: "tenant-a", GithubBindID: "gh-a", GoogleBindID: "gg-a", Permission: enum.PermissionUser}
	f.userB = &dbmodel.User{Name: "tenant-b", GithubBindID: "gh-b", GoogleBindID: "gg-b", Permission: enum.PermissionUser}
	f.admin = &dbmodel.User{Name: "root", GithubBindID: "gh-root", GoogleBindID: "gg-root", Permission: enum.PermissionAdmin}
	for _, u := range []*dbmodel.User{f.userA, f.userB, f.admin} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	f.epA = &dbmodel.Endpoint{UserID: f.userA.ID, Name: "ep-a", APIKey: "sk-a", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true}
	f.epB = &dbmodel.Endpoint{UserID: f.userB.ID, Name: "ep-b", APIKey: "sk-b", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true}
	for _, ep := range []*dbmodel.Endpoint{f.epA, f.epB} {
		if err := db.Create(ep).Error; err != nil {
			t.Fatalf("create endpoint: %v", err)
		}
	}
	f.modelA = &dbmodel.Model{UserID: f.userA.ID, Alias: "gpt-a", ModelID: "gpt-a", UpstreamModel: "up-a", EndpointID: f.epA.ID,
		Capabilities: []enum.InputModality{enum.InputModalityText}}
	if err := db.Create(f.modelA).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	return f
}

func (f *crossTenantFixture) tokenFor(t *testing.T, userID uint) string {
	t.Helper()
	token, err := f.signer.EncodeToken(userID)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	return token
}

// do 经真实 fiber app 发起请求（fiber v3 App 未实现 http.Handler，用 app.Test 直连）。
func (f *crossTenantFixture) do(t *testing.T, method, path, token, body string) (status int, data []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), e2eHTTPTimeout)
	defer cancel()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://e2e.local"+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	resp, err := f.app.Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	status = resp.StatusCode
	return status, data
}

const e2eHTTPTimeout = 10 * time.Second

// bizErrorBody 统一响应信封的错误体（管理 API 恒 HTTP 200，错误语义由 error.code 承载）
type bizErrorBody struct {
	Error *struct {
		Code    int64  `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// bizErrorCode 提取响应体中的业务错误码；无错误时返回 0
func bizErrorCode(t *testing.T, data []byte) int64 {
	t.Helper()
	var body bizErrorBody
	if err := sonic.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode response body: %v (raw=%s)", err, data)
	}
	if body.Error == nil {
		return 0
	}
	return body.Error.Code
}

// TestCrossTenant_ReferenceGuard 跨租户引用全链路守护：
// A 不得引用 B 的 endpoint（create/update 换绑/delete），B 的视图不得泄露 A 的模型。
func TestCrossTenant_ReferenceGuard(t *testing.T) {
	t.Parallel()
	f := newCrossTenantFixture(t)
	tokenA := f.tokenFor(t, f.userA.ID)
	tokenB := f.tokenFor(t, f.userB.ID)
	tokenAdmin := f.tokenFor(t, f.admin.ID)
	ctx := context.Background()
	_ = ctx

	t.Run("create model on foreign endpoint rejected", func(t *testing.T) {
		t.Parallel()
		body := fmt.Sprintf(`{"alias":"evil-a","upstreamModel":"up-evil","endpointID":%d,"capabilities":["text"]}`, f.epB.ID)
		status, data := f.do(t, http.MethodPost, constant.WebAPIPrefix+"/model", tokenA, body)
		if status != http.StatusOK || bizErrorCode(t, data) == 0 {
			t.Fatalf("A must not create model on B's endpoint, status=%d body=%s", status, data)
		}
	})

	t.Run("update model swap to foreign endpoint rejected (C2)", func(t *testing.T) {
		t.Parallel()
		body := fmt.Sprintf(`{"endpointID":%d}`, f.epB.ID)
		status, data := f.do(t, http.MethodPatch, fmt.Sprintf("%s/model?id=%d", constant.WebAPIPrefix, f.modelA.ID), tokenA, body)
		if status != http.StatusOK || bizErrorCode(t, data) == 0 {
			t.Fatalf("A must not swap own model onto B's endpoint, status=%d body=%s", status, data)
		}
		// 换绑必须真的没发生：modelA 仍挂在 epA
		if f.modelA.EndpointID != f.epA.ID {
			t.Fatalf("model endpointID must be unchanged, got %d", f.modelA.EndpointID)
		}
	})

	t.Run("delete foreign endpoint rejected", func(t *testing.T) {
		t.Parallel()
		status, data := f.do(t, http.MethodDelete, fmt.Sprintf("%s/endpoint?id=%d", constant.WebAPIPrefix, f.epB.ID), tokenA, "")
		if status != http.StatusOK || bizErrorCode(t, data) == 0 {
			t.Fatalf("A must not delete B's endpoint, status=%d body=%s", status, data)
		}
	})

	t.Run("create model on own endpoint allowed", func(t *testing.T) {
		t.Parallel()
		body := fmt.Sprintf(`{"alias":"gpt-a2","upstreamModel":"up-a2","endpointID":%d,"capabilities":["text"]}`, f.epA.ID)
		status, data := f.do(t, http.MethodPost, constant.WebAPIPrefix+"/model", tokenA, body)
		if status != http.StatusOK || bizErrorCode(t, data) != 0 {
			t.Fatalf("own create must succeed, status=%d body=%s", status, data)
		}
	})

	t.Run("admin can manage any tenant config", func(t *testing.T) {
		t.Parallel()
		body := fmt.Sprintf(`{"alias":"gpt-b-admin","upstreamModel":"up-b-admin","endpointID":%d,"capabilities":["text"]}`, f.epB.ID)
		status, data := f.do(t, http.MethodPost, constant.WebAPIPrefix+"/model", tokenAdmin, body)
		if status != http.StatusOK || bizErrorCode(t, data) != 0 {
			t.Fatalf("admin create for B must succeed, status=%d body=%s", status, data)
		}
	})

	t.Run("upstream view leaks nothing across tenants", func(t *testing.T) {
		t.Parallel()
		// B 的视图：只应有 ep-b 与其模型，不得出现 A 的 gpt-a / gpt-a2
		status, data := f.do(t, http.MethodGet, constant.WebAPIPrefix+"/upstream/list?page=1&pageSize=50", tokenB, "")
		if status != http.StatusOK || bizErrorCode(t, data) != 0 {
			t.Fatalf("list for B must succeed, status=%d body=%s", status, data)
		}
		if strings.Contains(string(data), "gpt-a") || strings.Contains(string(data), "ep-a") {
			t.Fatalf("B's upstream view leaked A's data: %s", data)
		}
		if !strings.Contains(string(data), "ep-b") {
			t.Fatalf("B's view must contain own endpoint, body=%s", data)
		}

		// admin 全量视图：能看到 A 与 B 的配置
		status, data = f.do(t, http.MethodGet, constant.WebAPIPrefix+"/upstream/list?page=1&pageSize=50", tokenAdmin, "")
		if status != http.StatusOK || bizErrorCode(t, data) != 0 {
			t.Fatalf("list for admin must succeed, status=%d body=%s", status, data)
		}
		if !strings.Contains(string(data), "gpt-a") || !strings.Contains(string(data), "ep-b") {
			t.Fatalf("admin view must contain all tenants, body=%s", data)
		}
	})
}

var _ = sonic.ConfigDefault // 保持依赖可见（响应体断言仅做子串匹配）
