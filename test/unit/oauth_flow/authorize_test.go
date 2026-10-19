package oauth_flow

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/oauthprovider"
)

// fakeFlowStore 内存版授权流存储（语义与 Redis GetDel 对齐：Take 即取即删）。
type fakeFlowStore struct {
	flows map[string]vo.OAuthFlow
}

func newFakeFlowStore() *fakeFlowStore {
	return &fakeFlowStore{flows: map[string]vo.OAuthFlow{}}
}

func (s *fakeFlowStore) Put(_ context.Context, state string, flow vo.OAuthFlow) error {
	s.flows[state] = flow
	return nil
}

func (s *fakeFlowStore) Take(_ context.Context, state string) (vo.OAuthFlow, bool, error) {
	flow, ok := s.flows[state]
	if ok {
		delete(s.flows, state)
	}
	return flow, ok, nil
}

var _ service.OAuthFlowStore = (*fakeFlowStore)(nil)

func newAuthorizeHandler(store service.OAuthFlowStore) port.AuthorizeOAuthHandler {
	return command.NewOAuthAuthorizeHandler(store, oauthprovider.NewClients())
}

// TestAuthorizeStoresVerifierMatchingChallenge URL 里的 code_challenge 必须与 flow 中 verifier 配套（S256）。
func TestAuthorizeStoresVerifierMatchingChallenge(t *testing.T) {
	t.Parallel()

	store := newFakeFlowStore()
	handler := newAuthorizeHandler(store)

	result, err := handler.Handle(t.Context(), port.AuthorizeOAuthCommand{UserID: 42, Provider: enum.OAuthProviderAnthropic})
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}

	parsed, err := url.Parse(result.AuthorizeURL)
	if err != nil {
		t.Fatalf("authorize url unparsable: %v", err)
	}
	challenge := parsed.Query().Get("code_challenge")
	if challenge == "" {
		t.Fatal("authorize url missing code_challenge")
	}
	if parsed.Query().Get("state") != result.State {
		t.Fatalf("state mismatch: url %q result %q", parsed.Query().Get("state"), result.State)
	}

	flow, ok, err := store.Take(t.Context(), result.State)
	if err != nil || !ok {
		t.Fatalf("flow not stored: ok=%v err=%v", ok, err)
	}
	if flow.UserID != 42 || flow.Provider != enum.OAuthProviderAnthropic {
		t.Fatalf("unexpected flow: %+v", flow)
	}
	sum := sha256.Sum256([]byte(flow.Verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); challenge != want {
		t.Fatalf("challenge %q not paired with stored verifier", challenge)
	}
	if time.Since(flow.CreatedAt) > time.Minute {
		t.Fatalf("flow createdAt not recent: %v", flow.CreatedAt)
	}
}

// TestFlowStoreSingleUse flow 一次性消费（防重放）。
func TestFlowStoreSingleUse(t *testing.T) {
	t.Parallel()

	store := newFakeFlowStore()
	handler := newAuthorizeHandler(store)
	result, err := handler.Handle(t.Context(), port.AuthorizeOAuthCommand{UserID: 7, Provider: enum.OAuthProviderOpenAI})
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}

	if _, ok, _ := store.Take(t.Context(), result.State); !ok {
		t.Fatal("first take must succeed")
	}
	if _, ok, _ := store.Take(t.Context(), result.State); ok {
		t.Fatal("second take must fail (single use)")
	}
}

// TestAuthorizeRejectsInvalidProvider 非法 provider 返回 ErrValidation。
func TestAuthorizeRejectsInvalidProvider(t *testing.T) {
	t.Parallel()

	handler := newAuthorizeHandler(newFakeFlowStore())
	_, err := handler.Handle(t.Context(), port.AuthorizeOAuthCommand{UserID: 1, Provider: enum.OAuthProvider("bogus")})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}
