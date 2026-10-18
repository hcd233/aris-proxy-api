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
	cred     *aggregate.OAuthCredential
	second   *aggregate.OAuthCredential // 非 nil 时第二次 Find 返回它（锁内双检场景）
	findCall int
	saveCall int
}

func (f *fakeCredRepo) FindByEndpointID(context.Context, uint) (*aggregate.OAuthCredential, error) {
	f.findCall++
	if f.findCall > 1 && f.second != nil {
		return f.second, nil
	}
	return f.cred, nil
}

func (f *fakeCredRepo) Save(_ context.Context, cred *aggregate.OAuthCredential) error {
	f.saveCall++
	f.cred = cred
	return nil
}

func (f *fakeCredRepo) DeleteByEndpointID(context.Context, uint) error { return nil }

func (f *fakeCredRepo) ListActive(context.Context) ([]*aggregate.OAuthCredential, error) {
	return nil, nil
}

type fakeAccessCache struct {
	access    service.OAuthAccess
	present   bool
	putCall   int
	lastTTL   time.Duration
	getCalled int
}

func (f *fakeAccessCache) Get(context.Context, uint) (service.OAuthAccess, bool) {
	f.getCalled++
	return f.access, f.present
}

func (f *fakeAccessCache) Put(_ context.Context, _ uint, access service.OAuthAccess, ttl time.Duration) {
	f.putCall++
	f.access, f.present, f.lastTTL = access, true, ttl
}

func (f *fakeAccessCache) Delete(context.Context, uint) {}

type fakeLocker struct{ locked bool }

func (f *fakeLocker) Lock(context.Context, string, string, time.Duration) (bool, error) {
	return f.locked, nil
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

// isSwitchable 断言错误可切换（OAuthCredentialError 已在 CanSwitchEndpoint 白名单）
func isSwitchable(t *testing.T, err error) bool {
	t.Helper()
	var oauthErr *model.OAuthCredentialError
	return errors.As(err, &oauthErr)
}
