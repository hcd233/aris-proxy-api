// Package oauth2_initiate 验证 application/oauth2/command.InitiateLoginHandler
// 的安全契约：必须调用 GetAuthURLWithState(state) 而非静态 GetAuthURL()，
// 且 state 来源于一次性生成的 StateManager，避免 CSRF。
package oauth2_initiate

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bytedance/sonic"
	xoauth2 "golang.org/x/oauth2"

	"github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/oauth2/vo"
)

type initiateCase struct {
	Name                   string `json:"name"`
	Description            string `json:"description"`
	Platform               string `json:"platform"`
	ExpectErrKind          string `json:"expectErrKind"`
	ExpectURLContainsState bool   `json:"expectURLContainsState"`
}

func loadCases(t *testing.T) []initiateCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []initiateCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

// stubPlatform 记录 GetAuthURLWithState 最近一次收到的 state，
// 并在 URL 中回显 state，便于测试断言。
type stubPlatform struct {
	name           string
	withStateCalls atomic.Int32
	lastReceived   atomic.Value // string
}

func newStubPlatform(name string) *stubPlatform {
	p := &stubPlatform{name: name}
	p.lastReceived.Store("")
	return p
}

func (p *stubPlatform) GetAuthURLWithState(state string) string {
	p.withStateCalls.Add(1)
	p.lastReceived.Store(state)
	return "https://example.test/auth?state=" + state + "&platform=" + p.name
}

func (p *stubPlatform) ExchangeToken(_ context.Context, _ string) (*xoauth2.Token, error) {
	return nil, errors.New("not used in initiate tests")
}

func (p *stubPlatform) GetUserInfo(_ context.Context, _ *xoauth2.Token) (vo.OAuthUserInfo, error) {
	return vo.NewOAuthUserInfo("", "", "", ""), errors.New("not used in initiate tests")
}

func TestInitiateLogin(t *testing.T) {
	ctx := context.Background()

	githubStub := newStubPlatform(constant.OAuthProviderGithub)
	googleStub := newStubPlatform(constant.OAuthProviderGoogle)
	platforms := map[string]service.Platform{
		constant.OAuthProviderGithub: githubStub,
		constant.OAuthProviderGoogle: googleStub,
	}
	handler := command.NewInitiateLoginHandler(platforms)

	for _, tc := range loadCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			result, err := handler.Handle(ctx, command.InitiateLoginCommand{Platform: tc.Platform})

			switch tc.ExpectErrKind {
			case "":
				if err != nil {
					t.Fatalf("expected success, got err: %v", err)
				}
				if result == nil || result.RedirectURL == "" {
					t.Fatal("expected non-empty RedirectURL")
				}

				stub := platforms[tc.Platform].(*stubPlatform)
				if stub.withStateCalls.Load() == 0 {
					t.Fatal("expected GetAuthURLWithState to be called, but it was not (static state fallback regressed)")
				}
				state, _ := stub.lastReceived.Load().(string)
				if state == "" {
					t.Fatal("expected non-empty state passed to GetAuthURLWithState")
				}
				if tc.ExpectURLContainsState && !strings.Contains(result.RedirectURL, state) {
					t.Errorf("redirect URL does not contain state: url=%q state=%q", result.RedirectURL, state)
				}
				// 安全断言：state 必须是 StateManager 生成的一次性值（非空、长度 >= 32 hex chars = 16 bytes）
				if len(state) < 32 {
					t.Errorf("state length = %d, expected at least 32 hex chars (16 bytes) from one-time StateManager", len(state))
				}
			case "bad_request":
				if !errors.Is(err, ierr.ErrBadRequest) {
					t.Errorf("expected ErrBadRequest, got: %v", err)
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %+v", result)
				}
			default:
				t.Fatalf("unknown expectErrKind: %q", tc.ExpectErrKind)
			}
		})
	}
}
