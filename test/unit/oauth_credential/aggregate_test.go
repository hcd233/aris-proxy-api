// Package oauth_credential 验证 OAuth 凭据聚合的状态机与令牌轮换语义。
package oauth_credential

import (
	"errors"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// TestCreateOAuthCredentialRejectsMissingFields 必填字段缺失必须拒绝。
//
// endpointID=0 合法：创建路径在端点落库后由 SetEndpointID 回填（事务内绑定）。
func TestCreateOAuthCredentialRejectsMissingFields(t *testing.T) {
	t.Parallel()

	exp := time.Now().Add(time.Hour)
	cases := []struct {
		name      string
		endpoint  uint
		access    string
		refresh   string
		wantError bool
	}{
		{name: "zero endpoint id allowed before binding", endpoint: 0, access: "at", refresh: "rt", wantError: false},
		{name: "empty access token", endpoint: 1, access: "", refresh: "rt", wantError: true},
		{name: "empty refresh token", endpoint: 1, access: "at", refresh: "", wantError: true},
		{name: "valid", endpoint: 1, access: "at", refresh: "rt", wantError: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := aggregate.CreateOAuthCredential(tc.endpoint, enum.OAuthProviderAnthropic, tc.access, tc.refresh, exp, "org-1", "a@b.c", "max")
			if tc.wantError && !errors.Is(err, ierr.ErrValidation) {
				t.Fatalf("want ErrValidation, got %v", err)
			}
			if !tc.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestOAuthCredentialLifecycle 覆盖令牌轮换、重授权标记与额度快照写入。
func TestOAuthCredentialLifecycle(t *testing.T) {
	t.Parallel()

	exp := time.Now().Add(time.Hour)
	cred, err := aggregate.CreateOAuthCredential(7, enum.OAuthProviderAnthropic, "at-1", "rt-1", exp, "org-1", "alice@example.com", "max")
	if err != nil {
		t.Fatalf("CreateOAuthCredential: %v", err)
	}
	if cred.Status() != enum.OAuthCredentialStatusActive {
		t.Fatalf("new credential status = %q, want active", cred.Status())
	}
	if cred.EndpointID() != 7 || cred.AccessToken() != "at-1" || cred.RefreshToken() != "rt-1" {
		t.Fatal("constructor did not store fields")
	}
	if cred.AccountRef() != "org-1" || cred.AccountEmail() != "alice@example.com" || cred.PlanType() != "max" {
		t.Fatal("constructor did not store account metadata")
	}

	// MarkReauth 置状态并记录错误
	cred.MarkReauth("invalid_grant")
	if cred.Status() != enum.OAuthCredentialStatusReauthRequired {
		t.Fatalf("status = %q, want reauth_required", cred.Status())
	}
	if cred.LastError() != "invalid_grant" {
		t.Fatalf("last error = %q, want invalid_grant", cred.LastError())
	}

	// ApplyTokens：refresh 为空保留旧值（Claude 刷新可能不轮换）；清 lastError；恢复 active
	newExp := exp.Add(2 * time.Hour)
	cred.ApplyTokens("at-2", "", newExp)
	if cred.AccessToken() != "at-2" || cred.RefreshToken() != "rt-1" {
		t.Fatalf("empty refresh must keep old value, got access %q refresh %q", cred.AccessToken(), cred.RefreshToken())
	}
	if !cred.ExpiresAt().Equal(newExp) {
		t.Fatalf("expiresAt = %v, want %v", cred.ExpiresAt(), newExp)
	}
	if cred.Status() != enum.OAuthCredentialStatusActive || cred.LastError() != "" {
		t.Fatalf("apply tokens must clear reauth state, got status %q error %q", cred.Status(), cred.LastError())
	}

	// ApplyTokens：refresh 非空覆盖（轮换）
	cred.ApplyTokens("at-3", "rt-2", newExp)
	if cred.RefreshToken() != "rt-2" {
		t.Fatalf("rotated refresh token not stored, got %q", cred.RefreshToken())
	}

	// SetQuota 写入快照与时间
	at := time.Now()
	snapshot := vo.QuotaSnapshot{
		Windows:    []vo.QuotaWindow{{Key: "five_hour", UsedPercent: 12.5, ResetsAt: at.Add(time.Hour), WindowMinutes: 300}},
		PlanType:   "max",
		CapturedAt: at,
	}
	cred.SetQuota(snapshot, at)
	if len(cred.Quota().Windows) != 1 || cred.Quota().Windows[0].Key != "five_hour" {
		t.Fatal("quota snapshot not stored")
	}
	if !cred.QuotaUpdatedAt().Equal(at) {
		t.Fatalf("quotaUpdatedAt = %v, want %v", cred.QuotaUpdatedAt(), at)
	}
}
