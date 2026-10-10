// Package read_cache 验证高频读接口（模型目录类）Redis 读缓存的端到端行为：
//
//   - 缓存命中后不再回源查库（GORM 查询计数）
//   - 空结果缓存防缓存穿透（重复查询不存在的数据不再打库）
//   - 模型写路径失效缓存（不脏读）
//   - TTL 随机抖动防缓存雪崩
//   - 加缓存前后 QPS 对比压测（qps_test.go）
//
// 装配与生产同构：RegisterAPIRouter + APIKey/JWT 中间件 + 真实仓储/用例 + sqlite + miniredis，
// 沿用 test/e2e/goroutine_leak 与 test/e2e/cross_tenant_reference 的夹具范式。
//
// 串行约束：sqlite 内存库 + 共享夹具，子测试串行执行（沿用 cross_tenant 系列结论）。
//
//nolint:paralleltest // 共享 sqlite 内存库与缓存夹具，子测试必须串行
package read_cache

import (
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

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	modelcommand "github.com/hcd233/aris-proxy-api/internal/application/model/command"
	modelquery "github.com/hcd233/aris-proxy-api/internal/application/model/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmproxyservice "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

const (
	e2eHTTPTimeout = 10 * time.Second
	proxyAPIKey    = "sk-e2e-read-cache"
)

// 嵌入接口空结构体：注册期满足方法集、不解引用（cross_tenant / goroutine_leak 惯例）
type (
	stubPingHandler       struct{ handler.PingHandler }
	stubTraceHandler      struct{ handler.TraceHandler }
	stubTokenHandler      struct{ handler.TokenHandler }
	stubOauth2Handler     struct{ handler.Oauth2Handler }
	stubUserHandler       struct{ handler.UserHandler }
	stubDemoHandler       struct{ handler.DemoHandler }
	stubAPIKeyHandler     struct{ handler.APIKeyHandler }
	stubSessionHandler    struct{ handler.SessionHandler }
	stubEndpointHandler   struct{ handler.EndpointHandler }
	stubAuditHandler      struct{ handler.AuditHandler }
	stubCronHandler       struct{ handler.CronHandler }
	stubTriggerHandler    struct{ handler.TriggerHandler }
	stubMetricsHandler    struct{ handler.MetricsHandler }
	stubDatasetHandler    struct{ handler.DatasetHandler }
	stubUpstreamHandler   struct{ handler.UpstreamHandler }
	stubPlaygroundHandler struct{ handler.PlaygroundHandler }
)

// noopTaskSubmitter 丢弃异步任务（读接口不触发）
type noopTaskSubmitter struct{}

func (noopTaskSubmitter) SubmitModelCallAuditTask(_ *dto.ModelCallAuditTask) error { return nil }
func (noopTaskSubmitter) SubmitMessageStoreTask(_ *dto.MessageStoreTask) error     { return nil }

// noopTriggerChecker 触发词检查 no-op（读接口不触发）
type noopTriggerChecker struct{}

func (noopTriggerChecker) Check(_ string) []uint                           { return nil }
func (noopTriggerChecker) MatchedWords(ids []uint) []string                { return nil }
func (noopTriggerChecker) DenyIDs(ids []uint) []uint                       { return nil }
func (noopTriggerChecker) OmitIDs(ids []uint) []uint                       { return nil }
func (noopTriggerChecker) CaptureIDs(ids []uint) []uint                    { return nil }
func (noopTriggerChecker) IncrementHits(_ context.Context, _ []uint) error { return nil }

// dbSeq 同一测试内多次构建夹具时保证 sqlite 内存库名唯一（cache=shared 下同名即同库）
var dbSeq atomic.Int64

type fixture struct {
	db         *gorm.DB
	mr         *miniredis.Miniredis
	baseURL    string
	client     *http.Client
	jwt        string
	apiKey     string
	userID     uint
	endpointID uint
	modelCount int
	sqlQueries *atomic.Int64
}

// newFixture 构建端到端夹具：withCache 决定仓储是否装配读缓存
// （false = 「加缓存前」的原路径，true = 「加缓存后」）。
func newFixture(t *testing.T, withCache bool, modelCount int) *fixture {
	t.Helper()

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
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap sql db: %v", err)
	}
	// 单连接串行化 DB 访问：模拟生产 DB 连接池瓶颈，同时规避 sqlite 共享缓存的锁冲突
	sqlDB.SetMaxOpenConns(1)

	f := &fixture{db: db, mr: mr, apiKey: proxyAPIKey, modelCount: modelCount, sqlQueries: &atomic.Int64{}}
	if err := db.Callback().Query().Before("gorm:query").Register("test:count_query", func(gdb *gorm.DB) {
		f.sqlQueries.Add(1)
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.User{}, &dbmodel.ProxyAPIKey{}, &dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	endpointRepo, modelRepo, readRepo := f.repos(withCache, rdb)
	userRepo := repository.NewUserRepository(db)

	httpclient.InitHTTPClient()
	registry := metrics.NewRegistry()
	tokenMetrics := metrics.NewTokenUsageCounter(registry)
	tracker := inflight.NewTracker()
	guard := transport.NewEndpointGuard(registry)
	openAIProxy := transport.NewOpenAIProxy(tracker, guard)
	anthropicProxy := transport.NewAnthropicProxy(tracker, guard)

	openAIUC := usecase.NewOpenAIUseCase(
		llmproxyservice.NewEndpointResolver(endpointRepo, modelRepo, false, nil),
		usecase.NewListOpenAIModels(readRepo),
		openAIProxy,
		anthropicProxy,
		noopTaskSubmitter{},
		noopTriggerChecker{},
		tokenMetrics,
		nil,
	)
	anthropicUC := usecase.NewAnthropicUseCase(
		llmproxyservice.NewEndpointResolver(endpointRepo, modelRepo, false, nil),
		usecase.NewListAnthropicModels(readRepo),
		usecase.NewCountTokens(readRepo, anthropicProxy),
		anthropicProxy,
		openAIProxy,
		noopTaskSubmitter{},
		noopTriggerChecker{},
		tokenMetrics,
		nil,
	)
	modelHandler := handler.NewModelHandler(handler.ModelDependencies{
		Create: modelcommand.NewCreateModelHandler(endpointRepo, modelRepo),
		Update: modelcommand.NewUpdateModelHandler(endpointRepo, modelRepo),
		Delete: modelcommand.NewDeleteModelHandler(modelRepo),
		List:   modelquery.NewListModelHandler(modelRepo, endpointRepo, userRepo),
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("read cache e2e", "1.0"))
	sseGauge := metrics.NewSSEGauge(registry)
	// 测试环境无 api.env：显式给出 JWT 密钥与有效期（沿用 cross_tenant 夹具做法）
	config.JwtAccessTokenSecret = "read-cache-e2e-secret"
	config.JwtAccessTokenExpired = time.Hour
	signer := jwt.NewAccessTokenSigner()
	router.RegisterAPIRouter(api, router.APIRouterDependencies{
		DB:                db,
		Cache:             rdb,
		AccessSigner:      signer,
		PingHandler:       &stubPingHandler{},
		TraceHandler:      &stubTraceHandler{},
		TokenHandler:      &stubTokenHandler{},
		Oauth2Handler:     &stubOauth2Handler{},
		UserHandler:       &stubUserHandler{},
		DemoHandler:       &stubDemoHandler{},
		APIKeyHandler:     &stubAPIKeyHandler{},
		SessionHandler:    &stubSessionHandler{},
		EndpointHandler:   &stubEndpointHandler{},
		ModelHandler:      modelHandler,
		UpstreamHandler:   &stubUpstreamHandler{},
		PlaygroundHandler: &stubPlaygroundHandler{},
		AuditHandler:      &stubAuditHandler{},
		CronHandler:       &stubCronHandler{},
		TriggerHandler:    &stubTriggerHandler{},
		OpenAIHandler:     handler.NewOpenAIHandler(handler.OpenAIDependencies{UseCase: openAIUC, SSEGauge: sseGauge}),
		AnthropicHandler:  handler.NewAnthropicHandler(handler.AnthropicDependencies{UseCase: anthropicUC, SSEGauge: sseGauge}),
		MetricsHandler:    &stubMetricsHandler{},
		DatasetHandler:    &stubDatasetHandler{},
		ClientHandler:     handler.NewClientHandler(handler.ClientDependencies{List: usecase.NewListClientModels(readRepo)}),
	})

	f.seed(t, signer)

	listenConfig := net.ListenConfig{}
	ln, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = app.ShutdownWithContext(shutdownCtx)
	})

	f.baseURL = "http://" + ln.Addr().String()
	f.client = &http.Client{
		Timeout:   e2eHTTPTimeout,
		Transport: &http.Transport{MaxIdleConns: 256, MaxIdleConnsPerHost: 256},
	}
	f.waitReady(t)
	return f
}

// repos 按 withCache 装配仓储（false = 无缓存原路径，true = 带读缓存/失效钩子）
func (f *fixture) repos(withCache bool, rdb *redis.Client) (llmproxy.EndpointRepository, llmproxy.ModelRepository, llmproxy.EndpointReadRepository) {
	if !withCache {
		return repository.NewEndpointRepository(f.db),
			repository.NewModelRepository(f.db),
			repository.NewEndpointReadRepository(f.db)
	}
	readCache := cache.NewReadCache(rdb)
	return repository.NewCachedEndpointRepository(f.db, readCache),
		repository.NewCachedModelRepository(f.db, readCache),
		repository.NewCachedEndpointReadRepository(f.db, readCache)
}

// seed 播种：普通用户 + API Key + endpoint + modelCount 个模型
func (f *fixture) seed(t *testing.T, signer jwt.TokenSigner) {
	t.Helper()
	user := &dbmodel.User{Name: "read-cache-user", GithubBindID: "gh-read-cache", GoogleBindID: "gg-read-cache", Permission: enum.PermissionUser}
	if err := f.db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	f.userID = user.ID
	if err := f.db.Create(&dbmodel.ProxyAPIKey{UserID: user.ID, Name: "read-cache", Key: proxyAPIKey}).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}
	ep := &dbmodel.Endpoint{UserID: user.ID, Name: "ep-read-cache", APIKey: "sk-upstream",
		OpenaiBaseURL: "https://upstream.invalid", AnthropicBaseURL: "https://upstream.invalid",
		SupportOpenAIChatCompletion: true, SupportAnthropicMessage: true}
	if err := f.db.Create(ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	f.endpointID = ep.ID
	for i := range f.modelCount {
		alias := fmt.Sprintf("model-%03d", i)
		m := &dbmodel.Model{UserID: user.ID, Alias: alias, ModelID: alias, UpstreamModel: "up-" + alias,
			EndpointID: ep.ID, Capabilities: []enum.InputModality{enum.InputModalityText}}
		if err := f.db.Create(m).Error; err != nil {
			t.Fatalf("create model %s: %v", alias, err)
		}
	}
	token, err := signer.EncodeToken(user.ID)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	f.jwt = token
}

func (f *fixture) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, body := f.do(t, http.MethodGet, "/api/openai/v1/models", f.apiKey, "")
		if status == http.StatusOK && strings.Contains(string(body), `"data"`) {
			return
		}
		<-time.After(10 * time.Millisecond) //nolint:revive // 就绪轮询间隔
	}
	t.Fatalf("server not ready in 10s")
}

// do 发起一次请求（真实 TCP），返回状态码与响应体
func (f *fixture) do(t *testing.T, method, path, token, body string) (status int, data []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), e2eHTTPTimeout)
	defer cancel()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, f.baseURL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	resp, err := f.client.Do(req)
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

// doJSON 发起请求并断言 200 + 响应体可解析，返回原始响应体
func (f *fixture) doJSON(t *testing.T, method, path, token, body string) []byte {
	t.Helper()
	status, data := f.do(t, method, path, token, body)
	if status != http.StatusOK {
		t.Fatalf("%s %s status = %d, want 200 (body=%s)", method, path, status, data)
	}
	var envelope map[string]any
	if err := sonic.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("decode envelope: %v (raw=%s)", err, data)
	}
	if bizErr, ok := envelope["error"]; ok && bizErr != nil {
		t.Fatalf("%s %s business error: %s", method, path, data)
	}
	return data
}

// get 发起一次 GET 请求（压测用，不触发 t.Fatalf，可在并发协程中调用）
func (f *fixture) get(path, token string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), e2eHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+path, http.NoBody)
	if err != nil {
		return 0, err
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	resp, err := f.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// queryCount 快照当前 SQL 计数
func (f *fixture) queryCount() int64 {
	return f.sqlQueries.Load()
}

// jsonHas 断言响应体包含子串
func jsonHas(t *testing.T, data []byte, want string) {
	t.Helper()
	if !strings.Contains(string(data), want) {
		t.Fatalf("response missing %q: %s", want, data)
	}
}
