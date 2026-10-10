// Package playground_attribution 验证 Playground 调用的 API Key 归属（CR 2026-10-09）。
//
// 缺陷背景：
//   - routeParams 漏注入 PlaygroundHandler，生产启动在 initPlaygroundRouter 对 nil
//     handler 解引用 panic（CrashLoopBackOff）；
//   - Playground 走 JWT 无 API Key，审计 api_key_id 记成 0，用户视角的审计列表/
//     成本图表（按 api_key_id IN 用户名下 key 过滤）看不到自己的调试用量。
//
// 修复后调用方须经 query apiKeyID 指定自己名下的 Key：归属校验通过后审计按该 Key
// 记账、会话不落地；他人 Key / 缺参一律拒绝且不触达上游。
//
// 生产同构装配：真实 RegisterAPIRouter + JWT 中间件 + 真实 OpenAI usecase/transport
// 链 + sqlite + miniredis + httptest 上游。子测试共享 fixture，串行执行。
//
//nolint:paralleltest // 共享 sqlite 内存库与上游计数器，子测试必须串行
package playground_attribution

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/config"
	llmproxyservice "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/router"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

const (
	e2eHTTPTimeout = 10 * time.Second
	exposedModel   = "pg-model"
)

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
	stubOpenAIHandler    struct{ handler.OpenAIHandler }
	stubAnthropicHandler struct{ handler.AnthropicHandler }
	stubTriggerHandler   struct{ handler.TriggerHandler }
	stubMetricsHandler   struct{ handler.MetricsHandler }
	stubDatasetHandler   struct{ handler.DatasetHandler }
	stubClientHandler    struct{ handler.ClientHandler }
	stubUpstreamHandler  struct{ handler.UpstreamHandler }
)

// recordingSubmitter 记录审计任务与会话存储任务次数（playground 应只审计、不落会话）。
type recordingSubmitter struct {
	mu     sync.Mutex
	audits []*dto.ModelCallAuditTask
	stores int
}

func (s *recordingSubmitter) SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audits = append(s.audits, task)
	return nil
}

func (s *recordingSubmitter) SubmitMessageStoreTask(_ *dto.MessageStoreTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stores++
	return nil
}

func (s *recordingSubmitter) snapshot() (audits []*dto.ModelCallAuditTask, stores int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*dto.ModelCallAuditTask(nil), s.audits...), s.stores
}

type noopTriggerChecker struct{}

func (noopTriggerChecker) Check(_ string) []uint                           { return nil }
func (noopTriggerChecker) MatchedWords(_ []uint) []string                  { return nil }
func (noopTriggerChecker) DenyIDs(_ []uint) []uint                         { return nil }
func (noopTriggerChecker) OmitIDs(_ []uint) []uint                         { return nil }
func (noopTriggerChecker) CaptureIDs(_ []uint) []uint                      { return nil }
func (noopTriggerChecker) IncrementHits(_ context.Context, _ []uint) error { return nil }

type fixture struct {
	app           *fiber.App
	submitter     *recordingSubmitter
	upstreamCalls *atomic.Int64
	tokenA        string
	keyA          uint
	keyB          uint
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	config.JwtAccessTokenExpired = time.Hour
	config.JwtAccessTokenSecret = "playground-attribution-e2e-secret"

	upstreamCalls := &atomic.Int64{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-pg","object":"chat.completion","model":"up","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
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

	userA := &dbmodel.User{Name: "pg-a", GithubBindID: "gh-pg-a", GoogleBindID: "gg-pg-a", Permission: enum.PermissionUser}
	userB := &dbmodel.User{Name: "pg-b", GithubBindID: "gh-pg-b", GoogleBindID: "gg-pg-b", Permission: enum.PermissionUser}
	for _, u := range []*dbmodel.User{userA, userB} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	keyA := &dbmodel.ProxyAPIKey{UserID: userA.ID, Name: "key-a", Key: "sk-pg-a"}
	keyB := &dbmodel.ProxyAPIKey{UserID: userB.ID, Name: "key-b", Key: "sk-pg-b"}
	for _, k := range []*dbmodel.ProxyAPIKey{keyA, keyB} {
		if err := db.Create(k).Error; err != nil {
			t.Fatalf("create api key: %v", err)
		}
	}
	ep := &dbmodel.Endpoint{UserID: userA.ID, Name: "pg-ep", APIKey: "sk-up", OpenaiBaseURL: upstream.URL, SupportOpenAIChatCompletion: true}
	if err := db.Create(ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	m := &dbmodel.Model{UserID: userA.ID, Alias: exposedModel, ModelID: exposedModel, UpstreamModel: "up", EndpointID: ep.ID, Enabled: true,
		Weight: 1, Capabilities: []enum.InputModality{enum.InputModalityText}}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	httpclient.InitHTTPClient()
	registry := metrics.NewRegistry()
	tracker := inflight.NewTracker()
	guard := transport.NewEndpointGuard(registry)
	submitter := &recordingSubmitter{}
	resolver := llmproxyservice.NewEndpointResolver(repository.NewEndpointRepository(db), repository.NewModelRepository(db), false, nil)
	openAIUC := usecase.NewOpenAIUseCase(resolver, usecase.NewListOpenAIModels(repository.NewEndpointReadRepository(db)),
		transport.NewOpenAIProxy(tracker, guard), transport.NewAnthropicProxy(tracker, guard),
		submitter, noopTriggerChecker{}, metrics.NewTokenUsageCounter(registry), nil)
	playgroundHandler := handler.NewPlaygroundHandler(handler.PlaygroundDependencies{UseCase: openAIUC, SSEGauge: metrics.NewSSEGauge(registry)})

	signer := jwt.NewAccessTokenSigner()
	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("playground attribution", "1.0"))
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
		ModelHandler:      &stubModelHandler{},
		AuditHandler:      &stubAuditHandler{},
		CronHandler:       &stubCronHandler{},
		OpenAIHandler:     &stubOpenAIHandler{},
		PlaygroundHandler: playgroundHandler,
		AnthropicHandler:  &stubAnthropicHandler{},
		TriggerHandler:    &stubTriggerHandler{},
		MetricsHandler:    &stubMetricsHandler{},
		DatasetHandler:    &stubDatasetHandler{},
		ClientHandler:     &stubClientHandler{},
		UpstreamHandler:   &stubUpstreamHandler{},
	})

	tokenA, err := signer.EncodeToken(userA.ID)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	return &fixture{app: app, submitter: submitter, upstreamCalls: upstreamCalls, tokenA: tokenA, keyA: keyA.ID, keyB: keyB.ID}
}

func (f *fixture) chat(t *testing.T, query string) (status int, body []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), e2eHTTPTimeout)
	defer cancel()
	reqBody := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}],"stream":false}`, exposedModel)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/api/web/v1/playground/chat"+query, bytes.NewReader([]byte(reqBody)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+f.tokenA)
	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	resp, err := f.app.Test(req, fiber.TestConfig{Timeout: e2eHTTPTimeout})
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ = io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func TestPlaygroundAPIKeyAttribution(t *testing.T) {
	f := newFixture(t)

	t.Run("自己名下的 Key：转发成功且审计归属该 Key", func(t *testing.T) {
		status, body := f.chat(t, fmt.Sprintf("?apiKeyID=%d", f.keyA))
		if status != http.StatusOK || !bytes.Contains(body, []byte("chatcmpl-pg")) {
			t.Fatalf("status=%d body=%s, want 200 + 上游 completion", status, body)
		}
		audits, stores := f.submitter.snapshot()
		if len(audits) != 1 {
			t.Fatalf("审计条数 = %d, want 1", len(audits))
		}
		if got := util.CtxValueUint(audits[0].Ctx, constant.CtxKeyAPIKeyID); got != f.keyA {
			t.Fatalf("审计 api_key_id = %d, want %d（修复前恒为 0，用户审计视图不可见）", got, f.keyA)
		}
		if stores != 0 {
			t.Fatalf("会话存储任务 = %d, want 0（playground 流量不落会话库）", stores)
		}
	})

	t.Run("他人名下的 Key：拒绝且不触达上游", func(t *testing.T) {
		before := f.upstreamCalls.Load()
		_, body := f.chat(t, fmt.Sprintf("?apiKeyID=%d", f.keyB))
		if !bytes.Contains(body, []byte(`"code":10002`)) {
			t.Fatalf("body=%s, want no_permission(10002)", body)
		}
		if f.upstreamCalls.Load() != before {
			t.Fatal("越权请求不应触达上游")
		}
	})

	t.Run("缺少 apiKeyID：拒绝且不触达上游", func(t *testing.T) {
		before := f.upstreamCalls.Load()
		_, body := f.chat(t, "")
		if !bytes.Contains(body, []byte(`"code":10006`)) {
			t.Fatalf("body=%s, want validation(10006)", body)
		}
		if f.upstreamCalls.Load() != before {
			t.Fatal("缺参请求不应触达上游")
		}
	})
}
