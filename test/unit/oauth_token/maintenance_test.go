package oauth_token

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// maintenanceCredRepo 支持多凭据的维护场景 fake
type maintenanceCredRepo struct {
	creds     []*aggregate.OAuthCredential
	saveCalls int
}

func (f *maintenanceCredRepo) FindByEndpointID(_ context.Context, endpointID uint) (*aggregate.OAuthCredential, error) {
	for _, cred := range f.creds {
		if cred.EndpointID() == endpointID {
			return cred, nil
		}
	}
	return nil, nil
}

func (f *maintenanceCredRepo) Save(context.Context, *aggregate.OAuthCredential) error {
	f.saveCalls++
	return nil
}

func (f *maintenanceCredRepo) DeleteByEndpointID(context.Context, uint) error { return nil }

func (f *maintenanceCredRepo) ListActive(context.Context) ([]*aggregate.OAuthCredential, error) {
	return f.creds, nil
}

// maintenanceTokens 记录刷新调用的 token provider fake
type maintenanceTokens struct {
	refreshCalls []uint
}

func (f *maintenanceTokens) RefreshIfNeeded(_ context.Context, endpointID uint) error {
	f.refreshCalls = append(f.refreshCalls, endpointID)
	return nil
}

func (f *maintenanceTokens) GetAccessToken(context.Context, uint) (service.OAuthAccess, error) {
	return service.OAuthAccess{Token: "at", AccountRef: "org-1", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

// probeClient 可编程探测结果与调用计数
type probeClient struct {
	fakeRefreshClient
	probeCalls int
	probeErr   error
}

func (p *probeClient) ProbeQuota(context.Context, string, string) (*vo.QuotaSnapshot, error) {
	p.probeCalls++
	if p.probeErr != nil {
		return nil, p.probeErr
	}
	return &vo.QuotaSnapshot{Windows: []vo.QuotaWindow{{Key: "five_hour", UsedPercent: 10}}, CapturedAt: time.Now()}, nil
}

func newMaintenanceCredential(t *testing.T, endpointID uint, quotaAge time.Duration) *aggregate.OAuthCredential {
	t.Helper()
	cred, err := aggregate.CreateOAuthCredential(endpointID, enum.OAuthProviderAnthropic, "at-old", "rt-old", time.Now().Add(time.Hour), "org-1", "a@b.c", "max")
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	cred.SetQuota(vo.QuotaSnapshot{Windows: []vo.QuotaWindow{}}, time.Now().Add(-quotaAge))
	return cred
}

// TestSweepOAuthMaintenanceProbesStaleQuota 陈旧额度被探测写回；窗口内凭据跳过探测；刷新对每个 active 凭据都尝试。
func TestSweepOAuthMaintenanceProbesStaleQuota(t *testing.T) {
	t.Parallel()

	repo := &maintenanceCredRepo{creds: []*aggregate.OAuthCredential{
		newMaintenanceCredential(t, 1, time.Hour),     // 陈旧 → 探测
		newMaintenanceCredential(t, 2, 5*time.Minute), // 窗口内 → 跳过
	}}
	tokens := &maintenanceTokens{}
	client := &probeClient{}
	providers := map[enum.OAuthProvider]service.OAuthProviderClient{enum.OAuthProviderAnthropic: client}

	if err := cron.SweepOAuthMaintenance(t.Context(), repo, tokens, providers); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if client.probeCalls != 1 {
		t.Fatalf("probe calls = %d, want 1 (only stale credential)", client.probeCalls)
	}
	if repo.saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1", repo.saveCalls)
	}
	if len(tokens.refreshCalls) != 2 {
		t.Fatalf("refresh attempts = %d, want 2 (one per active credential)", len(tokens.refreshCalls))
	}
}

// TestSweepOAuthMaintenanceContinuesAfterProbeFailure 单账号探测失败不中断其余账号。
func TestSweepOAuthMaintenanceContinuesAfterProbeFailure(t *testing.T) {
	t.Parallel()

	repo := &maintenanceCredRepo{creds: []*aggregate.OAuthCredential{
		newMaintenanceCredential(t, 1, time.Hour),
		newMaintenanceCredential(t, 2, time.Hour),
	}}
	tokens := &maintenanceTokens{}
	client := &probeClient{probeErr: ierr.New(ierr.ErrProxyRequest, "upstream down")}
	providers := map[enum.OAuthProvider]service.OAuthProviderClient{enum.OAuthProviderAnthropic: client}

	if err := cron.SweepOAuthMaintenance(t.Context(), repo, tokens, providers); err != nil {
		t.Fatalf("sweep must not fail on single-credential probe error: %v", err)
	}
	if client.probeCalls != 2 {
		t.Fatalf("probe calls = %d, want 2 (loop must continue)", client.probeCalls)
	}
	// M1：探测失败写 last_error 供展示（不改状态机），每个失败账号 save 一次
	if repo.saveCalls != 2 {
		t.Fatalf("save calls = %d, want 2 (probe error must be persisted)", repo.saveCalls)
	}
	if repo.creds[0].LastError() == "" || repo.creds[0].Status() != enum.OAuthCredentialStatusActive {
		t.Fatalf("probe failure must set last_error without changing status: %q/%q", repo.creds[0].LastError(), repo.creds[0].Status())
	}
}

// TestSweepOAuthMaintenanceEmptyList 空列表安全返回（无凭据时 no-op）。
func TestSweepOAuthMaintenanceEmptyList(t *testing.T) {
	t.Parallel()

	repo := &maintenanceCredRepo{}
	if err := cron.SweepOAuthMaintenance(t.Context(), repo, &maintenanceTokens{}, map[enum.OAuthProvider]service.OAuthProviderClient{}); err != nil {
		t.Fatalf("empty sweep: %v", err)
	}
}
