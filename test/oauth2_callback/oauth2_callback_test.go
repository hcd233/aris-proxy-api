// Package oauth2_callback 验证 application/oauth2/command.HandleCallbackHandler
// 的行为：state 验证 → token 交换 → 用户查找/创建 → token 签发
package oauth2_callback

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	xoauth2 "golang.org/x/oauth2"

	"github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	identityvo "github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/oauth2/vo"
	infraoauth2 "github.com/hcd233/aris-proxy-api/internal/infrastructure/oauth2"
)

// callbackCase 回调测试用例
type callbackCase struct {
	Name                string              `json:"name"`
	Description         string              `json:"description"`
	Platform            string              `json:"platform"`
	Code                string              `json:"code"`
	State               string              `json:"state"`
	StubUserInfo        *stubUserInfo       `json:"stub_user_info"`
	StubExistingUser    *existingStubUser   `json:"stub_existing_user"`
	ExpectedIsNew       bool                `json:"expected_is_new"`
	ExpectErrorKind     string              `json:"expect_error_kind"`
}

// stubUserInfo fixture 中模拟的用户信息
type stubUserInfo struct {
	GhBind   string `json:"gh_bind"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

// existingStubUser 从 fixture stub_existing_user 反序列化的已存在用户信息
type existingStubUser struct {
	ID        string `json:"id"`
	GhBind    string `json:"gh_bind"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Avatar    string `json:"avatar"`
	LastLogin string `json:"last_login"` // RFC3339 format
	CreatedAt string `json:"created_at"` // RFC3339 format
}

// LastLoginTime 返回 last_login 时间
func (e *existingStubUser) LastLoginTime() (time.Time, error) {
	return time.Parse(time.RFC3339, e.LastLogin)
}

// CreatedAtTime 返回 created_at 时间
func (e *existingStubUser) CreatedAtTime() (time.Time, error) {
	return time.Parse(time.RFC3339, e.CreatedAt)
}

func loadCases(t *testing.T) []callbackCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []callbackCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

func findCase(t *testing.T, cases []callbackCase, name string) callbackCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return callbackCase{}
}

// ==================== Stubs ====================

// stubPlatform 模拟 OAuth2 平台：固定返回 token 和用户信息
type stubPlatform struct {
	name          string
	exchangeToken func(ctx context.Context, code string) (*xoauth2.Token, error)
	getUserInfo   func(ctx context.Context, token *xoauth2.Token) (vo.OAuthUserInfo, error)
}

func newStubPlatform(name string, userInfo *stubUserInfo) *stubPlatform {
	return &stubPlatform{
		name: name,
		exchangeToken: func(_ context.Context, code string) (*xoauth2.Token, error) {
			return &xoauth2.Token{AccessToken: "stub-access-token", TokenType: "bearer"}, nil
		},
		getUserInfo: func(_ context.Context, _ *xoauth2.Token) (vo.OAuthUserInfo, error) {
			if userInfo == nil {
				return vo.OAuthUserInfo{}, nil
			}
			return vo.OAuthUserInfo{
				ID:     userInfo.GhBind,
				Name:   userInfo.Name,
				Email:  userInfo.Email,
				Avatar: userInfo.Avatar,
			}, nil
		},
	}
}

func (p *stubPlatform) GetAuthURLWithState(state string) string {
	return "https://example.test/auth?state=" + state
}

func (p *stubPlatform) ExchangeToken(ctx context.Context, code string) (*xoauth2.Token, error) {
	return p.exchangeToken(ctx, code)
}

func (p *stubPlatform) GetUserInfo(ctx context.Context, token *xoauth2.Token) (vo.OAuthUserInfo, error) {
	return p.getUserInfo(ctx, token)
}

// stubUserRepo 模拟用户仓储
type stubUserRepo struct {
	findByID           func(ctx context.Context, id uint) (*aggregate.User, error)
	findByGithubBindID func(ctx context.Context, bindID string) (*aggregate.User, error)
	findByGoogleBindID func(ctx context.Context, bindID string) (*aggregate.User, error)
	save               func(ctx context.Context, user *aggregate.User) error
	touchLastLogin     func(ctx context.Context, userID uint) error
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{}
}

func (r *stubUserRepo) FindByID(ctx context.Context, id uint) (*aggregate.User, error) {
	if r.findByID != nil {
		return r.findByID(ctx, id)
	}
	return nil, nil
}

func (r *stubUserRepo) FindByGithubBindID(ctx context.Context, bindID string) (*aggregate.User, error) {
	if r.findByGithubBindID != nil {
		return r.findByGithubBindID(ctx, bindID)
	}
	return nil, nil
}

func (r *stubUserRepo) FindByGoogleBindID(ctx context.Context, bindID string) (*aggregate.User, error) {
	if r.findByGoogleBindID != nil {
		return r.findByGoogleBindID(ctx, bindID)
	}
	return nil, nil
}

func (r *stubUserRepo) Save(ctx context.Context, user *aggregate.User) error {
	if r.save != nil {
		return r.save(ctx, user)
	}
	user.SetID(100) // simulate persistence assigning ID
	return nil
}

func (r *stubUserRepo) TouchLastLogin(ctx context.Context, userID uint) error {
	if r.touchLastLogin != nil {
		return r.touchLastLogin(ctx, userID)
	}
	return nil
}

// Ensure stubUserRepo implements identity.UserRepository
var _ identity.UserRepository = (*stubUserRepo)(nil)

// stubTokenSigner 模拟 token 签发者
type stubTokenSigner struct {
	encodeToken func(userID uint) (string, error)
}

func newStubTokenSigner(token string) *stubTokenSigner {
	return &stubTokenSigner{
		encodeToken: func(_ uint) (string, error) {
			return token, nil
		},
	}
}

func (s *stubTokenSigner) EncodeToken(userID uint) (string, error) {
	return s.encodeToken(userID)
}

func (s *stubTokenSigner) DecodeToken(_ string) (uint, error) {
	return 0, errors.New("not implemented")
}

// stubObjStorageDirCreator 模拟对象存储目录创建器
type stubObjStorageDirCreator struct {
	createDir func(ctx context.Context, userID uint) error
}

func newStubObjStorageDirCreator() *stubObjStorageDirCreator {
	return &stubObjStorageDirCreator{
		createDir: func(_ context.Context, _ uint) error {
			return nil
		},
	}
}

func (s *stubObjStorageDirCreator) CreateDir(ctx context.Context, userID uint) error {
	return s.createDir(ctx, userID)
}

// makeHandler 构造 HandleCallbackHandler 的工厂方法
func makeHandler(userRepo *stubUserRepo, tc *callbackCase) command.HandleCallbackHandler {
	githubStub := newStubPlatform(constant.OAuthProviderGithub, tc.StubUserInfo)
	platforms := map[string]service.Platform{
		constant.OAuthProviderGithub: githubStub,
	}

	accessSigner := newStubTokenSigner("access-token-stub")
	refreshSigner := newStubTokenSigner("refresh-token-stub")
	objDirCreator := newStubObjStorageDirCreator()

	return command.NewHandleCallbackHandler(
		platforms,
		userRepo,
		accessSigner,
		refreshSigner,
		objDirCreator,
	)
}

// TestHandleCallback_NewUser 新用户流程应注册并签发 token 对
func TestHandleCallback_NewUser(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "callback_new_user")
	ctx := context.Background()

	state, err := infraoauth2.GenerateOAuth2State()
	if err != nil {
		t.Fatalf("GenerateOAuth2State() error: %v", err)
	}

	userRepo := newStubUserRepo()
	handler := makeHandler(userRepo, &tc)

	result, err := handler.Handle(ctx, command.HandleCallbackCommand{
		Platform: tc.Platform,
		Code:     tc.Code,
		State:    state,
	})
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if result == nil {
		t.Fatal("Handle() returned nil result")
	}
	if result.TokenPair == nil {
		t.Fatal("TokenPair should not be nil")
	}
	if result.TokenPair.AccessToken != "access-token-stub" {
		t.Errorf("AccessToken = %q, want %q", result.TokenPair.AccessToken, "access-token-stub")
	}
	if result.TokenPair.RefreshToken != "refresh-token-stub" {
		t.Errorf("RefreshToken = %q, want %q", result.TokenPair.RefreshToken, "refresh-token-stub")
	}
	if result.UserID == 0 {
		t.Error("UserID should not be 0")
	}
	if !result.IsNewUser {
		t.Error("IsNewUser = false, want true for new user")
	}
}

// TestHandleCallback_ExistingUser 已存在用户应更新登录时间而非重新注册
func TestHandleCallback_ExistingUser(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "callback_existing_user")
	ctx := context.Background()

	state, err := infraoauth2.GenerateOAuth2State()
	if err != nil {
		t.Fatalf("GenerateOAuth2State() error: %v", err)
	}

	existingUserID, parseErr := strconv.ParseUint(tc.StubExistingUser.ID, 10, 64)
	if parseErr != nil {
		t.Fatalf("failed to parse existing user ID: %v", parseErr)
	}
	bindID := tc.StubUserInfo.GhBind

	lastLogin, _ := tc.StubExistingUser.LastLoginTime()
	createdAt, _ := tc.StubExistingUser.CreatedAtTime()

	repo := newStubUserRepo()
	repo.findByGithubBindID = func(_ context.Context, ghBindID string) (*aggregate.User, error) {
		if ghBindID == bindID {
			user := aggregate.RestoreUser(
				uint(existingUserID),
				identityvo.UserName(tc.StubExistingUser.Name),
				identityvo.Email(tc.StubExistingUser.Email),
				identityvo.Avatar(tc.StubExistingUser.Avatar),
				enum.PermissionUser,
				lastLogin,
				createdAt,
				bindID,
				"",
			)
			return user, nil
		}
		return nil, nil
	}

	touchCalled := false
	repo.touchLastLogin = func(_ context.Context, userID uint) error {
		touchCalled = true
		if userID != uint(existingUserID) {
			t.Errorf("TouchLastLogin userID = %d, want %d", userID, existingUserID)
		}
		return nil
	}

	handler := makeHandler(repo, &tc)

	result, err := handler.Handle(ctx, command.HandleCallbackCommand{
		Platform: tc.Platform,
		Code:     tc.Code,
		State:    state,
	})
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if result == nil {
		t.Fatal("Handle() returned nil result")
	}
	if result.UserID != uint(existingUserID) {
		t.Errorf("UserID = %d, want %d", result.UserID, existingUserID)
	}
	if result.IsNewUser {
		t.Error("IsNewUser = true, want false for existing user")
	}
	if !touchCalled {
		t.Error("TouchLastLogin was not called for existing user")
	}
}

// TestHandleCallback_InvalidPlatform 无效平台应返回错误
func TestHandleCallback_InvalidPlatform(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "callback_invalid_platform")
	ctx := context.Background()

	state, err := infraoauth2.GenerateOAuth2State()
	if err != nil {
		t.Fatalf("GenerateOAuth2State() error: %v", err)
	}

	userRepo := newStubUserRepo()
	handler := makeHandler(userRepo, &tc)

	_, err = handler.Handle(ctx, command.HandleCallbackCommand{
		Platform: tc.Platform,
		Code:     tc.Code,
		State:    state,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got: %v", err)
	}
}

// TestHandleCallback_InvalidState 无效 state 应返回 Unauthorized 错误
func TestHandleCallback_InvalidState(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "callback_invalid_state")
	ctx := context.Background()

	userRepo := newStubUserRepo()
	handler := makeHandler(userRepo, &tc)

	_, err := handler.Handle(ctx, command.HandleCallbackCommand{
		Platform: tc.Platform,
		Code:     tc.Code,
		State:    tc.State,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}
