package oauth_token

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

// TestLockContentionFallsBackToOldToken 锁被持有时用尚未真正过期的旧 token（Review Focus 2 的降级分支）。
func TestLockContentionFallsBackToOldToken(t *testing.T) {
	t.Parallel()

	// 凭据在 3 分钟刷新提前量内（触发刷新路径）但尚未真正过期
	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(time.Minute), enum.OAuthCredentialStatusActive)}
	cache := &fakeAccessCache{}
	client := &fakeRefreshClient{provider: enum.OAuthProviderAnthropic}
	provider := newTokenProvider(credRepo, cache, &fakeLocker{locked: false}, client)

	access, err := provider.GetAccessToken(t.Context(), 5)
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if access.Token != "at-old" {
		t.Fatalf("lock contention with live token must fall back to old token, got %q", access.Token)
	}
	if client.refreshCall != 0 {
		t.Fatal("lock held by another instance: this instance must not refresh")
	}
}

// TestLockContentionWithoutLiveTokenFailsSwitchable 锁竞争且旧 token 已过期 → 可切换错误。
func TestLockContentionWithoutLiveTokenFailsSwitchable(t *testing.T) {
	t.Parallel()

	credRepo := &fakeCredRepo{cred: newCredential(t, time.Now().Add(-time.Minute), enum.OAuthCredentialStatusActive)}
	cache := &fakeAccessCache{}
	client := &fakeRefreshClient{provider: enum.OAuthProviderAnthropic}
	provider := newTokenProvider(credRepo, cache, &fakeLocker{locked: false}, client)

	_, err := provider.GetAccessToken(t.Context(), 5)
	if err == nil {
		t.Fatal("expired token with lock contention must fail")
	}
	if !isSwitchable(t, err) {
		t.Fatalf("error must be switchable (OAuthCredentialError), got %v", err)
	}
}

// TestDoubleCheckSkipsSecondRefresh 锁内双检：等待期间他方已刷新 → 本实例不再刷新。
func TestDoubleCheckSkipsSecondRefresh(t *testing.T) {
	t.Parallel()

	// 第一次读（进入刷新路径前）是临期旧凭据；第二次读（锁内双检）已是他方刷好的新鲜凭据
	stale := newCredential(t, time.Now().Add(time.Minute), enum.OAuthCredentialStatusActive)
	fresh, err := aggregate.CreateOAuthCredential(5, enum.OAuthProviderAnthropic, "at-fresh", "rt-old", time.Now().Add(time.Hour), "org-1", "a@b.c", "max")
	if err != nil {
		t.Fatalf("create fresh credential: %v", err)
	}
	credRepo := &fakeCredRepo{cred: stale, second: fresh}
	cache := &fakeAccessCache{}
	client := &fakeRefreshClient{provider: enum.OAuthProviderAnthropic}
	provider := newTokenProvider(credRepo, cache, &fakeLocker{locked: true}, client)

	access, err := provider.GetAccessToken(t.Context(), 5)
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if client.refreshCall != 0 {
		t.Fatalf("double-check must skip refresh when another instance already refreshed, calls=%d", client.refreshCall)
	}
	if access.Token != "at-fresh" {
		t.Fatalf("token = %q, want fresh from double-check", access.Token)
	}
}
