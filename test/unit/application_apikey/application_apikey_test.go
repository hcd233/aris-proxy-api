// Package application_apikey 针对 application/apikey/command 的单元测试
package application_apikey

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/apikey/command"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/vo"
)

func mustAPIKeySecret(raw string) vo.APIKeySecret {
	secret, err := vo.NewAPIKeySecret(raw)
	if err != nil {
		panic(err)
	}
	return secret
}

func restoreProxyAPIKeyForTest(id, userID uint, name, secret string, createdAt time.Time) *aggregate.ProxyAPIKey {
	return aggregate.RestoreProxyAPIKey(id, userID, vo.APIKeyName(name), mustAPIKeySecret(secret), createdAt)
}

// mockAPIKeyRepository mock APIKeyRepository
type mockAPIKeyRepository struct {
	saveFunc    func(ctx context.Context, key *aggregate.ProxyAPIKey) error
	countByUser func(ctx context.Context, userID uint) (int64, error)
	saveCalled  bool
	savedKey    *aggregate.ProxyAPIKey
}

func (m *mockAPIKeyRepository) Save(ctx context.Context, key *aggregate.ProxyAPIKey) error {
	m.saveCalled = true
	m.savedKey = key
	if m.saveFunc != nil {
		return m.saveFunc(ctx, key)
	}
	return nil
}

func (m *mockAPIKeyRepository) FindByID(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
	return nil, nil
}

func (m *mockAPIKeyRepository) ListByUser(ctx context.Context, userID uint) ([]*aggregate.ProxyAPIKey, error) {
	return nil, nil
}

func (m *mockAPIKeyRepository) ListAll(ctx context.Context) ([]*aggregate.ProxyAPIKey, error) {
	return nil, nil
}

func (m *mockAPIKeyRepository) CountByUser(ctx context.Context, userID uint) (int64, error) {
	if m.countByUser != nil {
		return m.countByUser(ctx, userID)
	}
	return 0, nil
}

func (m *mockAPIKeyRepository) Delete(ctx context.Context, id uint) error {
	return nil
}

// mockAPIKeyGenerator mock APIKeyGenerator
type mockAPIKeyGenerator struct {
	generateFunc func() (vo.APIKeySecret, error)
}

func (m *mockAPIKeyGenerator) Generate() (vo.APIKeySecret, error) {
	if m.generateFunc != nil {
		return m.generateFunc()
	}
	return mustAPIKeySecret("sk-aris-default"), nil
}

// mockUserExistenceChecker mock UserExistenceChecker
type mockUserExistenceChecker struct {
	existsFunc func(ctx context.Context, userID uint) (bool, error)
}

func (m *mockUserExistenceChecker) Exists(ctx context.Context, userID uint) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, userID)
	}
	return true, nil
}

// TestIssueAPIKeyHandler_HappyPath 验证成功签发场景
func TestIssueAPIKeyHandler_HappyPath(t *testing.T) {
	repo := &mockAPIKeyRepository{
		countByUser: func(ctx context.Context, userID uint) (int64, error) {
			return 3, nil
		},
	}
	generator := &mockAPIKeyGenerator{
		generateFunc: func() (vo.APIKeySecret, error) {
			return mustAPIKeySecret("sk-aris-test123"), nil
		},
	}
	userExistsCh := &mockUserExistenceChecker{
		existsFunc: func(ctx context.Context, userID uint) (bool, error) {
			return true, nil
		},
	}

	handler := command.NewIssueAPIKeyHandler(repo, generator, userExistsCh)

	result, err := handler.Handle(context.Background(), command.IssueAPIKeyCommand{
		UserID: 1,
		Name:   "my-key",
	})

	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Name != "my-key" {
		t.Errorf("Name = %q, want %q", result.Name, "my-key")
	}
	if result.Secret != "sk-aris-test123" {
		t.Errorf("Secret = %q, want %q", result.Secret, "sk-aris-test123")
	}
	if !repo.saveCalled {
		t.Error("expected Save to be called")
	}
}

// TestIssueAPIKeyHandler_QuotaExceeded 验证配额超限
func TestIssueAPIKeyHandler_QuotaExceeded(t *testing.T) {
	repo := &mockAPIKeyRepository{
		countByUser: func(ctx context.Context, userID uint) (int64, error) {
			return 5, nil // 已达配额
		},
	}
	generator := &mockAPIKeyGenerator{}
	userExistsCh := &mockUserExistenceChecker{}

	handler := command.NewIssueAPIKeyHandler(repo, generator, userExistsCh)

	_, err := handler.Handle(context.Background(), command.IssueAPIKeyCommand{
		UserID: 1,
		Name:   "my-key",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrQuotaExceeded) {
		t.Errorf("expected ErrQuotaExceeded, got: %v", err)
	}
}

// TestIssueAPIKeyHandler_UserNotFound 验证用户不存在
func TestIssueAPIKeyHandler_UserNotFound(t *testing.T) {
	repo := &mockAPIKeyRepository{}
	generator := &mockAPIKeyGenerator{}
	userExistsCh := &mockUserExistenceChecker{
		existsFunc: func(ctx context.Context, userID uint) (bool, error) {
			return false, nil
		},
	}

	handler := command.NewIssueAPIKeyHandler(repo, generator, userExistsCh)

	_, err := handler.Handle(context.Background(), command.IssueAPIKeyCommand{
		UserID: 999,
		Name:   "my-key",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Errorf("expected ErrDataNotExists, got: %v", err)
	}
}

// TestIssueAPIKeyHandler_Validation 验证参数校验
func TestIssueAPIKeyHandler_Validation(t *testing.T) {
	tests := []struct {
		name        string
		nameInput   string
		secretValue string
		expectErr   string
	}{
		{
			name:        "empty_name",
			nameInput:   "",
			secretValue: "sk-aris-test",
			expectErr:   "validation",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockAPIKeyRepository{}
			generator := &mockAPIKeyGenerator{
				generateFunc: func() (vo.APIKeySecret, error) {
					return mustAPIKeySecret(tc.secretValue), nil
				},
			}
			userExistsCh := &mockUserExistenceChecker{}

			handler := command.NewIssueAPIKeyHandler(repo, generator, userExistsCh)

			_, err := handler.Handle(context.Background(), command.IssueAPIKeyCommand{
				UserID: 1,
				Name:   tc.nameInput,
			})

			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, ierr.ErrValidation) {
				t.Errorf("expected ErrValidation, got: %v", err)
			}
		})
	}
}

// TestIssueAPIKeyHandler_EmptySecret 验证空密钥时使用 generator 返回值
func TestIssueAPIKeyHandler_EmptySecretFromGenerator(t *testing.T) {
	repo := &mockAPIKeyRepository{}
	generator := &mockAPIKeyGenerator{
		generateFunc: func() (vo.APIKeySecret, error) {
			return vo.APIKeySecret{}, nil // generator 返回空密钥
		},
	}
	userExistsCh := &mockUserExistenceChecker{}

	handler := command.NewIssueAPIKeyHandler(repo, generator, userExistsCh)

	_, err := handler.Handle(context.Background(), command.IssueAPIKeyCommand{
		UserID: 1,
		Name:   "my-key",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("expected ErrValidation for empty secret, got: %v", err)
	}
}

// TestIssueAPIKeyHandler_NilUserChecker 验证 userExistsCh 为 nil 时跳过校验
func TestIssueAPIKeyHandler_NilUserChecker(t *testing.T) {
	repo := &mockAPIKeyRepository{
		countByUser: func(ctx context.Context, userID uint) (int64, error) {
			return 0, nil
		},
	}
	generator := &mockAPIKeyGenerator{
		generateFunc: func() (vo.APIKeySecret, error) {
			return mustAPIKeySecret("sk-aris-test"), nil
		},
	}

	// userExistsCh 为 nil，跳过用户存在性校验
	handler := command.NewIssueAPIKeyHandler(repo, generator, nil)

	result, err := handler.Handle(context.Background(), command.IssueAPIKeyCommand{
		UserID: 999, // 不存在的用户 ID，但会被跳过
		Name:   "my-key",
	})

	if err != nil {
		t.Fatalf("expected success when user checker is nil, got err: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Name != "my-key" {
		t.Errorf("Name = %q, want %q", result.Name, "my-key")
	}
}

// TestIssueAPIKeyHandler_RepoError 验证仓储错误传播
func TestIssueAPIKeyHandler_RepoError(t *testing.T) {
	repoErr := errors.New("database error")
	repo := &mockAPIKeyRepository{
		countByUser: func(ctx context.Context, userID uint) (int64, error) {
			return 0, repoErr
		},
	}
	generator := &mockAPIKeyGenerator{}
	userExistsCh := &mockUserExistenceChecker{}

	handler := command.NewIssueAPIKeyHandler(repo, generator, userExistsCh)

	_, err := handler.Handle(context.Background(), command.IssueAPIKeyCommand{
		UserID: 1,
		Name:   "my-key",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != repoErr {
		t.Errorf("expected repoErr, got: %v", err)
	}
}

// TestIssueAPIKeyHandler_GeneratorError 验证生成器错误传播
func TestIssueAPIKeyHandler_GeneratorError(t *testing.T) {
	genErr := errors.New("random generation failed")
	repo := &mockAPIKeyRepository{
		countByUser: func(ctx context.Context, userID uint) (int64, error) {
			return 0, nil
		},
	}
	generator := &mockAPIKeyGenerator{
		generateFunc: func() (vo.APIKeySecret, error) {
			return vo.APIKeySecret{}, genErr
		},
	}
	userExistsCh := &mockUserExistenceChecker{}

	handler := command.NewIssueAPIKeyHandler(repo, generator, userExistsCh)

	_, err := handler.Handle(context.Background(), command.IssueAPIKeyCommand{
		UserID: 1,
		Name:   "my-key",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != genErr {
		t.Errorf("expected genErr, got: %v", err)
	}
}

// TestIssueAPIKeyHandler_SaveSetsID 验证 Save 后回填 ID
func TestIssueAPIKeyHandler_SaveSetsID(t *testing.T) {
	repo := &mockAPIKeyRepository{
		countByUser: func(ctx context.Context, userID uint) (int64, error) {
			return 0, nil
		},
		saveFunc: func(ctx context.Context, key *aggregate.ProxyAPIKey) error {
			// 模拟数据库回填 ID
			key.SetID(42)
			return nil
		},
	}
	generator := &mockAPIKeyGenerator{
		generateFunc: func() (vo.APIKeySecret, error) {
			return mustAPIKeySecret("sk-aris-test"), nil
		},
	}
	userExistsCh := &mockUserExistenceChecker{}

	handler := command.NewIssueAPIKeyHandler(repo, generator, userExistsCh)

	result, err := handler.Handle(context.Background(), command.IssueAPIKeyCommand{
		UserID: 1,
		Name:   "my-key",
	})

	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if result.KeyID != 42 {
		t.Errorf("KeyID = %d, want %d", result.KeyID, 42)
	}
}

// nowTime 返回测试用稳定时间
func nowTime() time.Time {
	return time.Unix(1700000000, 0).UTC()
}

// mockRevokeAPIKeyRepository mock Repository for revoke tests
type mockRevokeAPIKeyRepository struct {
	findByIDFunc func(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error)
	deleteFunc   func(ctx context.Context, id uint) error
	deleteCalled bool
}

func (m *mockRevokeAPIKeyRepository) Save(ctx context.Context, key *aggregate.ProxyAPIKey) error {
	return nil
}

func (m *mockRevokeAPIKeyRepository) FindByID(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockRevokeAPIKeyRepository) ListByUser(ctx context.Context, userID uint) ([]*aggregate.ProxyAPIKey, error) {
	return nil, nil
}

func (m *mockRevokeAPIKeyRepository) ListAll(ctx context.Context) ([]*aggregate.ProxyAPIKey, error) {
	return nil, nil
}

func (m *mockRevokeAPIKeyRepository) CountByUser(ctx context.Context, userID uint) (int64, error) {
	return 0, nil
}

func (m *mockRevokeAPIKeyRepository) Delete(ctx context.Context, id uint) error {
	m.deleteCalled = true
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

// TestRevokeAPIKeyHandler_OwnerSuccess 验证所有者成功吊销
func TestRevokeAPIKeyHandler_OwnerSuccess(t *testing.T) {
	repo := &mockRevokeAPIKeyRepository{
		findByIDFunc: func(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
			return restoreProxyAPIKeyForTest(1, 101, "my-key", "sk-aris-test", nowTime()), nil
		},
	}

	handler := command.NewRevokeAPIKeyHandler(repo)

	err := handler.Handle(context.Background(), command.RevokeAPIKeyCommand{
		KeyID:               1,
		RequesterID:         101,
		RequesterPermission: enum.PermissionUser,
	})

	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if !repo.deleteCalled {
		t.Error("expected Delete to be called")
	}
}

// TestRevokeAPIKeyHandler_AdminSuccess 验证 admin 成功吊销他人 Key
func TestRevokeAPIKeyHandler_AdminSuccess(t *testing.T) {
	repo := &mockRevokeAPIKeyRepository{
		findByIDFunc: func(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
			return restoreProxyAPIKeyForTest(1, 999, "other-key", "sk-aris-other", nowTime()), nil
		},
	}

	handler := command.NewRevokeAPIKeyHandler(repo)

	err := handler.Handle(context.Background(), command.RevokeAPIKeyCommand{
		KeyID:               1,
		RequesterID:         1, // admin
		RequesterPermission: enum.PermissionAdmin,
	})

	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
}

// TestRevokeAPIKeyHandler_NotFound 验证 Key 不存在
func TestRevokeAPIKeyHandler_NotFound(t *testing.T) {
	repo := &mockRevokeAPIKeyRepository{
		findByIDFunc: func(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
			return nil, nil // not found
		},
	}

	handler := command.NewRevokeAPIKeyHandler(repo)

	err := handler.Handle(context.Background(), command.RevokeAPIKeyCommand{
		KeyID:               999,
		RequesterID:         101,
		RequesterPermission: enum.PermissionUser,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Errorf("expected ErrDataNotExists, got: %v", err)
	}
}

// TestRevokeAPIKeyHandler_NoPermission 验证无权限吊销
func TestRevokeAPIKeyHandler_NoPermission(t *testing.T) {
	repo := &mockRevokeAPIKeyRepository{
		findByIDFunc: func(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
			return restoreProxyAPIKeyForTest(1, 999, "other-key", "sk-aris-other", nowTime()), nil
		},
	}

	handler := command.NewRevokeAPIKeyHandler(repo)

	err := handler.Handle(context.Background(), command.RevokeAPIKeyCommand{
		KeyID:               1,
		RequesterID:         101, // 不是所有者
		RequesterPermission: enum.PermissionUser,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrNoPermission) {
		t.Errorf("expected ErrNoPermission, got: %v", err)
	}
}

// TestRevokeAPIKeyHandler_LegacyKeyNoPermission 验证 legacy Key 禁止普通用户吊销
func TestRevokeAPIKeyHandler_LegacyKeyNoPermission(t *testing.T) {
	repo := &mockRevokeAPIKeyRepository{
		findByIDFunc: func(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
			return restoreProxyAPIKeyForTest(1, 0, "legacy-key", "sk-aris-legacy", nowTime()), nil
		},
	}

	handler := command.NewRevokeAPIKeyHandler(repo)

	err := handler.Handle(context.Background(), command.RevokeAPIKeyCommand{
		KeyID:               1,
		RequesterID:         101,
		RequesterPermission: enum.PermissionUser,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ierr.ErrNoPermission) {
		t.Errorf("expected ErrNoPermission for legacy key, got: %v", err)
	}
}

// TestRevokeAPIKeyHandler_RepoError 验证仓储错误传播
func TestRevokeAPIKeyHandler_RepoError(t *testing.T) {
	repoErr := errors.New("database error")
	repo := &mockRevokeAPIKeyRepository{
		findByIDFunc: func(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
			return nil, repoErr
		},
	}

	handler := command.NewRevokeAPIKeyHandler(repo)

	err := handler.Handle(context.Background(), command.RevokeAPIKeyCommand{
		KeyID:               1,
		RequesterID:         101,
		RequesterPermission: enum.PermissionUser,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != repoErr {
		t.Errorf("expected repoErr, got: %v", err)
	}
}

// TestRevokeAPIKeyHandler_DeleteError 验证删除错误传播
func TestRevokeAPIKeyHandler_DeleteError(t *testing.T) {
	deleteErr := errors.New("delete failed")
	repo := &mockRevokeAPIKeyRepository{
		findByIDFunc: func(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
			return restoreProxyAPIKeyForTest(1, 101, "my-key", "sk-aris-test", nowTime()), nil
		},
		deleteFunc: func(ctx context.Context, id uint) error {
			return deleteErr
		},
	}

	handler := command.NewRevokeAPIKeyHandler(repo)

	err := handler.Handle(context.Background(), command.RevokeAPIKeyCommand{
		KeyID:               1,
		RequesterID:         101,
		RequesterPermission: enum.PermissionUser,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != deleteErr {
		t.Errorf("expected deleteErr, got: %v", err)
	}
}
