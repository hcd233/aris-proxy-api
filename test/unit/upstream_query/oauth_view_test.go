package upstream_query

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/upstream/port"
	"github.com/hcd233/aris-proxy-api/internal/application/upstream/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	useragg "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	llmvo "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// fakeOAuthCredRepo 可编程凭据仓储（null 值 = 无凭据行）
type fakeOAuthCredRepo struct {
	creds map[uint]*llmagg.OAuthCredential
	// displays 非 nil 时直接返回该投影（用于构造任意 lastError 等展示字段）
	displays map[uint]llmvo.OAuthCredentialDisplay
}

func (f *fakeOAuthCredRepo) FindByEndpointID(_ context.Context, endpointID uint) (*llmagg.OAuthCredential, error) {
	return f.creds[endpointID], nil
}

func (f *fakeOAuthCredRepo) Save(context.Context, *llmagg.OAuthCredential) error { return nil }

func (f *fakeOAuthCredRepo) ListActive(context.Context) ([]*llmagg.OAuthCredential, error) {
	return nil, nil
}

// ListDisplayByEndpointIDs 从内存凭据组装展示投影（与仓储实现的列映射保持一致）
func (f *fakeOAuthCredRepo) ListDisplayByEndpointIDs(_ context.Context, endpointIDs []uint) (map[uint]llmvo.OAuthCredentialDisplay, error) {
	if f.displays != nil {
		return f.displays, nil
	}
	displays := make(map[uint]llmvo.OAuthCredentialDisplay, len(endpointIDs))
	for _, id := range endpointIDs {
		cred := f.creds[id]
		if cred == nil {
			continue
		}
		displays[id] = llmvo.OAuthCredentialDisplay{
			EndpointID:     cred.EndpointID(),
			Provider:       cred.Provider(),
			AccountEmail:   cred.AccountEmail(),
			PlanType:       cred.PlanType(),
			Status:         cred.Status(),
			LastError:      cred.LastError(),
			Quota:          cred.Quota(),
			QuotaUpdatedAt: cred.QuotaUpdatedAt(),
		}
	}
	return displays, nil
}

func (f *fakeOAuthCredRepo) UpdateQuota(context.Context, uint, llmvo.QuotaSnapshot, time.Time) error {
	return nil
}

func (f *fakeOAuthCredRepo) UpdateLastError(context.Context, uint, string) error { return nil }

// TestListUpstreamOAuthView 订阅端点视图装配：auth_type/状态/邮箱/额度；demo 掩码邮箱且额度保留（Review Focus 3）。
func TestListUpstreamOAuthView(t *testing.T) {
	t.Parallel()

	const endpointID = 101
	ep, err := llmagg.CreateEndpoint(endpointID, "claude-sub", "", "https://api.anthropic.com/v1", "", false, false, true, false, enum.EndpointAuthOAuthAnthropic)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	ep.SetUserID(uAlice)
	ep.SetTimestamps(testTime(), testTime())

	m, err := llmagg.CreateModel(201, llmvo.EndpointAlias("claude-sonnet"), "claude-sonnet-4-5", endpointID, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	m.SetUserID(uAlice)
	m.SetTimestamps(testTime(), testTime())

	cred, err := llmagg.CreateOAuthCredential(endpointID, enum.OAuthProviderAnthropic, "at-secret", "rt-secret", testTime().Add(time.Hour), "org-1", "alice@example.com", "max")
	if err != nil {
		t.Fatalf("CreateOAuthCredential: %v", err)
	}
	cred.SetQuota(llmvo.QuotaSnapshot{
		Windows:    []llmvo.QuotaWindow{{Key: "five_hour", UsedPercent: 12.5, ResetsAt: testTime(), WindowMinutes: 300}},
		CapturedAt: testTime(),
	}, testTime())

	handler := query.NewListUpstreamHandler(
		&fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{endpointID: ep}},
		&fakeModelRepo{models: []*llmagg.Model{m}},
		&fakeUserRepo{users: map[uint]*useragg.User{uAlice: alice(t)}},
		&fakeOAuthCredRepo{creds: map[uint]*llmagg.OAuthCredential{endpointID: cred}},
	)

	queryGroups := func(isDemo bool) *port.UpstreamEndpointView {
		t.Helper()
		groups, _, _, err := handler.Handle(context.Background(), port.ListUpstreamQuery{
			CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
			IsDemo:      isDemo,
		})
		if err != nil {
			t.Fatalf("Handle(isDemo=%v): %v", isDemo, err)
		}
		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		return groups[0].Endpoint
	}

	view := queryGroups(false)
	if view.AuthType != string(enum.EndpointAuthOAuthAnthropic) {
		t.Fatalf("auth type = %q", view.AuthType)
	}
	if view.OAuth == nil || view.OAuth.Status != string(enum.OAuthCredentialStatusActive) {
		t.Fatalf("oauth view missing/wrong: %+v", view.OAuth)
	}
	if view.OAuth.AccountEmail != "alice@example.com" || view.OAuth.PlanType != "max" {
		t.Fatalf("oauth view fields wrong: %+v", view.OAuth)
	}
	if view.OAuth.Quota == nil || len(view.OAuth.Quota.Windows) != 1 || view.OAuth.Quota.Windows[0].UsedPercent != 12.5 {
		t.Fatalf("quota not assembled: %+v", view.OAuth.Quota)
	}

	demoView := queryGroups(true)
	if demoView.OAuth == nil || demoView.OAuth.AccountEmail != "***" {
		t.Fatalf("demo view must mask account email: %+v", demoView.OAuth)
	}
	if demoView.OAuth.LastError != "" {
		t.Fatalf("demo view must hide last error: %q", demoView.OAuth.LastError)
	}
	if demoView.OAuth.Quota == nil || len(demoView.OAuth.Quota.Windows) != 1 {
		t.Fatalf("demo view must keep quota: %+v", demoView.OAuth.Quota)
	}
}

// TestListUpstreamOAuthViewWithoutCredential 缺凭据行的 OAuth 端点视图按 reauth_required 呈现（fail-closed 口径一致）。
func TestListUpstreamOAuthViewWithoutCredential(t *testing.T) {
	t.Parallel()

	const endpointID = 102
	ep, err := llmagg.CreateEndpoint(endpointID, "codex-sub", "https://chatgpt.com/backend-api/codex", "", "", false, true, false, false, enum.EndpointAuthOAuthOpenAI)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	ep.SetUserID(uAlice)
	ep.SetTimestamps(testTime(), testTime())

	m, err := llmagg.CreateModel(202, llmvo.EndpointAlias("gpt-5-codex"), "gpt-5-codex", endpointID, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	m.SetUserID(uAlice)
	m.SetTimestamps(testTime(), testTime())

	handler := query.NewListUpstreamHandler(
		&fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{endpointID: ep}},
		&fakeModelRepo{models: []*llmagg.Model{m}},
		&fakeUserRepo{users: map[uint]*useragg.User{uAlice: alice(t)}},
		&fakeOAuthCredRepo{},
	)

	groups, _, _, err := handler.Handle(context.Background(), port.ListUpstreamQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	view := groups[0].Endpoint
	if view.AuthType != string(enum.EndpointAuthOAuthOpenAI) || view.OAuth == nil {
		t.Fatalf("oauth view missing: %+v", view)
	}
	if view.OAuth.Status != string(enum.OAuthCredentialStatusReauthRequired) {
		t.Fatalf("missing credential must render reauth_required, got %q", view.OAuth.Status)
	}
}

// TestListUpstreamOAuthViewMasksNonOwner 普通用户看他人公开 OAuth 端点时，
// 账号邮箱与错误详情必须脱敏（跨租户 PII）；归属者与 admin 仍看到完整值。
func TestListUpstreamOAuthViewMasksNonOwner(t *testing.T) {
	t.Parallel()

	const endpointID = 103
	ep, err := llmagg.CreateEndpoint(endpointID, "claude-public", "", "https://api.anthropic.com/v1", "", false, false, true, false, enum.EndpointAuthOAuthAnthropic)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	ep.SetUserID(uAlice)
	ep.RestoreVisibility(enum.EndpointVisibilityPublic)
	ep.SetTimestamps(testTime(), testTime())

	handler := query.NewListUpstreamHandler(
		&fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{endpointID: ep}},
		&fakeModelRepo{},
		&fakeUserRepo{users: map[uint]*useragg.User{uAlice: alice(t)}},
		&fakeOAuthCredRepo{displays: map[uint]llmvo.OAuthCredentialDisplay{
			endpointID: {
				EndpointID:   endpointID,
				Provider:     enum.OAuthProviderAnthropic,
				AccountEmail: "alice@example.com",
				PlanType:     "max",
				Status:       enum.OAuthCredentialStatusActive,
				LastError:    "invalid_grant",
				Quota:        llmvo.QuotaSnapshot{Windows: []llmvo.QuotaWindow{}},
			},
		}},
	)

	viewFor := func(scope *uint) *port.UpstreamEndpointView {
		t.Helper()
		groups, _, _, err := handler.Handle(context.Background(), port.ListUpstreamQuery{
			CommonParam:   model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
			ScopeUserID:   scope,
			IncludePublic: scope != nil,
		})
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if len(groups) != 1 || groups[0].Endpoint == nil || groups[0].Endpoint.OAuth == nil {
			t.Fatalf("oauth view missing: %+v", groups)
		}
		return groups[0].Endpoint
	}

	other := uBob
	otherView := viewFor(&other)
	if otherView.OAuth.AccountEmail != "***" || otherView.OAuth.LastError != "" {
		t.Fatalf("non-owner view must mask email/error, got %+v", otherView.OAuth)
	}
	if otherView.OAuth.PlanType != "max" {
		t.Fatalf("non-owner view keeps plan type, got %q", otherView.OAuth.PlanType)
	}

	owner := uAlice
	ownerView := viewFor(&owner)
	if ownerView.OAuth.AccountEmail != "alice@example.com" || ownerView.OAuth.LastError != "invalid_grant" {
		t.Fatalf("owner view must be unmasked, got %+v", ownerView.OAuth)
	}

	adminView := viewFor(nil)
	if adminView.OAuth.AccountEmail != "alice@example.com" {
		t.Fatalf("admin view must be unmasked, got %+v", adminView.OAuth)
	}
}
