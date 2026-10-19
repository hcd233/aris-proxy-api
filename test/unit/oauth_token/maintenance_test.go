package oauth_token

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// maintenanceCredRepo 支持多凭据的维护场景 fake。
//
// 分别记录「整行保存（Save）」与「窄列更新（UpdateQuota/UpdateLastError）」：
// 维护 sweep 只允许后者——Save 是按 endpoint_id 的整行 upsert，会把 token provider
// 刚刷新进 DB 的 token 覆盖回 ListActive 快照里的旧值。
type maintenanceCredRepo struct {
	creds       []*aggregate.OAuthCredential
	saveCalls   int
	quotaWrites int
	lastErrors  []string
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

func (f *maintenanceCredRepo) ListActive(context.Context) ([]*aggregate.OAuthCredential, error) {
	return f.creds, nil
}

func (f *maintenanceCredRepo) ListDisplayByEndpointIDs(context.Context, []uint) (map[uint]vo.OAuthCredentialDisplay, error) {
	return nil, nil
}

func (f *maintenanceCredRepo) UpdateQuota(_ context.Context, _ uint, _ vo.QuotaSnapshot, _ time.Time) error {
	f.quotaWrites++
	return nil
}

func (f *maintenanceCredRepo) UpdateLastError(_ context.Context, _ uint, lastError string) error {
	f.lastErrors = append(f.lastErrors, lastError)
	return nil
}

var _ llmproxy.OAuthCredentialRepository = (*maintenanceCredRepo)(nil)

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
	if repo.quotaWrites != 1 {
		t.Fatalf("quota writes = %d, want 1", repo.quotaWrites)
	}
	if len(tokens.refreshCalls) != 2 {
		t.Fatalf("refresh attempts = %d, want 2 (one per active credential)", len(tokens.refreshCalls))
	}
}

// TestSweepOAuthMaintenanceDoesNotOverwriteRefreshedToken 维护 sweep 的探测落库不得整行回写。
//
// 回归（CR C1）：token provider 的 RefreshIfNeeded 会把新 token 写进 DB，而 ListActive
// 快照仍持旧 token；探测成功后若用快照整行 Save，DB 中的新 token 会被旧值覆盖——
// OpenAI/Codex 轮换 refresh token 后旧值已失效，凭据随即报废并要求用户重新授权。
func TestSweepOAuthMaintenanceDoesNotOverwriteRefreshedToken(t *testing.T) {
	t.Parallel()

	repo := &maintenanceCredRepo{creds: []*aggregate.OAuthCredential{newMaintenanceCredential(t, 1, time.Hour)}}
	tokens := &maintenanceTokens{}
	client := &probeClient{}
	providers := map[enum.OAuthProvider]service.OAuthProviderClient{enum.OAuthProviderAnthropic: client}

	if err := cron.SweepOAuthMaintenance(t.Context(), repo, tokens, providers); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if repo.saveCalls != 0 {
		t.Fatalf("sweep must not full-row save (%d calls): it would overwrite the token refreshed by the provider", repo.saveCalls)
	}
	if repo.quotaWrites != 1 {
		t.Fatalf("quota writes = %d, want 1 (probe result must be persisted via narrow column update)", repo.quotaWrites)
	}
}

// TestSweepOAuthMaintenanceContinuesAfterProbeFailure 单账号探测失败不中断其余账号，且失败原因走窄列更新。
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
	if len(repo.lastErrors) != 2 {
		t.Fatalf("last error writes = %d, want 2 (probe error must be persisted per failing credential)", len(repo.lastErrors))
	}
	if !strings.Contains(repo.lastErrors[0], "upstream down") {
		t.Fatalf("last error must carry the probe failure reason, got %q", repo.lastErrors[0])
	}
	if repo.saveCalls != 0 {
		t.Fatalf("probe failure must not full-row save, got %d calls", repo.saveCalls)
	}
	if repo.creds[0].Status() != enum.OAuthCredentialStatusActive {
		t.Fatalf("probe failure must not change status machine, got %q", repo.creds[0].Status())
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
