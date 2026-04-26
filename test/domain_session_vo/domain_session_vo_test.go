// Package domain_session_vo 验证 domain/session/vo 值对象的行为
package domain_session_vo

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
)

// sessionVOCase 会话值对象测试用例结构
type sessionVOCase struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Text          string  `json:"text"`
	Error         string  `json:"error"`
	Coherence     float64 `json:"coherence"`
	Depth         float64 `json:"depth"`
	Value         float64 `json:"value"`
	Total         float64 `json:"total"`
	ExpectedTotal float64 `json:"expected_total"`
	Version       string  `json:"version"`
	ExpectFailed  bool    `json:"expect_failed"`
	ExpectEmpty   bool    `json:"expect_empty"`
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

// TestNewSessionSummary_Valid 验证有效摘要的构造函数和 getter
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
	if s.Failed() != tc.ExpectFailed {
		t.Errorf("Failed() = %v, want %v", s.Failed(), tc.ExpectFailed)
	}
	if s.IsEmpty() != tc.ExpectEmpty {
		t.Errorf("IsEmpty() = %v, want %v", s.IsEmpty(), tc.ExpectEmpty)
	}
}

// TestNewSessionSummary_Failed 验证失败状态摘要
func TestNewSessionSummary_Failed(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_summary_failed")

	s := vo.NewSessionSummary(tc.Text, tc.Error)
	if s.Text() != tc.Text {
		t.Errorf("Text() = %q, want %q", s.Text(), tc.Text)
	}
	if s.Error() != tc.Error {
		t.Errorf("Error() = %q, want %q", s.Error(), tc.Error)
	}
	if !s.Failed() {
		t.Error("Failed() = false, want true")
	}
	if !s.IsEmpty() {
		t.Error("IsEmpty() = false, want true for empty text")
	}
}

// TestNewSessionSummary_EmptyText 空文本应被识别为 empty
func TestNewSessionSummary_EmptyText(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_summary_empty_text")

	s := vo.NewSessionSummary(tc.Text, tc.Error)
	if !s.IsEmpty() {
		t.Error("IsEmpty() = false, want true for empty text")
	}
}

// TestNewSessionSummary_WhitespaceText 空白文本应被识别为 empty
func TestNewSessionSummary_WhitespaceText(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_summary_whitespace")

	s := vo.NewSessionSummary(tc.Text, tc.Error)
	if !s.IsEmpty() {
		t.Error("IsEmpty() = false, want true for whitespace text")
	}
}

// TestNewSessionScore_Total 验证 score 的 total 是三维均值
func TestNewSessionScore_Total(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_score_valid")

	now := time.Now().UTC()
	s, err := vo.NewSessionScore(tc.Coherence, tc.Depth, tc.Value, tc.Version, now)
	if err != nil {
		t.Fatalf("NewSessionScore() error: %v", err)
	}

	if s.Coherence() != tc.Coherence {
		t.Errorf("Coherence() = %f, want %f", s.Coherence(), tc.Coherence)
	}
	if s.Depth() != tc.Depth {
		t.Errorf("Depth() = %f, want %f", s.Depth(), tc.Depth)
	}
	if s.Value() != tc.Value {
		t.Errorf("Value() = %f, want %f", s.Value(), tc.Value)
	}

	// 比较浮点数
	gotTotal := s.Total()
	if math.Abs(gotTotal-tc.ExpectedTotal) > 0.0001 {
		t.Errorf("Total() = %f, want %f", gotTotal, tc.ExpectedTotal)
	}

	if s.Version() != tc.Version {
		t.Errorf("Version() = %q, want %q", s.Version(), tc.Version)
	}
	if s.At() == nil {
		t.Fatal("At() should not be nil for NewSessionScore")
	}
	if !s.At().Equal(now) {
		t.Errorf("At() = %v, want %v", s.At(), now)
	}
	if s.Failed() != tc.ExpectFailed {
		t.Errorf("Failed() = %v, want %v", s.Failed(), tc.ExpectFailed)
	}
}

// TestNewSessionScore_Zero 全零 score 应返回验证错误（范围 [1,10]）
func TestNewSessionScore_Zero(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_score_zero")

	_, err := vo.NewSessionScore(tc.Coherence, tc.Depth, tc.Value, tc.Version, time.Now().UTC())
	if err == nil {
		t.Error("NewSessionScore() with zero values should return validation error, got nil")
	}
}

// TestNewFailedSessionScore 失败评分应设置 error 和时间
func TestNewFailedSessionScore(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "session_score_failed")

	now := time.Now().UTC()
	s := vo.NewFailedSessionScore(tc.Error, now)

	if s.Error() != tc.Error {
		t.Errorf("Error() = %q, want %q", s.Error(), tc.Error)
	}
	if !s.Failed() {
		t.Error("Failed() = false, want true")
	}
	if s.At() == nil {
		t.Fatal("At() should not be nil for NewFailedSessionScore")
	}
	if !s.At().Equal(now) {
		t.Errorf("At() = %v, want %v", s.At(), now)
	}
}

// TestRestoreSessionScore 从持久化数据重建 score 应保留所有字段
func TestRestoreSessionScore(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "restore_session_score")

	now := time.Now().UTC()
	s := vo.RestoreSessionScore(tc.Coherence, tc.Depth, tc.Value, tc.Total, tc.Version, &now, tc.Error)

	if s.Coherence() != tc.Coherence {
		t.Errorf("Coherence() = %f, want %f", s.Coherence(), tc.Coherence)
	}
	if s.Depth() != tc.Depth {
		t.Errorf("Depth() = %f, want %f", s.Depth(), tc.Depth)
	}
	if s.Value() != tc.Value {
		t.Errorf("Value() = %f, want %f", s.Value(), tc.Value)
	}
	if s.Total() != tc.Total {
		t.Errorf("Total() = %f, want %f", s.Total(), tc.Total)
	}
	if s.Version() != tc.Version {
		t.Errorf("Version() = %q, want %q", s.Version(), tc.Version)
	}
	if s.At() == nil {
		t.Fatal("At() should not be nil for RestoreSessionScore")
	}
	if !s.At().Equal(now) {
		t.Errorf("At() = %v, want %v", s.At(), now)
	}
	if s.Error() != tc.Error {
		t.Errorf("Error() = %q, want %q", s.Error(), tc.Error)
	}
	if s.Failed() != tc.ExpectFailed {
		t.Errorf("Failed() = %v, want %v", s.Failed(), tc.ExpectFailed)
	}
}

// TestRestoreSessionScore_NilAt nil At 应与 IsEmpty 兼容
func TestRestoreSessionScore_NilAt(t *testing.T) {
	s := vo.RestoreSessionScore(0, 0, 0, 0, "", nil, "")

	if s.At() != nil {
		t.Error("At() should be nil for nil parameter")
	}
	if s.Error() != "" {
		t.Errorf("Error() = %q, want empty", s.Error())
	}
}
