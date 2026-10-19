package oauth_flow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/oauthprovider"
)

// ── fake provider ──

// fakeProviderClient 可编程 provider（记录调用参数并返回预置结果）
type fakeProviderClient struct {
	provider     enum.OAuthProvider
	exchangeErr  error
	tokenSet     *service.OAuthTokenSet
	gotCode      string
	gotVerifier  string
	gotState     string
	exchangeCall int
}

func newFakeProvider() *fakeProviderClient {
	return &fakeProviderClient{
		provider: enum.OAuthProviderAnthropic,
		tokenSet: &service.OAuthTokenSet{
			AccessToken:  "at-new",
			RefreshToken: "rt-new",
			ExpiresAt:    time.Now().Add(time.Hour),
			AccountRef:   "org-1",
			AccountEmail: "alice@example.com",
			PlanType:     "max",
		},
	}
}

func (f *fakeProviderClient) Provider() enum.OAuthProvider { return f.provider }
func (f *fakeProviderClient) AuthorizeURL(_, _ string) string {
	return "https://authorize.example.com"
}

func (f *fakeProviderClient) ExchangeCode(_ context.Context, code, verifier, state string) (*service.OAuthTokenSet, error) {
	f.exchangeCall++
	f.gotCode, f.gotVerifier, f.gotState = code, verifier, state
	if f.exchangeErr != nil {
		return nil, f.exchangeErr
	}
	return f.tokenSet, nil
}

func (f *fakeProviderClient) Refresh(context.Context, string) (*service.OAuthTokenSet, error) {
	return f.tokenSet, nil
}

func (f *fakeProviderClient) ProbeQuota(context.Context, string, string) (*vo.QuotaSnapshot, error) {
	return &vo.QuotaSnapshot{Windows: []vo.QuotaWindow{}}, nil
}

func newProviderMap(provider *fakeProviderClient) map[enum.OAuthProvider]service.OAuthProviderClient {
	clients := oauthprovider.NewClients()
	clients[provider.Provider()] = provider
	return clients
}

// ── fake 仓储 ──

// fakeOAuthEndpointRepo 记录 CreateWithOAuthCredential 入参
type fakeOAuthEndpointRepo struct {
	createdEndpoint *aggregate.Endpoint
	createdCred     *aggregate.OAuthCredential
	createCall      int
	createErr       error
}

func (f *fakeOAuthEndpointRepo) CreateWithOAuthCredential(_ context.Context, endpoint *aggregate.Endpoint, credential *aggregate.OAuthCredential, _ uint) (uint, error) {
	f.createCall++
	f.createdEndpoint, f.createdCred = endpoint, credential
	if f.createErr != nil {
		return 0, f.createErr
	}
	return 101, nil
}

var _ llmproxy.OAuthEndpointRepository = (*fakeOAuthEndpointRepo)(nil)

// fakeEndpointFinder 提供 FindByID（reauth 用）
type fakeEndpointFinder struct {
	*stubEndpointRepoForOAuth
	endpoint *aggregate.Endpoint
}

func (f *fakeEndpointFinder) FindByID(context.Context, uint, *uint) (*aggregate.Endpoint, error) {
	return f.endpoint, nil
}

// stubEndpointRepoForOAuth 空实现其余接口方法（嵌入共享）
type stubEndpointRepoForOAuth struct{}

func (s *stubEndpointRepoForOAuth) FindByID(context.Context, uint, *uint) (*aggregate.Endpoint, error) {
	return nil, nil
}
func (s *stubEndpointRepoForOAuth) BatchFindByIDs(context.Context, []uint) (map[uint]*aggregate.Endpoint, error) {
	return map[uint]*aggregate.Endpoint{}, nil
}
func (s *stubEndpointRepoForOAuth) Create(context.Context, *aggregate.Endpoint, uint) (uint, error) {
	return 0, nil
}
func (s *stubEndpointRepoForOAuth) Update(context.Context, *aggregate.Endpoint) error { return nil }
func (s *stubEndpointRepoForOAuth) Delete(context.Context, uint, *uint) error         { return nil }
func (s *stubEndpointRepoForOAuth) DeleteCascade(context.Context, uint, *uint) error  { return nil }
func (s *stubEndpointRepoForOAuth) List(context.Context) ([]*aggregate.Endpoint, error) {
	return nil, nil
}
func (s *stubEndpointRepoForOAuth) Paginate(context.Context, commonmodel.CommonParam, *uint) ([]*aggregate.Endpoint, *commonmodel.PageInfo, error) {
	return nil, nil, nil
}
func (s *stubEndpointRepoForOAuth) FindIDsByScope(context.Context, *uint) ([]uint, error) {
	return nil, nil
}

// fakeOAuthCredRepo 内存凭据仓储
type fakeOAuthCredRepo struct {
	cred      *aggregate.OAuthCredential
	savedCall int
}

func (f *fakeOAuthCredRepo) FindByEndpointID(context.Context, uint) (*aggregate.OAuthCredential, error) {
	return f.cred, nil
}

func (f *fakeOAuthCredRepo) Save(_ context.Context, cred *aggregate.OAuthCredential) error {
	f.savedCall++
	f.cred = cred
	return nil
}

func (f *fakeOAuthCredRepo) ListActive(context.Context) ([]*aggregate.OAuthCredential, error) {
	return nil, nil
}

func (f *fakeOAuthCredRepo) ListDisplayByEndpointIDs(context.Context, []uint) (map[uint]vo.OAuthCredentialDisplay, error) {
	return nil, nil
}

func (f *fakeOAuthCredRepo) UpdateQuota(context.Context, uint, vo.QuotaSnapshot, time.Time) error {
	return nil
}

func (f *fakeOAuthCredRepo) UpdateLastError(context.Context, uint, string) error { return nil }

var _ llmproxy.OAuthCredentialRepository = (*fakeOAuthCredRepo)(nil)

// fakeReauthAccessCache 记录 Delete 调用（C1：重新授权必须失效 access 缓存）
type fakeReauthAccessCache struct {
	deleteCalls []uint
}

func (f *fakeReauthAccessCache) Get(context.Context, uint) (service.OAuthAccess, bool) {
	return service.OAuthAccess{}, false
}

func (f *fakeReauthAccessCache) Put(context.Context, uint, service.OAuthAccess, time.Duration) {}

func (f *fakeReauthAccessCache) Delete(_ context.Context, endpointID uint) {
	f.deleteCalls = append(f.deleteCalls, endpointID)
}

var _ service.OAuthAccessCache = (*fakeReauthAccessCache)(nil)

// ── 测试 ──

// TestExchangeCreatesOAuthEndpointAndCredential 成功路径：固定连接字段写入 + 凭据落库。
func TestExchangeCreatesOAuthEndpointAndCredential(t *testing.T) {
	t.Parallel()

	store := newFakeFlowStore()
	provider := newFakeProvider()
	endpointRepo := &fakeOAuthEndpointRepo{}
	handler := command.NewOAuthExchangeHandler(store, newProviderMap(provider), endpointRepo)
	if err := store.Put(t.Context(), "st-1", vo.OAuthFlow{UserID: 9, Provider: enum.OAuthProviderAnthropic, Verifier: "v-1", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("seed flow: %v", err)
	}

	result, err := handler.Handle(t.Context(), port.ExchangeOAuthCommand{
		OperatorUserID: 9,
		OwnerUserID:    9,
		Name:           "claude-sub",
		Provider:       enum.OAuthProviderAnthropic,
		CodeInput:      "ac-1#st-1",
	})
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if result.EndpointID != 101 || endpointRepo.createCall != 1 {
		t.Fatalf("endpoint not created: result=%+v calls=%d", result, endpointRepo.createCall)
	}
	if provider.gotCode != "ac-1" || provider.gotVerifier != "v-1" || provider.gotState != "st-1" {
		t.Fatalf("provider got (%q,%q,%q)", provider.gotCode, provider.gotVerifier, provider.gotState)
	}

	ep := endpointRepo.createdEndpoint
	if ep.AuthType() != enum.EndpointAuthOAuthAnthropic {
		t.Fatalf("auth type = %q", ep.AuthType())
	}
	if ep.AnthropicBaseURL() != constant.OAuthClaudeUpstreamBaseURL || !ep.SupportAnthropicMessage() {
		t.Fatalf("oauth endpoint fixed fields wrong: base=%q message=%v", ep.AnthropicBaseURL(), ep.SupportAnthropicMessage())
	}
	if ep.OpenaiBaseURL() != "" || ep.SupportOpenAIChatCompletion() || ep.SupportOpenAIResponse() {
		t.Fatal("oauth_anthropic endpoint must not enable openai protocols")
	}

	cred := endpointRepo.createdCred
	if cred.AccessToken() != "at-new" || cred.RefreshToken() != "rt-new" || cred.AccountRef() != "org-1" {
		t.Fatalf("credential not stored: %+v", cred)
	}
}

// TestExchangeRejectsForeignFlow state 属于他人时拒绝（跨用户防护）。
func TestExchangeRejectsForeignFlow(t *testing.T) {
	t.Parallel()

	store := newFakeFlowStore()
	provider := newFakeProvider()
	handler := command.NewOAuthExchangeHandler(store, newProviderMap(provider), &fakeOAuthEndpointRepo{})
	if err := store.Put(t.Context(), "st-2", vo.OAuthFlow{UserID: 1, Provider: enum.OAuthProviderAnthropic, Verifier: "v", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("seed flow: %v", err)
	}

	_, err := handler.Handle(t.Context(), port.ExchangeOAuthCommand{
		OperatorUserID: 9, OwnerUserID: 9, Name: "x", Provider: enum.OAuthProviderAnthropic, CodeInput: "ac#st-2",
	})
	if !errors.Is(err, ierr.ErrNoPermission) {
		t.Fatalf("want ErrNoPermission, got %v", err)
	}
}

// TestExchangeRejectsMissingFlow flow 不存在/过期时拒绝。
func TestExchangeRejectsMissingFlow(t *testing.T) {
	t.Parallel()

	handler := command.NewOAuthExchangeHandler(newFakeFlowStore(), newProviderMap(newFakeProvider()), &fakeOAuthEndpointRepo{})
	_, err := handler.Handle(t.Context(), port.ExchangeOAuthCommand{
		OperatorUserID: 9, OwnerUserID: 9, Name: "x", Provider: enum.OAuthProviderAnthropic, CodeInput: "ac#nope",
	})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

// TestExchangeProviderFailurePropagates 换 token 失败时不创建任何行。
func TestExchangeProviderFailurePropagates(t *testing.T) {
	t.Parallel()

	store := newFakeFlowStore()
	provider := newFakeProvider()
	provider.exchangeErr = &service.OAuthProtocolError{StatusCode: 400, Reason: "invalid_grant"}
	repo := &fakeOAuthEndpointRepo{}
	handler := command.NewOAuthExchangeHandler(store, newProviderMap(provider), repo)
	if err := store.Put(t.Context(), "st-3", vo.OAuthFlow{UserID: 9, Provider: enum.OAuthProviderAnthropic, Verifier: "v", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("seed flow: %v", err)
	}

	_, err := handler.Handle(t.Context(), port.ExchangeOAuthCommand{
		OperatorUserID: 9, OwnerUserID: 9, Name: "x", Provider: enum.OAuthProviderAnthropic, CodeInput: "ac#st-3",
	})
	if err == nil {
		t.Fatal("provider failure must propagate")
	}
	if repo.createCall != 0 {
		t.Fatal("no rows may be created when exchange fails")
	}
}

// TestReauthUpdatesExistingCredential 重新授权：凭据更新且回到 active。
func TestReauthUpdatesExistingCredential(t *testing.T) {
	t.Parallel()

	store := newFakeFlowStore()
	provider := newFakeProvider()
	ep, err := aggregate.CreateEndpoint(5, "claude-sub", "", constant.OAuthClaudeUpstreamBaseURL, "", false, false, true, false, enum.EndpointAuthOAuthAnthropic)
	if err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	existing, err := aggregate.CreateOAuthCredential(5, enum.OAuthProviderAnthropic, "at-old", "rt-old", time.Now(), "org-1", "a@b.c", "max")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	existing.MarkReauth("invalid_grant")

	ender := &fakeEndpointFinder{stubEndpointRepoForOAuth: &stubEndpointRepoForOAuth{}, endpoint: ep}
	credRepo := &fakeOAuthCredRepo{cred: existing}
	accessCache := &fakeReauthAccessCache{}
	handler := command.NewOAuthReauthHandler(store, newProviderMap(provider), ender, credRepo, accessCache)
	if err := store.Put(t.Context(), "st-4", vo.OAuthFlow{UserID: 9, Provider: enum.OAuthProviderAnthropic, Verifier: "v-4", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("seed flow: %v", err)
	}

	if err := handler.Handle(t.Context(), port.ReauthOAuthCommand{
		OperatorUserID: 9,
		EndpointID:     5,
		CodeInput:      "ac-4#st-4",
	}); err != nil {
		t.Fatalf("reauth: %v", err)
	}
	if credRepo.savedCall != 1 {
		t.Fatalf("credential not saved: %d", credRepo.savedCall)
	}
	if credRepo.cred.AccessToken() != "at-new" || credRepo.cred.Status() != enum.OAuthCredentialStatusActive || credRepo.cred.LastError() != "" {
		t.Fatalf("credential not refreshed: status=%q err=%q", credRepo.cred.Status(), credRepo.cred.LastError())
	}
	// C1：旧 access token 可能已被吊销，重新授权必须失效缓存（否则死 token 在 TTL 内继续被使用）
	if len(accessCache.deleteCalls) != 1 || accessCache.deleteCalls[0] != 5 {
		t.Fatalf("reauth must invalidate access cache for endpoint 5, got %v", accessCache.deleteCalls)
	}
}

// TestReauthRejectsNonOAuthEndpoint 非 OAuth 端点拒绝重新授权。
func TestReauthRejectsNonOAuthEndpoint(t *testing.T) {
	t.Parallel()

	store := newFakeFlowStore()
	provider := newFakeProvider()
	apiKeyEp, err := aggregate.CreateEndpoint(6, "apikey", "https://o.example.com", "", "sk-x", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	ender := &fakeEndpointFinder{stubEndpointRepoForOAuth: &stubEndpointRepoForOAuth{}, endpoint: apiKeyEp}
	handler := command.NewOAuthReauthHandler(store, newProviderMap(provider), ender, &fakeOAuthCredRepo{}, &fakeReauthAccessCache{})
	if err := store.Put(t.Context(), "st-5", vo.OAuthFlow{UserID: 9, Provider: enum.OAuthProviderAnthropic, Verifier: "v", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("seed flow: %v", err)
	}

	err = handler.Handle(t.Context(), port.ReauthOAuthCommand{OperatorUserID: 9, EndpointID: 6, CodeInput: "ac#st-5"})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}
