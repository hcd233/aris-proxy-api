// Package domain_apikey 针对 domain/apikey 聚合的单元测试
package domain_apikey

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/vo"
)

type issueCase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	UserID      uint   `json:"userID"`
	KeyName     string `json:"keyName"`
	Secret      string `json:"secret"`
	QuotaMax    int    `json:"quotaMax"`
	Existing    int64  `json:"existing"`
	ExpectErr   string `json:"expectErr"`
}

func loadIssueCases(t *testing.T) []issueCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/issue_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures: %v", err)
	}
	var cases []issueCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures: %v", err)
	}
	return cases
}

// TestIssueProxyAPIKey 验证签发聚合的 happy path + 3 个边界
func TestIssueProxyAPIKey(t *testing.T) {
	cases := loadIssueCases(t)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			secret, err := vo.NewAPIKeySecret(tc.Secret)
			if err != nil {
				if tc.ExpectErr == "validation" {
					if !errors.Is(err, ierr.ErrValidation) {
						t.Errorf("expected ErrValidation from NewAPIKeySecret, got: %v", err)
					}
					return
				}
				t.Fatalf("unexpected NewAPIKeySecret error: %v", err)
			}
			quota, err := vo.NewAPIKeyQuota(tc.QuotaMax)
			if err != nil {
				if tc.ExpectErr == "validation" {
					if !errors.Is(err, ierr.ErrValidation) {
						t.Errorf("expected ErrValidation from NewAPIKeyQuota, got: %v", err)
					}
					return
				}
				t.Fatalf("unexpected NewAPIKeyQuota error: %v", err)
			}

			key, err := aggregate.IssueProxyAPIKey(
				tc.UserID,
				vo.APIKeyName(tc.KeyName),
				secret,
				quota,
				tc.Existing,
				time.Now(),
			)

			switch tc.ExpectErr {
			case "":
				if err != nil {
					t.Fatalf("expected success, got err: %v", err)
				}
				if key == nil {
					t.Fatal("expected key, got nil")
				}
				if key.UserID() != tc.UserID {
					t.Errorf("UserID = %d, want %d", key.UserID(), tc.UserID)
				}
				if key.Name().String() != tc.KeyName {
					t.Errorf("Name = %q, want %q", key.Name().String(), tc.KeyName)
				}
				if key.AggregateType() != constant.AggregateTypeAPIKey {
					t.Errorf("AggregateType = %q, want %q", key.AggregateType(), constant.AggregateTypeAPIKey)
				}
			case "quota_exceeded":
				if !errors.Is(err, ierr.ErrQuotaExceeded) {
					t.Errorf("expected ErrQuotaExceeded, got: %v", err)
				}
				if key != nil {
					t.Errorf("expected nil key on error, got %+v", key)
				}
			case "validation":
				if !errors.Is(err, ierr.ErrValidation) {
					t.Errorf("expected ErrValidation, got: %v", err)
				}
				if key != nil {
					t.Errorf("expected nil key on error, got %+v", key)
				}
			default:
				t.Fatalf("unknown expectErr: %q", tc.ExpectErr)
			}
		})
	}
}

// TestProxyAPIKey_IsOwnedBy 验证所有权判定：严格匹配 UserID；
// UserID==0 的 legacy key 不再被视为任何普通用户所有，必须走 admin 分支。
func TestProxyAPIKey_IsOwnedBy(t *testing.T) {
	secX, err := vo.NewAPIKeySecret("x")
	if err != nil {
		t.Fatalf("failed to create secret x: %v", err)
	}
	secY, err := vo.NewAPIKeySecret("y")
	if err != nil {
		t.Fatalf("failed to create secret y: %v", err)
	}
	secZ, err := vo.NewAPIKeySecret("z")
	if err != nil {
		t.Fatalf("failed to create secret z: %v", err)
	}
	own := aggregate.RestoreProxyAPIKey(1, 101, "a", secX, nowTime(t))
	legacy := aggregate.RestoreProxyAPIKey(2, 0, "b", secY, nowTime(t))
	other := aggregate.RestoreProxyAPIKey(3, 999, "c", secZ, nowTime(t))

	if !own.IsOwnedBy(101) {
		t.Error("own key should be owned by user 101")
	}
	if legacy.IsOwnedBy(101) {
		t.Error("legacy key (UserID=0) must NOT be treated as owned by normal user 101")
	}
	if legacy.IsOwnedBy(0) {
		t.Error("legacy key (UserID=0) must NOT match userID=0 either (guard against uninitialized ctx)")
	}
	if other.IsOwnedBy(101) {
		t.Error("other user's key should not be owned by user 101")
	}
}

// TestAPIKeySecret_Masked 验证脱敏输出稳定
func TestAPIKeySecret_Masked(t *testing.T) {
	s, err := vo.NewAPIKeySecret("sk-abcdefghijklmnop")
	if err != nil {
		t.Fatalf("failed to create secret: %v", err)
	}
	if s.Raw() != "sk-abcdefghijklmnop" {
		t.Errorf("Raw = %q, want original", s.Raw())
	}
	if s.Masked() == s.Raw() {
		t.Error("Masked should not equal Raw")
	}
	if s.IsEmpty() {
		t.Error("expected not empty")
	}
}

// nowTime 返回测试用稳定时间
func nowTime(t *testing.T) (now time.Time) {
	t.Helper()
	return time.Unix(1700000000, 0).UTC()
}
