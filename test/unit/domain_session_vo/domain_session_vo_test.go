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

type sessionVOCase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Text        string `json:"text"`
	Error       string `json:"error"`
	Score       *int   `json:"score"`
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

func TestNewSessionScore_Valid(t *testing.T) {
	t.Parallel()
	cases := loadCases(t)
	tc := findCase(t, cases, "session_score_valid")

	now := time.Now().UTC()
	s, err := vo.NewSessionScore(*tc.Score, now)
	if err != nil {
		t.Fatalf("NewSessionScore() error: %v", err)
	}

	if s.Score() != *tc.Score {
		t.Errorf("Score() = %d, want %d", s.Score(), *tc.Score)
	}
	if s.At().IsZero() {
		t.Fatal("At() should not be zero for NewSessionScore")
	}
}

func TestNewSessionScore_OutOfRange(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	cases := loadCases(t)
	tc := findCase(t, cases, "restore_session_score")

	now := time.Now().UTC()
	s := vo.RestoreSessionScore(tc.Score, &now)

	if s.IsAbsent() {
		t.Fatal("RestoreSessionScore() should return Some for non-nil score")
	}
	score, ok := s.Get()
	if !ok {
		t.Fatal("expected Some")
	}
	if score.Score() != *tc.Score {
		t.Errorf("Score() = %d, want %d", score.Score(), *tc.Score)
	}
	if score.At().IsZero() {
		t.Fatal("At() should not be zero for RestoreSessionScore")
	}
}

func TestRestoreSessionScore_Nil(t *testing.T) {
	t.Parallel()
	s := vo.RestoreSessionScore(nil, nil)

	if s.IsPresent() {
		t.Error("RestoreSessionScore(nil) should return None")
	}
}
