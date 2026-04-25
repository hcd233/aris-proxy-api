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
	Name                  string            `json:"name"`
	Description           string            `json:"description"`
	Owner                 string            `json:"owner"`
	MessageIDs            []uint            `json:"messageIDs"`
	ToolIDs               []uint            `json:"toolIDs"`
	Metadata              map[string]string `json:"metadata"`
	ID                    uint              `json:"id"`
	SummaryText           string            `json:"summary_text"`
	SummaryError          string            `json:"summary_error"`
	ScoreCoherence        float64           `json:"score_coherence"`
	ScoreDepth            float64           `json:"score_depth"`
	ScoreValue            float64           `json:"score_value"`
	ScoreTotal            float64           `json:"score_total"`
	ScoreVersion          string            `json:"score_version"`
	ScoreError            string            `json:"score_error"`
	CheckOwner            string            `json:"check_owner"`
	ExpectOwned           bool              `json:"expect_owned"`
	ExpectErrorKind       string            `json:"expectErrorKind"`
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
	if s.AggregateType() != "session.session" {
		t.Errorf("AggregateType = %q, want %q", s.AggregateType(), "session.session")
	}
}

// TestCreateSession_EmptyOwner 空 owner 应返回校验错误
func TestCreateSession_EmptyOwner(t *testing.T) {
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
	cases := loadCases(t)
	tc := findCase(t, cases, "restore_session_full")

	now := time.Now().UTC()
	summary := vo.NewSessionSummary(tc.SummaryText, tc.SummaryError)
	score := vo.RestoreSessionScore(
		tc.ScoreCoherence, tc.ScoreDepth, tc.ScoreValue,
		tc.ScoreTotal, tc.ScoreVersion, &now, tc.ScoreError,
	)

	s := aggregate.RestoreSession(
		tc.ID,
		vo.APIKeyOwner(tc.Owner),
		tc.MessageIDs,
		tc.ToolIDs,
		tc.Metadata,
		summary,
		score,
		now,
		now,
	)

	if s == nil {
		t.Fatal("RestoreSession() returned nil")
	}
	if s.AggregateID() != tc.ID {
		t.Errorf("AggregateID = %d, want %d", s.AggregateID(), tc.ID)
	}
	if s.Owner().String() != tc.Owner {
		t.Errorf("Owner = %q, want %q", s.Owner().String(), tc.Owner)
	}
	if s.Summary().Text() != tc.SummaryText {
		t.Errorf("Summary.Text = %q, want %q", s.Summary().Text(), tc.SummaryText)
	}
	if s.Score().Coherence() != tc.ScoreCoherence {
		t.Errorf("Score.Coherence = %f, want %f", s.Score().Coherence(), tc.ScoreCoherence)
	}
	if s.Score().Depth() != tc.ScoreDepth {
		t.Errorf("Score.Depth = %f, want %f", s.Score().Depth(), tc.ScoreDepth)
	}
	if s.Score().Value() != tc.ScoreValue {
		t.Errorf("Score.Value = %f, want %f", s.Score().Value(), tc.ScoreValue)
	}
}

// TestUpdateSummary_Valid 更新有效摘要应设置摘要并更新时间
func TestUpdateSummary_Valid(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "update_summary_valid")

	s, err := aggregate.CreateSession(vo.APIKeyOwner(tc.Owner), nil, nil, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	summary := vo.NewSessionSummary(tc.SummaryText, tc.SummaryError)
	s.UpdateSummary(summary, time.Now().UTC())

	if s.Summary().Text() != tc.SummaryText {
		t.Errorf("Summary.Text = %q, want %q", s.Summary().Text(), tc.SummaryText)
	}
	if s.Summary().Failed() {
		t.Error("Summary.Failed() = true, want false")
	}
}

// TestUpdateSummary_Failed 更新失败摘要应反映在 Failed() 中
func TestUpdateSummary_Failed(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "update_summary_failed")

	s, err := aggregate.CreateSession(vo.APIKeyOwner(tc.Owner), nil, nil, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	summary := vo.NewSessionSummary(tc.SummaryText, tc.SummaryError)
	s.UpdateSummary(summary, time.Now().UTC())

	if !s.Summary().Failed() {
		t.Error("Summary.Failed() = false, want true for error summary")
	}
	if !s.Summary().IsEmpty() {
		t.Error("Summary.IsEmpty() = false, want true for empty text")
	}
	if s.Summary().Error() != tc.SummaryError {
		t.Errorf("Summary.Error = %q, want %q", s.Summary().Error(), tc.SummaryError)
	}
}

// TestUpdateScore_Valid 更新有效评分应正确设置各维度值
func TestUpdateScore_Valid(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "update_score_valid")

	s, err := aggregate.CreateSession(vo.APIKeyOwner(tc.Owner), nil, nil, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	score := vo.NewSessionScore(tc.ScoreCoherence, tc.ScoreDepth, tc.ScoreValue, tc.ScoreVersion, time.Now().UTC())
	s.UpdateScore(score, time.Now().UTC())

	if s.Score().Coherence() != tc.ScoreCoherence {
		t.Errorf("Score.Coherence = %f, want %f", s.Score().Coherence(), tc.ScoreCoherence)
	}
	if s.Score().Depth() != tc.ScoreDepth {
		t.Errorf("Score.Depth = %f, want %f", s.Score().Depth(), tc.ScoreDepth)
	}
	if s.Score().Value() != tc.ScoreValue {
		t.Errorf("Score.Value = %f, want %f", s.Score().Value(), tc.ScoreValue)
	}
	if s.Score().Version() != tc.ScoreVersion {
		t.Errorf("Score.Version = %q, want %q", s.Score().Version(), tc.ScoreVersion)
	}
}

// TestIsOwnedBy_Matching 匹配 owner 应返回 true
func TestIsOwnedBy_Matching(t *testing.T) {
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
