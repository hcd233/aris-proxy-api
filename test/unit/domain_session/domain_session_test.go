// Package domain_session 验证 domain/session/aggregate.Session 聚合根的行为
package domain_session

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
)

// sessionCase 会话测试用例结构
type sessionCase struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Owner           string            `json:"owner"`
	MessageIDs      []uint            `json:"messageIDs"`
	ToolIDs         []uint            `json:"toolIDs"`
	Metadata        map[string]string `json:"metadata"`
	ID              uint              `json:"id"`
	SummaryText     string            `json:"summary_text"`
	SummaryError    string            `json:"summary_error"`
	Score           *int              `json:"score"`
	CheckOwner      string            `json:"check_owner"`
	ExpectOwned     bool              `json:"expect_owned"`
	ExpectErrorKind string            `json:"expectErrorKind"`
}

func loadCases(t *testing.T) []sessionCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []sessionCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

func findCase(t *testing.T, cases []sessionCase, name string) sessionCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return sessionCase{}
}

// TestCreateSession_ValidOwner 用非空 owner 创建 Session 应成功
func TestCreateSession_ValidOwner(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)
	tc := findCase(t, cases, "create_session_valid_owner")

	s, err := aggregate.CreateSession(
		vo.APIKeyOwner(tc.Owner),
		tc.MessageIDs,
		tc.ToolIDs,
		tc.Metadata,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}
	if s == nil {
		t.Fatal("CreateSession() returned nil")
	}
	if s.Owner().String() != tc.Owner {
		t.Errorf("Owner = %q, want %q", s.Owner().String(), tc.Owner)
	}
}

// TestCreateSession_EmptyOwner 空 owner 应返回校验错误
func TestCreateSession_EmptyOwner(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)
	tc := findCase(t, cases, "create_session_empty_owner")

	s, err := aggregate.CreateSession(
		vo.APIKeyOwner(tc.Owner),
		tc.MessageIDs,
		tc.ToolIDs,
		tc.Metadata,
		time.Now().UTC(),
	)
	if err == nil {
		t.Fatal("CreateSession() expected error, got nil")
	}
	if s != nil {
		t.Errorf("CreateSession() expected nil result, got %+v", s)
	}
	if !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("error kind = %v, want ErrValidation", err)
	}
}

// TestRestoreSession 从持久化数据重建 Session 应正确还原所有字段
func TestRestoreSession(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)
	tc := findCase(t, cases, "restore_session_full")

	now := time.Now().UTC()
	score := vo.RestoreSessionScore(tc.Score, &now)

	s := aggregate.RestoreSession(
		tc.ID,
		vo.APIKeyOwner(tc.Owner),
		tc.MessageIDs,
		tc.ToolIDs,
		tc.Metadata,
		score,
		now,
		now,
	)

	if s == nil {
		t.Fatal("RestoreSession() returned nil")
		return
	}
	if s.AggregateID() != tc.ID {
		t.Errorf("AggregateID = %d, want %d", s.AggregateID(), tc.ID)
	}
	if s.Owner().String() != tc.Owner {
		t.Errorf("Owner = %q, want %q", s.Owner().String(), tc.Owner)
	}
	if s.Score().IsEmpty() && tc.Score != nil {
		t.Errorf("Score.IsEmpty() = true, want false")
	}
	if *s.Score().Score() != *tc.Score {
		t.Errorf("Score = %d, want %d", *s.Score().Score(), *tc.Score)
	}
}

// TestUpdateScore_Valid 更新有效评分应正确设置各维度值
func TestUpdateScore_Valid(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)
	tc := findCase(t, cases, "update_score_valid")

	s, err := aggregate.CreateSession(vo.APIKeyOwner(tc.Owner), nil, nil, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	score, err := vo.NewSessionScore(*tc.Score, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewSessionScore() error: %v", err)
	}
	s.UpdateScore(score, time.Now().UTC())

	if s.Score().IsEmpty() {
		t.Error("Score.IsEmpty() = true, want false")
	}
	if *s.Score().Score() != *tc.Score {
		t.Errorf("Score = %d, want %d", *s.Score().Score(), *tc.Score)
	}
}

// TestIsOwnedBy_Matching 匹配 owner 应返回 true
func TestIsOwnedBy_Matching(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)
	tc := findCase(t, cases, "is_owned_by_matching")

	s, err := aggregate.CreateSession(vo.APIKeyOwner(tc.Owner), nil, nil, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	if !s.IsOwnedBy(tc.CheckOwner) {
		t.Errorf("IsOwnedBy(%q) = false, want true", tc.CheckOwner)
	}
}

// TestIsOwnedBy_NonMatching 不匹配 owner 应返回 false
func TestIsOwnedBy_NonMatching(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)
	tc := findCase(t, cases, "is_owned_by_non_matching")

	s, err := aggregate.CreateSession(vo.APIKeyOwner(tc.Owner), nil, nil, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	if s.IsOwnedBy(tc.CheckOwner) {
		t.Errorf("IsOwnedBy(%q) = true, want false", tc.CheckOwner)
	}
}
