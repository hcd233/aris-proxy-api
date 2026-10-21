// Package public_endpoint 守护「公开端点」的可见性与只读边界：
//
//   - admin 创建的 public 端点，普通用户在 upstream 列表可见（带 visibility 字段）；
//   - 普通用户不能改/删该端点及其模型（租户 scope 天然拦截 → data_not_exists）；
//   - 普通用户不能把端点创建成 public（只有 admin 可以 → unauthorized）；
//   - admin 改回 private 后，普通用户立即看不到。
//
// 装配方式与 cross_tenant_reference 一致：真实路由 + JWT + sqlite 仓储 + miniredis。
//
//nolint:paralleltest // 共享 sqlite 内存库与 fixture，子测试串行执行
package public_endpoint

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
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
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// 端点创建会做 baseURL 的 SSRF 校验（含 DNS 解析）：注入固定公网解析结果，
// 避免测试依赖真实 DNS（与 test/unit/endpoint_command 同一做法）。
func init() {
	util.LookupIPFn = func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.10")}, nil
	}
}

// e2eStubHandlers 嵌入接口空结构体：注册期满足方法集、不解引用。
type (
	stubPingHandler       struct{ handler.PingHandler }
	stubTraceHandler      struct{ handler.TraceHandler }
	stubTokenHandler      struct{ handler.TokenHandler }
	stubOauth2Handler     struct{ handler.Oauth2Handler }
	stubUserHandler       struct{ handler.UserHandler }
	stubDemoHandler       struct{ handler.DemoHandler }
	stubAPIKeyHandler     struct{ handler.APIKeyHandler }
	stubSessionHandler    struct{ handler.SessionHandler }
	stubAuditHandler      struct{ handler.AuditHandler }
	stubCronHandler       struct{ handler.CronHandler }
	stubTriggerHandler    struct{ handler.TriggerHandler }
	stubOpenAIHandler     struct{ handler.OpenAIHandler }
	stubPlaygroundHandler struct{ handler.PlaygroundHandler }
	stubAnthropicHandler  struct{ handler.AnthropicHandler }
	stubMetricsHandler    struct{ handler.MetricsHandler }
	stubDatasetHandler    struct{ handler.DatasetHandler }
	stubClientHandler     struct{ handler.ClientHandler }
)

type fixture struct {
	db     *gorm.DB
	app    *fiber.App
	signer jwt.TokenSigner
	user   *dbmodel.User
	admin  *dbmodel.User
}

// dbSeq 保证每次装配的内存库名唯一（cache=shared 的 sqlite 内存库按名字共享生命周期）。
var dbSeq atomic.Uint64

func newFixture(t *testing.T) *fixture {
	t.Helper()
	config.JwtAccessTokenExpired = time.Hour
	config.JwtAccessTokenSecret = "public-endpoint-e2e-secret"

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dbName = fmt.Sprintf("%s-%d", dbName, dbSeq.Add(1))
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
		List: upstreamquery.NewListUpstreamHandler(endpointRepo, modelRepo, userRepo, repository.NewOAuthCredentialRepository(db)),
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("public endpoint e2e", "1.0"))
	router.RegisterAPIRouter(api, router.APIRouterDependencies{
		DB:                db,
		Cache:             rdb,
		AccessSigner:      jwt.NewAccessTokenSigner(),
		PingHandler:       &stubPingHandler{},
		TraceHandler:      &stubTraceHandler{},
		TokenHandler:      &stubTokenHandler{},
		Oauth2Handler:     &stubOauth2Handler{},
		UserHandler:       &stubUserHandler{},
		DemoHandler:       &stubDemoHandler{},
		APIKeyHandler:     &stubAPIKeyHandler{},
		SessionHandler:    &stubSessionHandler{},
		EndpointHandler:   endpointHandler,
		ModelHandler:      modelHandler,
		UpstreamHandler:   upstreamHandler,
		AuditHandler:      &stubAuditHandler{},
		CronHandler:       &stubCronHandler{},
		TriggerHandler:    &stubTriggerHandler{},
		OpenAIHandler:     &stubOpenAIHandler{},
		PlaygroundHandler: &stubPlaygroundHandler{},
		AnthropicHandler:  &stubAnthropicHandler{},
		MetricsHandler:    &stubMetricsHandler{},
		DatasetHandler:    &stubDatasetHandler{},
		ClientHandler:     &stubClientHandler{},
	})

	f := &fixture{db: db, app: app, signer: jwt.NewAccessTokenSigner()}
	f.user = &dbmodel.User{Name: "alice", GithubBindID: "gh-alice", GoogleBindID: "gg-alice", Permission: enum.PermissionUser}
	f.admin = &dbmodel.User{Name: "root", GithubBindID: "gh-root", GoogleBindID: "gg-root", Permission: enum.PermissionAdmin}
	for _, u := range []*dbmodel.User{f.user, f.admin} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	return f
}

func (f *fixture) tokenFor(t *testing.T, userID uint) string {
	t.Helper()
	token, err := f.signer.EncodeToken(userID)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	return token
}

func (f *fixture) do(t *testing.T, method, path, token, body string) (status int, data []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	return resp.StatusCode, data
}

const (
	bizCodeUnauthorized  int64 = 10001
	bizCodeDataNotExists int64 = 10003
)

type bizErrorBody struct {
	Error *struct {
		Code int64 `json:"code"`
	} `json:"error,omitempty"`
}

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

func requireBizCode(t *testing.T, status int, data []byte, want int64, what string) {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("%s: expect HTTP 200 envelope, got status=%d body=%s", what, status, data)
	}
	if got := bizErrorCode(t, data); got != want {
		t.Fatalf("%s: biz code = %d, want %d body=%s", what, got, want, data)
	}
}

type upstreamListRsp struct {
	Groups []struct {
		Endpoint struct {
			ID         uint   `json:"id"`
			Name       string `json:"name"`
			Visibility string `json:"visibility"`
		} `json:"endpoint"`
	} `json:"groups"`
}

func (f *fixture) listUpstream(t *testing.T, token string) upstreamListRsp {
	t.Helper()
	status, data := f.do(t, http.MethodGet, constant.WebAPIPrefix+"/upstream/list?page=1&pageSize=50", token, "")
	requireBizCode(t, status, data, 0, "list upstream")
	var rsp upstreamListRsp
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("decode list rsp: %v (raw=%s)", err, data)
	}
	return rsp
}

func TestPublicEndpoint_VisibilityAndReadOnly(t *testing.T) {
	f := newFixture(t)
	userToken := f.tokenFor(t, f.user.ID)
	adminToken := f.tokenFor(t, f.admin.ID)

	// admin 创建公开端点
	status, data := f.do(t, http.MethodPost, constant.WebAPIPrefix+"/endpoint", adminToken,
		`{"name":"ep-public","visibility":"public","apiKey":"sk-pub","openaiBaseURL":"https://o.example.com","supportOpenAIChatCompletion":true}`)
	requireBizCode(t, status, data, 0, "admin create public endpoint")

	var ep dbmodel.Endpoint
	if err := f.db.Where("name = ?", "ep-public").First(&ep).Error; err != nil {
		t.Fatalf("reload public endpoint: %v", err)
	}
	if ep.Visibility != string(enum.EndpointVisibilityPublic) {
		t.Fatalf("persisted visibility = %q, want public", ep.Visibility)
	}

	// admin 在该端点下建模型
	body := fmt.Sprintf(`{"alias":"pub-model","upstreamModel":"up-pub","endpointID":%d,"capabilities":["text"]}`, ep.ID)
	status, data = f.do(t, http.MethodPost, constant.WebAPIPrefix+"/model", adminToken, body)
	requireBizCode(t, status, data, 0, "admin create model on public endpoint")

	var m dbmodel.Model
	if err := f.db.Where("alias = ?", "pub-model").First(&m).Error; err != nil {
		t.Fatalf("reload public model: %v", err)
	}

	// 普通用户可见且带 visibility=public
	list := f.listUpstream(t, userToken)
	found := false
	for _, g := range list.Groups {
		if g.Endpoint.Name == "ep-public" {
			found = true
			if g.Endpoint.Visibility != "public" {
				t.Fatalf("user view visibility = %q, want public", g.Endpoint.Visibility)
			}
		}
	}
	if !found {
		t.Fatalf("public endpoint must be visible to normal user, body groups=%+v", list.Groups)
	}

	// 普通用户不能改/删公开端点
	status, data = f.do(t, http.MethodPatch, fmt.Sprintf("%s/endpoint?id=%d", constant.WebAPIPrefix, ep.ID), userToken, `{"name":"hijack"}`)
	requireBizCode(t, status, data, bizCodeDataNotExists, "user patch public endpoint")

	status, data = f.do(t, http.MethodDelete, fmt.Sprintf("%s/endpoint?id=%d", constant.WebAPIPrefix, ep.ID), userToken, "")
	requireBizCode(t, status, data, bizCodeDataNotExists, "user delete public endpoint")

	// 普通用户不能改/删公开端点下的模型
	status, data = f.do(t, http.MethodPatch, fmt.Sprintf("%s/model?id=%d", constant.WebAPIPrefix, m.ID), userToken, `{"alias":"hijack"}`)
	requireBizCode(t, status, data, bizCodeDataNotExists, "user patch public model")

	status, data = f.do(t, http.MethodDelete, fmt.Sprintf("%s/model?id=%d", constant.WebAPIPrefix, m.ID), userToken, "")
	requireBizCode(t, status, data, bizCodeDataNotExists, "user delete public model")

	// 普通用户不能创建公开端点
	status, data = f.do(t, http.MethodPost, constant.WebAPIPrefix+"/endpoint", userToken,
		`{"name":"ep-user-public","visibility":"public","apiKey":"sk-u","openaiBaseURL":"https://o.example.com","supportOpenAIChatCompletion":true}`)
	requireBizCode(t, status, data, bizCodeUnauthorized, "user create public endpoint")

	// 端点与模型都还在
	if err := f.db.First(&dbmodel.Endpoint{}, ep.ID).Error; err != nil {
		t.Fatalf("public endpoint must survive: %v", err)
	}
	if err := f.db.First(&dbmodel.Model{}, m.ID).Error; err != nil {
		t.Fatalf("public model must survive: %v", err)
	}

	// admin 改回私有后，普通用户立刻看不到
	status, data = f.do(t, http.MethodPatch, fmt.Sprintf("%s/endpoint?id=%d", constant.WebAPIPrefix, ep.ID), adminToken, `{"visibility":"private"}`)
	requireBizCode(t, status, data, 0, "admin flip to private")

	list = f.listUpstream(t, userToken)
	for _, g := range list.Groups {
		if g.Endpoint.Name == "ep-public" {
			t.Fatalf("private endpoint must disappear from user view: %+v", g.Endpoint)
		}
	}
}
