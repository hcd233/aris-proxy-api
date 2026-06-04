// Package domain_session_vo 验证 domain/session/vo 值对象的行为
package domain_session_vo

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
)

// sessionVOCase 会话值对象测试用例结构
type sessionVOCase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Text        string `json:"text"`
	Error       string `json:"error"`
	Score       *int   `json:"score"`
	ExpectEmpty bool   `json:"expect_empty"`
}

func loadCases(t *testing.T) []sessionVOCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []sessionVOCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

func findCase(t *testing.T, cases []sessionVOCase, name string) sessionVOCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return sessionVOCase{}
}

func TestNewSessionSummary_Valid(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_summary_valid")

	s := vo.NewSessionSummary(tc.Text, tc.Error)
	if s.Text() != tc.Text {
		t.Errorf("Text() = %q, want %q", s.Text(), tc.Text)
	}
	if s.Error() != tc.Error {
		t.Errorf("Error() = %q, want %q", s.Error(), tc.Error)
	}
	if s.IsEmpty() != tc.ExpectEmpty {
		t.Errorf("IsEmpty() = %v, want %v", s.IsEmpty(), tc.ExpectEmpty)
	}
}

func TestNewSessionSummary_Failed(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_summary_failed")

	s := vo.NewSessionSummary(tc.Text, tc.Error)
	if !s.Failed() {
		t.Error("Failed() = false, want true")
	}
	if !s.IsEmpty() {
		t.Error("IsEmpty() = false, want true for empty text")
	}
}

func TestNewSessionSummary_EmptyText(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_summary_empty_text")

	s := vo.NewSessionSummary(tc.Text, tc.Error)
	if !s.IsEmpty() {
		t.Error("IsEmpty() = false, want true for empty text")
	}
}

func TestNewSessionSummary_WhitespaceText(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_summary_whitespace")

	s := vo.NewSessionSummary(tc.Text, tc.Error)
	if !s.IsEmpty() {
		t.Error("IsEmpty() = false, want true for whitespace text")
	}
}

func TestNewSessionScore_Valid(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_score_valid")

	now := time.Now().UTC()
	s, err := vo.NewSessionScore(*tc.Score, now)
	if err != nil {
		t.Fatalf("NewSessionScore() error: %v", err)
	}

	if *s.Score() != *tc.Score {
		t.Errorf("Score() = %d, want %d", *s.Score(), *tc.Score)
	}
	if s.At() == nil {
		t.Fatal("At() should not be nil for NewSessionScore")
	}
	if s.IsEmpty() {
		t.Error("IsEmpty() = true, want false")
	}
}

func TestNewSessionScore_OutOfRange(t *testing.T) {
	now := time.Now().UTC()
	_, err := vo.NewSessionScore(0, now)
	if err == nil {
		t.Fatal("NewSessionScore(0) expected validation error, got nil")
	}
	if !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("expected ErrValidation, got: %v", err)
	}

	_, err = vo.NewSessionScore(6, now)
	if err == nil {
		t.Fatal("NewSessionScore(6) expected validation error, got nil")
	}
	if !errors.Is(err, ierr.ErrValidation) {
		t.Errorf("expected ErrValidation, got: %v", err)
	}
}

func TestRestoreSessionScore(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "restore_session_score")

	now := time.Now().UTC()
	s := vo.RestoreSessionScore(tc.Score, &now)

	if *s.Score() != *tc.Score {
		t.Errorf("Score() = %d, want %d", *s.Score(), *tc.Score)
	}
	if s.At() == nil {
		t.Fatal("At() should not be nil for RestoreSessionScore")
	}
	if s.IsEmpty() {
		t.Error("IsEmpty() = true, want false")
	}
}

func TestRestoreSessionScore_Nil(t *testing.T) {
	s := vo.RestoreSessionScore(nil, nil)

	if s.Score() != nil {
		t.Error("Score() should be nil for nil parameter")
	}
	if s.At() != nil {
		t.Error("At() should be nil for nil parameter")
	}
	if !s.IsEmpty() {
		t.Error("IsEmpty() = false, want true for nil score")
	}
}
