// Package oauth_token 验证订阅账号 access token 提供者：缓存命中、刷新/标记、可切换错误。
package oauth_token

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/oauthprovider"
)

// ── fakes ──

type fakeCredRepo struct {
	cred       *aggregate.OAuthCredential
	second     *aggregate.OAuthCredential // 非 nil 时第二次 Find 返回它（锁内双检场景）
	findCall   int
	saveCall   int
	saveCtxErr error // Save 时 ctx 的状态（验证落库不继承已取消的请求 ctx）
}

func (f *fakeCredRepo) FindByEndpointID(context.Context, uint) (*aggregate.OAuthCredential, error) {
	f.findCall++
	if f.findCall > 1 && f.second != nil {
		return f.second, nil
	}
	return f.cred, nil
}

func (f *fakeCredRepo) Save(ctx context.Context, cred *aggregate.OAuthCredential) error {
	f.saveCall++
	f.saveCtxErr = ctx.Err()
	f.cred = cred
	return nil
}

func (f *fakeCredRepo) ListActive(context.Context) ([]*aggregate.OAuthCredential, error) {
	return nil, nil
}

func (f *fakeCredRepo) ListDisplayByEndpointIDs(context.Context, []uint) (map[uint]vo.OAuthCredentialDisplay, error) {
	return nil, nil
}

func (f *fakeCredRepo) UpdateQuota(context.Context, uint, vo.QuotaSnapshot, time.Time) error {
	return nil
}

func (f *fakeCredRepo) UpdateLastError(context.Context, uint, string) error { return nil }

type fakeAccessCache struct {
	access      service.OAuthAccess
	present     bool
	putCall     int
	lastTTL     time.Duration
	getCalled   int
	deleteCalls []uint
}

func (f *fakeAccessCache) Get(context.Context, uint) (service.OAuthAccess, bool) {
	f.getCalled++
	return f.access, f.present
}

func (f *fakeAccessCache) Put(_ context.Context, _ uint, access service.OAuthAccess, ttl time.Duration) {
	f.putCall++
	f.access, f.present, f.lastTTL = access, true, ttl
}

func (f *fakeAccessCache) Delete(_ context.Context, endpointID uint) {
	f.deleteCalls = append(f.deleteCalls, endpointID)
}

type fakeLocker struct {
	locked bool
	err    error
}

func (f *fakeLocker) Lock(context.Context, string, string, time.Duration) (bool, error) {
	return f.locked, f.err
}
func (f *fakeLocker) Refresh(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeLocker) Unlock(context.Context, string, string) error { return nil }

type fakeRefreshClient struct {
	provider    enum.OAuthProvider
	refreshErr  error
	tokenSet    *service.OAuthTokenSet
	refreshCall int
	onRefresh   func() // 刷新期间的外部事件钩子（如模拟客户端取消请求）
}

func (f *fakeRefreshClient) Provider() enum.OAuthProvider { return f.provider }
func (f *fakeRefreshClient) AuthorizeURL(_, _ string) string {
	return "https://authorize.example.com"
}

func (f *fakeRefreshClient) ExchangeCode(context.Context, string, string, string) (*service.OAuthTokenSet, error) {
	return f.tokenSet, nil
}

func (f *fakeRefreshClient) Refresh(context.Context, string) (*service.OAuthTokenSet, error) {
	f.refreshCall++
	if f.onRefresh != nil {
		f.onRefresh()
	}
	if f.refreshErr != nil {
		return nil, f.refreshErr
	}
	return f.tokenSet, nil
}

func (f *fakeRefreshClient) ProbeQuota(context.Context, string, string) (*vo.QuotaSnapshot, error) {
	return &vo.QuotaSnapshot{Windows: []vo.QuotaWindow{}}, nil
}

func newTokenProvider(credRepo *fakeCredRepo, cache *fakeAccessCache, locker *fakeLocker, client *fakeRefreshClient) service.OAuthTokenProvider {
	clients := oauthprovider.NewClients()
	clients[client.Provider()] = client
	return oauthprovider.NewOAuthTokenProvider(credRepo, clients, cache, locker)
}

func newCredential(t *testing.T, expiresAt time.Time, status enum.OAuthCredentialStatus) *aggregate.OAuthCredential {
	t.Helper()
	cred, err := aggregate.CreateOAuthCredential(5, enum.OAuthProviderAnthropic, "at-old", "rt-old", expiresAt, "org-1", "a@b.c", "max")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	if status == enum.OAuthCredentialStatusReauthRequired {
		cred.MarkReauth("invalid_grant")
	}
	return cred
}

// ── 测试 ──

// TestCacheHitSkipsRepo 缓存命中时不查库、不刷新。
func TestCacheHitSkipsRepo(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{}
	cache := &fakeAccessCache{present: true, access: service.OAuthAccess{Token: "cached", AccountRef: "org-1", ExpiresAt: time.Now().Add(time.Hour)}}
	client := &fakeRefreshClient{provider: enum.OAuthProviderAnthropic}
	provider := newTokenProvider(credRepo, cache, &fakeLocker{locked: true}, client)

	access, err := provider.GetAccessToken(t.Context(), 5)
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if access.Token != "cached" || access.AccountRef != "org-1" {
		t.Fatalf("unexpected access: %+v", access)
	}
	if client.refreshCall != 0 {
		t.Fatal("cache hit must not refresh")
	}
}

// TestExpiringTokenRefreshesOnce 临期（3 分钟内）触发刷新：落库 + 写缓存。
func TestExpiringTokenRefreshesOnce(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(time.Minute), enum.OAuthCredentialStatusActive)}
	cache := &fakeAccessCache{}
	client := &fakeRefreshClient{
		provider: enum.OAuthProviderAnthropic,
		tokenSet: &service.OAuthTokenSet{AccessToken: "at-new", RefreshToken: "", ExpiresAt: time.Now().Add(time.Hour)},
	}
	provider := newTokenProvider(credRepo, cache, &fakeLocker{locked: true}, client)

	access, err := provider.GetAccessToken(t.Context(), 5)
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if client.refreshCall != 1 || credRepo.saveCall != 1 || cache.putCall != 1 {
		t.Fatalf("refresh=%d save=%d cachePut=%d", client.refreshCall, credRepo.saveCall, cache.putCall)
	}
	if access.Token != "at-new" {
		t.Fatalf("token = %q", access.Token)
	}
	// 空 refresh 保留旧值（轮换语义）
	if credRepo.cred.RefreshToken() != "rt-old" {
		t.Fatalf("refresh token must be preserved on empty rotation, got %q", credRepo.cred.RefreshToken())
	}
}

// TestInvalidGrantMarksReauth invalid_grant 标记需重授权并返回可切换错误。
func TestInvalidGrantMarksReauth(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(time.Minute), enum.OAuthCredentialStatusActive)}
	cache := &fakeAccessCache{}
	client := &fakeRefreshClient{
		provider:   enum.OAuthProviderAnthropic,
		refreshErr: &service.OAuthProtocolError{StatusCode: 400, Reason: "invalid_grant"},
	}
	provider := newTokenProvider(credRepo, cache, &fakeLocker{locked: true}, client)

	_, err := provider.GetAccessToken(t.Context(), 5)
	var oauthErr *model.OAuthCredentialError
	if !errors.As(err, &oauthErr) {
		t.Fatalf("want OAuthCredentialError, got %v", err)
	}
	if credRepo.saveCall != 1 || credRepo.cred.Status() != enum.OAuthCredentialStatusReauthRequired {
		t.Fatalf("credential must be marked reauth_required: save=%d status=%q", credRepo.saveCall, credRepo.cred.Status())
	}
	// 回归 I2：凭据不可用必须同时失效 access 缓存，否则旧 token 在剩余 TTL 内仍会被命中转发
	if len(cache.deleteCalls) != 1 || cache.deleteCalls[0] != 5 {
		t.Fatalf("reauth must invalidate access cache, delete calls = %v", cache.deleteCalls)
	}
	if !usecase.CanSwitchEndpoint(err) {
		t.Fatal("OAuthCredentialError must be switchable")
	}
}

// TestNetworkFailureKeepsCredential 网络类失败不标记 reauth（靠 cron 重试）。
func TestNetworkFailureKeepsCredential(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(time.Minute), enum.OAuthCredentialStatusActive)}
	cache := &fakeAccessCache{}
	client := &fakeRefreshClient{
		provider:   enum.OAuthProviderAnthropic,
		refreshErr: ierr.New(ierr.ErrProxyRequest, "connection reset"),
	}
	provider := newTokenProvider(credRepo, cache, &fakeLocker{locked: true}, client)

	_, err := provider.GetAccessToken(t.Context(), 5)
	var oauthErr *model.OAuthCredentialError
	if !errors.As(err, &oauthErr) {
		t.Fatalf("want OAuthCredentialError, got %v", err)
	}
	if credRepo.saveCall != 0 || credRepo.cred.Status() != enum.OAuthCredentialStatusActive {
		t.Fatalf("network failure must keep credential active: save=%d status=%q", credRepo.saveCall, credRepo.cred.Status())
	}
}

// TestReauthRequiredFastFails reauth_required 凭据直接快速失败（不尝试刷新）。
func TestReauthRequiredFastFails(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(time.Hour), enum.OAuthCredentialStatusReauthRequired)}
	cache := &fakeAccessCache{}
	client := &fakeRefreshClient{provider: enum.OAuthProviderAnthropic}
	provider := newTokenProvider(credRepo, cache, &fakeLocker{locked: true}, client)

	_, err := provider.GetAccessToken(t.Context(), 5)
	var oauthErr *model.OAuthCredentialError
	if !errors.As(err, &oauthErr) {
		t.Fatalf("want OAuthCredentialError, got %v", err)
	}
	if client.refreshCall != 0 {
		t.Fatal("reauth_required must not attempt refresh")
	}
}

// TestRefreshPersistsTokensAfterClientCancel 客户端取消请求后仍必须落库轮换后的 token。
//
// 回归：用请求 ctx 刷新/落库时，用户中断（Claude Code / Codex 常见）会导致
// 「上游已轮换 refresh token，新 token 却没存下来」——旧 refresh token 随即失效，凭据报废。
func TestRefreshPersistsTokensAfterClientCancel(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(time.Minute), enum.OAuthCredentialStatusActive)}
	cache := &fakeAccessCache{}
	client := &fakeRefreshClient{
		provider: enum.OAuthProviderAnthropic,
		tokenSet: &service.OAuthTokenSet{AccessToken: "at-new", RefreshToken: "rt-new", ExpiresAt: time.Now().Add(time.Hour)},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// 模拟刷新进行中用户断开连接
	client.onRefresh = cancel

	access, err := newTokenProvider(credRepo, cache, &fakeLocker{locked: true}, client).GetAccessToken(ctx, 5)
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if access.Token != "at-new" {
		t.Fatalf("token = %q, want at-new", access.Token)
	}
	if credRepo.saveCall != 1 {
		t.Fatalf("save calls = %d, want 1", credRepo.saveCall)
	}
	if credRepo.saveCtxErr != nil {
		t.Fatalf("refresh persist must not inherit canceled request ctx: %v", credRepo.saveCtxErr)
	}
	if credRepo.cred.RefreshToken() != "rt-new" {
		t.Fatalf("rotated refresh token must be persisted, got %q", credRepo.cred.RefreshToken())
	}
}

// TestLockFailureUsesLiveToken 锁基础设施故障时不得无锁刷新：仍有有效 token 就用旧的。
//
// 回归：OpenAI 会轮换 refresh token 并检测重复使用，无锁并发刷新可能导致整组 token 被吊销。
func TestLockFailureUsesLiveToken(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(time.Minute), enum.OAuthCredentialStatusActive)}
	client := &fakeRefreshClient{provider: enum.OAuthProviderAnthropic}
	locker := &fakeLocker{err: ierr.New(ierr.ErrInternal, "redis unavailable")}

	access, err := newTokenProvider(credRepo, &fakeAccessCache{}, locker, client).GetAccessToken(t.Context(), 5)
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if access.Token != "at-old" {
		t.Fatalf("token = %q, want old live token", access.Token)
	}
	if client.refreshCall != 0 {
		t.Fatalf("refresh calls = %d, must not refresh without lock", client.refreshCall)
	}
}

// TestLockFailureExpiredTokenFails 锁故障且 token 已过期：返回可切换错误，不冒险并发刷新。
func TestLockFailureExpiredTokenFails(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(-time.Minute), enum.OAuthCredentialStatusActive)}
	client := &fakeRefreshClient{provider: enum.OAuthProviderAnthropic}
	locker := &fakeLocker{err: ierr.New(ierr.ErrInternal, "redis unavailable")}

	_, err := newTokenProvider(credRepo, &fakeAccessCache{}, locker, client).GetAccessToken(t.Context(), 5)
	if !isSwitchable(t, err) {
		t.Fatalf("want switchable OAuthCredentialError, got %v", err)
	}
	if client.refreshCall != 0 {
		t.Fatalf("refresh calls = %d, must not refresh without lock", client.refreshCall)
	}
}

// TestForbiddenWithoutInvalidGrantKeepsCredential 401/403 不等于凭据失效。
//
// 回归：token 端点经 WAF / Cloudflare 返回 403 时，旧逻辑会把整批账号标记 reauth_required。
func TestForbiddenWithoutInvalidGrantKeepsCredential(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(time.Minute), enum.OAuthCredentialStatusActive)}
	cache := &fakeAccessCache{}
	client := &fakeRefreshClient{
		provider:   enum.OAuthProviderAnthropic,
		refreshErr: &service.OAuthProtocolError{StatusCode: 403, Reason: "forbidden"},
	}

	_, err := newTokenProvider(credRepo, cache, &fakeLocker{locked: true}, client).GetAccessToken(t.Context(), 5)
	if !isSwitchable(t, err) {
		t.Fatalf("want switchable OAuthCredentialError, got %v", err)
	}
	if credRepo.saveCall != 0 || credRepo.cred.Status() != enum.OAuthCredentialStatusActive {
		t.Fatalf("403 without invalid_grant must keep credential active: save=%d status=%q", credRepo.saveCall, credRepo.cred.Status())
	}
	if len(cache.deleteCalls) != 0 {
		t.Fatalf("403 without invalid_grant must not invalidate cache: %v", cache.deleteCalls)
	}
}

// isSwitchable 断言错误可切换（OAuthCredentialError 已在 CanSwitchEndpoint 白名单）
func isSwitchable(t *testing.T, err error) bool {
	t.Helper()
	var oauthErr *model.OAuthCredentialError
	return errors.As(err, &oauthErr)
}
