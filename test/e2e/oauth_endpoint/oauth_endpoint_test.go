// Package oauth_endpoint 端到端验证订阅账号接入链路：
// REST 授权发起、upstream 列表凭据投影、订阅端点的转发候选筛选与 mock 上游转发。
//
// 覆盖边界：mock 上游（config.MockUpstreamEnabled + upstream_model=mock-llm）在 transport
// 装饰器层短路，本用例验证「凭据注入 → 候选筛选 → 转发执行」链路；headers/body 特化细节
// 由 test/unit/oauth_upstream 覆盖。真实 OAuth 授权需浏览器交互，作为部署后人工验收项。
package oauth_endpoint

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	endpointquery "github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
	llmproxyport "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	modelcommand "github.com/hcd233/aris-proxy-api/internal/application/model/command"
	upstreamquery "github.com/hcd233/aris-proxy-api/internal/application/upstream/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	llmproxyservice "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	llmvo "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/oauthprovider"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

// noopSubmitter 空实现异步任务提交（本包验证转发执行，审计落库由单测覆盖）
type noopSubmitter struct{}

func (n *noopSubmitter) SubmitModelCallAuditTask(*dto.ModelCallAuditTask) error { return nil }
func (n *noopSubmitter) SubmitMessageStoreTask(*dto.MessageStoreTask) error     { return nil }

// e2eStubHandlers 嵌入接口空结构体：注册期满足方法集、不解引用
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

var dbSeq atomic.Uint64

type fixture struct {
	db             *gorm.DB
	app            *fiber.App
	signer         jwt.TokenSigner
	user           *dbmodel.User
	endpointRepo   llmproxy.EndpointRepository
	oauthEndpoints llmproxy.OAuthEndpointRepository
	credRepo       llmproxy.OAuthCredentialRepository
	modelRepo      llmproxy.ModelRepository
	openAI         llmproxyport.OpenAIUseCase
	anthropic      llmproxyport.AnthropicUseCase
}

// newFixture 组装完整链路：sqlite 仓储 + miniredis + 生产路由 + mock 上游
func newFixture(t *testing.T) *fixture {
	t.Helper()
	config.OAuthCredentialKey = []byte("0123456789abcdef0123456789abcdef")
	config.MockUpstreamEnabled = true
	config.JwtAccessTokenExpired = time.Hour
	config.JwtAccessTokenSecret = "oauth-e2e-secret"
	config.UpstreamBulkheadEnabled = false

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
	if err := db.AutoMigrate(&dbmodel.User{}, &dbmodel.Endpoint{}, &dbmodel.Model{}, &dbmodel.ProxyAPIKey{}, &dbmodel.EndpointOAuthCredential{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	endpointRepo := repository.NewEndpointRepository(db)
	oauthEndpointRepo := repository.NewOAuthEndpointRepository(db, cache.NewReadCache(rdb))
	credRepo := repository.NewOAuthCredentialRepository(db)
	modelRepo := repository.NewModelRepository(db)
	userRepo := repository.NewUserRepository(db)

	flowStore := cache.NewOAuthFlowCache(rdb)
	clients := oauthprovider.NewClients()
	tokens := oauthprovider.NewOAuthTokenProvider(credRepo, clients, cache.NewOAuthAccessCache(rdb), lock.NewLocker(rdb))

	endpointHandler := handler.NewEndpointHandler(handler.EndpointDependencies{
		Create:   endpointcommand.NewCreateEndpointHandler(endpointRepo),
		Update:   endpointcommand.NewUpdateEndpointHandler(endpointRepo),
		Delete:   endpointcommand.NewDeleteEndpointHandler(endpointRepo),
		OAuth:    endpointcommand.NewOAuthAuthorizeHandler(flowStore, clients),
		Exchange: endpointcommand.NewOAuthExchangeHandler(flowStore, clients, oauthEndpointRepo),
		Reauth:   endpointcommand.NewOAuthReauthHandler(flowStore, clients, endpointRepo, credRepo, cache.NewOAuthAccessCache(rdb)),
		Quota:    endpointquery.NewGetOAuthQuotaHandler(endpointRepo, credRepo),
		Probe:    endpointcommand.NewOAuthProbeHandler(endpointRepo, credRepo, tokens, clients),
	})
	modelHandler := handler.NewModelHandler(handler.ModelDependencies{
		Create: modelcommand.NewCreateModelHandler(endpointRepo, modelRepo),
		Update: modelcommand.NewUpdateModelHandler(endpointRepo, modelRepo),
		Delete: modelcommand.NewDeleteModelHandler(modelRepo, endpointRepo),
	})
	upstreamHandler := handler.NewUpstreamHandler(handler.UpstreamDependencies{
		List: upstreamquery.NewListUpstreamHandler(endpointRepo, modelRepo, userRepo, credRepo),
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("oauth endpoint e2e", "1.0"))
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

	user := &dbmodel.User{Name: "oauth-user", GithubBindID: "gh-oauth", GoogleBindID: "gg-oauth", Permission: enum.PermissionUser}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 转发链路 usecase（mock 上游装饰器短路，不触达网络）
	resolver := llmproxyservice.NewEndpointResolver(endpointRepo, modelRepo, false, nil)
	readRepo := repository.NewEndpointReadRepository(db)
	guard := transport.NewEndpointGuard(nil)
	tracker := inflight.NewTracker()
	openAIProxy := transport.NewMockOpenAIProxy(transport.NewOpenAIProxy(tracker, guard))
	anthropicProxy := transport.NewMockAnthropicProxy(transport.NewAnthropicProxy(tracker, guard))
	submitter := &noopSubmitter{}

	return &fixture{
		db:             db,
		app:            app,
		signer:         jwt.NewAccessTokenSigner(),
		user:           user,
		endpointRepo:   endpointRepo,
		oauthEndpoints: oauthEndpointRepo,
		credRepo:       credRepo,
		modelRepo:      modelRepo,
		openAI: usecase.NewOpenAIUseCase(
			resolver,
			usecase.NewListOpenAIModels(readRepo),
			openAIProxy,
			anthropicProxy,
			submitter, nil, nil, nil, tokens,
		),
		anthropic: usecase.NewAnthropicUseCase(
			resolver,
			usecase.NewListAnthropicModels(readRepo),
			usecase.NewCountTokens(readRepo, anthropicProxy),
			anthropicProxy,
			openAIProxy,
			submitter, nil, nil, nil, tokens,
		),
	}
}

func (f *fixture) token(t *testing.T) string {
	t.Helper()
	token, err := f.signer.EncodeToken(f.user.ID)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	return token
}

func (f *fixture) do(t *testing.T, method, path, body string) (status int, data []byte) {
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
		req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+f.token(t))
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

// goCtx 构造带请求用户上下文的 ctx（LLM usecase 从 ctx 读 CtxKeyUserID）
func (f *fixture) goCtx(t *testing.T) context.Context {
	t.Helper()
	ctx := context.WithValue(t.Context(), constant.CtxKeyUserID, f.user.ID)
	return context.WithValue(ctx, constant.CtxKeyPermission, enum.PermissionUser)
}

// seedOAuthEndpoint 经真实仓储原子创建 OAuth 端点与凭据，并绑定模型别名（upstream_model=mock-llm）
func (f *fixture) seedOAuthEndpoint(t *testing.T, provider enum.OAuthProvider, alias string) uint {
	t.Helper()

	var (
		ep  *llmagg.Endpoint
		err error
	)
	switch provider {
	case enum.OAuthProviderAnthropic:
		ep, err = llmagg.CreateEndpoint(0, "claude-sub", "", constant.OAuthClaudeUpstreamBaseURL, "", false, false, true, false, enum.EndpointAuthOAuthAnthropic)
	case enum.OAuthProviderOpenAI:
		ep, err = llmagg.CreateEndpoint(0, "codex-sub", constant.OAuthOpenAIUpstreamBaseURL, "", "", false, true, false, false, enum.EndpointAuthOAuthOpenAI)
	}
	if err != nil {
		t.Fatalf("create endpoint aggregate: %v", err)
	}
	cred, err := llmagg.CreateOAuthCredential(0, provider, "at-e2e", "rt-e2e", time.Now().Add(time.Hour), "org-e2e", "e2e@example.com", "max")
	if err != nil {
		t.Fatalf("create credential aggregate: %v", err)
	}
	endpointID, err := f.oauthEndpoints.CreateWithOAuthCredential(t.Context(), ep, cred, f.user.ID)
	if err != nil {
		t.Fatalf("create oauth endpoint: %v", err)
	}

	model, err := llmagg.CreateModel(0, llmvo.EndpointAlias(alias), constant.MockUpstreamModel, endpointID, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("create model aggregate: %v", err)
	}
	if _, err := f.modelRepo.Create(t.Context(), model, f.user.ID); err != nil {
		t.Fatalf("create model: %v", err)
	}
	return endpointID
}

// TestAuthorizeOAuthEndpointREST 授权发起：REST 返回含 PKCE challenge 的授权链接与 state。
func TestAuthorizeOAuthEndpointREST(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	status, data := f.do(t, http.MethodPost, "/api/web/v1/endpoint/oauth/authorize", `{"provider":"anthropic"}`)
	if status != http.StatusOK {
		t.Fatalf("authorize status=%d body=%s", status, data)
	}
	var rsp struct {
		AuthorizeURL string `json:"authorizeURL"`
		State        string `json:"state"`
	}
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("decode: %v (raw=%s)", err, data)
	}
	url := rsp.AuthorizeURL
	if !strings.Contains(url, "code_challenge=") || !strings.Contains(url, "state=") || !strings.Contains(url, "code_challenge_method=S256") {
		t.Fatalf("authorize url missing PKCE params: %s", url)
	}
	if rsp.State == "" {
		t.Fatal("state must be returned for the follow-up exchange")
	}
}

// TestUpstreamListExposesOAuthView 列表投影：OAuth 端点携带 authType 与凭据展示字段（token 永不下发）。
func TestUpstreamListExposesOAuthView(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.seedOAuthEndpoint(t, enum.OAuthProviderAnthropic, "claude-list")

	status, data := f.do(t, http.MethodGet, "/api/web/v1/upstream/list?page=1&pageSize=10", "")
	if status != http.StatusOK {
		t.Fatalf("upstream list status=%d body=%s", status, data)
	}
	var rsp struct {
		Groups []struct {
			Endpoint struct {
				AuthType string `json:"authType"`
				OAuth    *struct {
					AccountEmail string `json:"accountEmail"`
					Status       string `json:"status"`
					Quota        *struct {
						Windows []struct {
							Key         string  `json:"key"`
							UsedPercent float64 `json:"usedPercent"`
						} `json:"windows"`
					} `json:"quota"`
				} `json:"oauth"`
			} `json:"endpoint"`
		} `json:"groups"`
	}
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("decode: %v (raw=%s)", err, data)
	}
	if len(rsp.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d (raw=%s)", len(rsp.Groups), data)
	}
	ep := rsp.Groups[0].Endpoint
	if ep.AuthType != string(enum.EndpointAuthOAuthAnthropic) {
		t.Fatalf("authType = %q", ep.AuthType)
	}
	if ep.OAuth == nil || ep.OAuth.Status != string(enum.OAuthCredentialStatusActive) || ep.OAuth.AccountEmail != "e2e@example.com" {
		t.Fatalf("oauth view wrong: %+v", ep.OAuth)
	}
}

// TestAnthropicForwardThroughOAuthEndpoint active 订阅端点经凭据注入正常转发（mock 上游）。
func TestAnthropicForwardThroughOAuthEndpoint(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.seedOAuthEndpoint(t, enum.OAuthProviderAnthropic, "claude-fwd")

	var req dto.AnthropicCreateMessageRequest
	if err := sonic.Unmarshal([]byte(`{"body":{"model":"claude-fwd","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}}`), &req); err != nil {
		t.Fatalf("build request: %v", err)
	}
	result, err := f.anthropic.CreateMessage(f.goCtx(t), &req)
	if err != nil {
		t.Fatalf("forward through oauth endpoint: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
}

// TestReauthRequiredEndpointExcludedFromForwarding 凭据失效后端点从候选剔除（404 语义）。
func TestReauthRequiredEndpointExcludedFromForwarding(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	endpointID := f.seedOAuthEndpoint(t, enum.OAuthProviderAnthropic, "claude-reauth")

	cred, err := f.credRepo.FindByEndpointID(t.Context(), endpointID)
	if err != nil || cred == nil {
		t.Fatalf("find credential: %v", err)
	}
	cred.MarkReauth("invalid_grant")
	if err := f.credRepo.Save(t.Context(), cred); err != nil {
		t.Fatalf("save credential: %v", err)
	}

	var req dto.AnthropicCreateMessageRequest
	if err := sonic.Unmarshal([]byte(`{"body":{"model":"claude-reauth","max_tokens":8,"messages":[{"role":"user","content":"hi"}]}}`), &req); err != nil {
		t.Fatalf("build request: %v", err)
	}
	if _, err := f.anthropic.CreateMessage(f.goCtx(t), &req); err == nil {
		t.Fatal("reauth_required endpoint must not be resolvable (expected model not found)")
	}
}

// TestCodexEndpointRequiresStream Codex 订阅端点仅接受 stream=true：非流式请求无候选（404），流式正常。
func TestCodexEndpointRequiresStream(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.seedOAuthEndpoint(t, enum.OAuthProviderOpenAI, "codex-fwd")

	var nonStream dto.OpenAICreateResponseRequest
	if err := sonic.Unmarshal([]byte(`{"body":{"model":"codex-fwd","stream":false,"input":"hi"}}`), &nonStream); err != nil {
		t.Fatalf("build request: %v", err)
	}
	if _, err := f.openAI.CreateResponse(f.goCtx(t), &nonStream); err == nil {
		t.Fatal("non-stream request must not reach a codex oauth endpoint")
	}

	var streamReq dto.OpenAICreateResponseRequest
	if err := sonic.Unmarshal([]byte(`{"body":{"model":"codex-fwd","stream":true,"input":"hi"}}`), &streamReq); err != nil {
		t.Fatalf("build request: %v", err)
	}
	if _, err := f.openAI.CreateResponse(f.goCtx(t), &streamReq); err != nil {
		t.Fatalf("stream request through codex endpoint: %v", err)
	}
}
