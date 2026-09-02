// Package goroutine_leak 复现生产 goroutine 泄漏（2026-09-02 排障）。
//
// 生产证据（Redis 运行时快照，pod 2g6x8/vbfs9，24h 窗口）：
//   - goroutine 随 LLM 代理流量阶梯上涨（55→62→89→126 / 60→65→91→118），流量过后不回落；
//   - sseActive 在流结束后正常归零（Read 路径正常完成），排除「流未消费」场景；
//   - 每请求泄漏约 5-10 个 goroutine。
//
// 本测试以生产同构装配（RegisterAPIRouter + APIKeyMiddleware + 真实
// transport/usecase/adapter 链 + httptest 上游 SSE + 真实 TCP listener）
// 复现：N 个流式请求结束后，关闭空闲连接排除连接池背景噪声，若
// goroutine 数仍不回落即泄漏成立，并 dump goroutine profile 定位栈。
package goroutine_leak

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	llmproxyservice "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

const (
	proxyAPIKey  = "sk-leak-repro"
	exposedModel = "gpt-leak"
	requestCount = 10
)

// e2eStubHandlers 嵌入接口空结构体：注册期满足方法集、不解引用（cross_tenant_test.go 惯例）。
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
	stubAuditHandler     struct{ handler.AuditHandler }
	stubCronHandler      struct{ handler.CronHandler }
	stubAnthropicHandler struct{ handler.AnthropicHandler }
	stubTriggerHandler   struct{ handler.TriggerHandler }
	stubMetricsHandler   struct{ handler.MetricsHandler }
	stubDatasetHandler   struct{ handler.DatasetHandler }
	stubClientHandler    struct{ handler.ClientHandler }
	stubUpstreamHandler  struct{ handler.UpstreamHandler }
)

// noopTaskSubmitter 丢弃异步任务：聚焦 transport/adapter 主链路泄漏；
// 若主链路无泄漏，再换真实 PoolManager 二分定位。
type noopTaskSubmitter struct{}

func (noopTaskSubmitter) SubmitModelCallAuditTask(_ *dto.ModelCallAuditTask) error { return nil }
func (noopTaskSubmitter) SubmitMessageStoreTask(_ *dto.MessageStoreTask) error     { return nil }

// noopTriggerChecker 触发词检查 no-op。
type noopTriggerChecker struct{}

func (noopTriggerChecker) Check(_ string) []uint                           { return nil }
func (noopTriggerChecker) MatchedWords(ids []uint) []string                { return nil }
func (noopTriggerChecker) DenyIDs(ids []uint) []uint                       { return nil }
func (noopTriggerChecker) OmitIDs(ids []uint) []uint                       { return nil }
func (noopTriggerChecker) CaptureIDs(ids []uint) []uint                    { return nil }
func (noopTriggerChecker) IncrementHits(_ context.Context, _ []uint) error { return nil }

type leakFixture struct {
	app        *fiber.App
	baseURL    string
	client     *http.Client
	upstreamUp func()
}

func newLeakFixture(t *testing.T) *leakFixture {
	t.Helper()

	// 上游：OpenAI Chat SSE 流（正常发 chunk 后 EOF）
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for i := range 3 {
			chunk := fmt.Sprintf(`{"id":"c%d","object":"chat.completion.chunk","model":"gpt-up","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`, i)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	t.Cleanup(upstream.Close)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.User{}, &dbmodel.ProxyAPIKey{}, &dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 真实代理链路装配（与 bootstrap/modules 同构）
	httpclient.InitHTTPClient()
	registry := metrics.NewRegistry()
	tracker := inflight.NewTracker()
	guard := transport.NewEndpointGuard(registry)
	openAIProxy := transport.NewOpenAIProxy(tracker, guard)
	anthropicProxy := transport.NewAnthropicProxy(tracker, guard)

	endpointRepo := repository.NewEndpointRepository(db)
	modelRepo := repository.NewModelRepository(db)
	resolver := llmproxyservice.NewEndpointResolver(endpointRepo, modelRepo, false)
	listModels := usecase.NewListOpenAIModels(repository.NewEndpointReadRepository(db))

	openAIUC := usecase.NewOpenAIUseCase(resolver, listModels, openAIProxy, anthropicProxy,
		noopTaskSubmitter{}, noopTriggerChecker{}, metrics.NewTokenUsageCounter(registry))
	openAIHandler := handler.NewOpenAIHandler(handler.OpenAIDependencies{
		UseCase:  openAIUC,
		SSEGauge: metrics.NewSSEGauge(registry),
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("goroutine leak repro", "1.0"))
	router.RegisterAPIRouter(api, router.APIRouterDependencies{
		DB:               db,
		Cache:            rdb,
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
		AuditHandler:     &stubAuditHandler{},
		CronHandler:      &stubCronHandler{},
		OpenAIHandler:    openAIHandler,
		AnthropicHandler: &stubAnthropicHandler{},
		TriggerHandler:   &stubTriggerHandler{},
		MetricsHandler:   &stubMetricsHandler{},
		DatasetHandler:   &stubDatasetHandler{},
		ClientHandler:    &stubClientHandler{},
		UpstreamHandler:  &stubUpstreamHandler{},
	})

	// 种子：用户 + 代理 API Key + endpoint（指向上游 httptest）+ model
	user := &dbmodel.User{Name: "leak-user", GithubBindID: "gh-leak", GoogleBindID: "gg-leak", Permission: enum.PermissionUser}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	key := &dbmodel.ProxyAPIKey{UserID: user.ID, Name: "leak", Key: proxyAPIKey}
	if err := db.Create(key).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}
	ep := &dbmodel.Endpoint{UserID: user.ID, Name: "ep-leak", APIKey: "sk-upstream",
		OpenaiBaseURL: upstream.URL, SupportOpenAIChatCompletion: true}
	if err := db.Create(ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	m := &dbmodel.Model{UserID: user.ID, Alias: exposedModel, ModelID: exposedModel, UpstreamModel: "gpt-up",
		EndpointID: ep.ID, Capabilities: []enum.InputModality{enum.InputModalityText}}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	// 真实 TCP listener（app.Test 走内存管道，写路径与生产不同，不用）
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

	f := &leakFixture{
		app:        app,
		baseURL:    "http://" + ln.Addr().String(),
		client:     &http.Client{Timeout: 30 * time.Second},
		upstreamUp: func() {},
	}

	// 等服务就绪：models 列表 200 即路由与鉴权可用
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, f.baseURL+"/api/openai/v1/models", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+proxyAPIKey)
		resp, err := f.client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return f
			}
		}
		<-time.After(100 * time.Millisecond) //nolint:revive // 就绪轮询间隔
	}
	t.Fatalf("server not ready in 10s")
	return nil
}

func (f *leakFixture) streamChat(t *testing.T) {
	t.Helper()
	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hello"}],"stream":true}`, exposedModel)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		f.baseURL+"/api/openai/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+proxyAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, data)
	}
	if !strings.Contains(string(data), "[DONE]") {
		t.Fatalf("stream missing [DONE]: %s", data)
	}
}

// stableGoroutineCount 反复 GC 后采样，连续两次一致即认为稳定（上限 wait）。
func stableGoroutineCount(wait time.Duration) int {
	deadline := time.Now().Add(wait)
	last := -1
	for time.Now().Before(deadline) {
		runtime.GC()
		n := runtime.NumGoroutine()
		if n == last {
			return n
		}
		last = n
		<-time.After(250 * time.Millisecond) //nolint:revive // 采样收敛轮询间隔
	}
	return last
}

// TestGoroutineLeakAfterStreamRequests 流式代理请求结束后 goroutine 必须回落。
//
// 判定：预热建立基线后发 requestCount 个流式请求，等待收尾并关闭客户端与上游
// client 的空闲连接（排除连接池常驻噪声），稳定 goroutine 数与基线差 > 2 即泄漏，
// 同时 dump goroutine profile 供栈级定位。
func TestGoroutineLeakAfterStreamRequests(t *testing.T) {
	t.Parallel()
	f := newLeakFixture(t)

	// 预热：2 个请求建立懒初始化路径（resolver/API key 缓存/连接池/fasthttp worker）
	for range 2 {
		f.streamChat(t)
	}

	baseline := stableGoroutineCount(5 * time.Second)
	t.Logf("baseline goroutines: %d", baseline)

	for i := range requestCount {
		f.streamChat(t)
		if i%5 == 4 {
			t.Logf("completed %d/%d requests", i+1, requestCount)
		}
	}

	// 等异步收尾（审计提交、审计落库、连接半关等）
	<-time.After(2 * time.Second) //nolint:revive // 异步收尾等待
	during := stableGoroutineCount(5 * time.Second)
	t.Logf("goroutines after %d requests (idle conns kept): %d", requestCount, during)

	// 关闭空闲连接排除池化背景噪声（客户端侧 + 上游侧共用单例 client）
	f.client.CloseIdleConnections()
	httpclient.GetHTTPClient().CloseIdleConnections()
	<-time.After(1 * time.Second) //nolint:revive // 空闲连接关闭等待

	final := stableGoroutineCount(5 * time.Second)
	t.Logf("goroutines after CloseIdleConnections: %d", final)

	delta := final - baseline
	if delta > 2 {
		dumpGoroutineProfile(t)
		t.Fatalf("goroutine leak reproduced: baseline=%d final=%d delta=%d after %d completed stream requests",
			baseline, final, delta, requestCount)
	}
	t.Logf("no leak: baseline=%d final=%d delta=%d", baseline, final, delta)
}

func dumpGoroutineProfile(t *testing.T) {
	t.Helper()
	path := "/tmp/goroutine-leak-profile.txt"
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create profile file: %v", err)
	}
	defer func() { _ = file.Close() }()
	if err := pprof.Lookup("goroutine").WriteTo(file, 1); err != nil {
		t.Fatalf("write goroutine profile: %v", err)
	}
	t.Logf("goroutine profile dumped to %s", path)
}
