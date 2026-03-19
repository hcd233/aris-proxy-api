package session_dto

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	model "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// sessionDTOCase represents a test case loaded from fixtures/cases.json
type sessionDTOCase struct {
	Name            string              `json:"name"`
	Description     string              `json:"description"`
	InputTime       string              `json:"input_time,omitempty"`
	ExpectedTimeStr string              `json:"expected_time_str,omitempty"`
	SessionSummary  *dto.SessionSummary `json:"session_summary,omitempty"`
	SessionDetail   *dto.SessionDetail  `json:"session_detail,omitempty"`
	ListSessionsRsp *listSessionsJSON   `json:"list_sessions_rsp,omitempty"`
	GetSessionRsp   *getSessionJSON     `json:"get_session_rsp,omitempty"`
}

// listSessionsJSON mirrors the JSON structure for ListSessionsRsp fixture
type listSessionsJSON struct {
	Sessions []*dto.SessionSummary `json:"sessions"`
}

// getSessionJSON mirrors the JSON structure for GetSessionRsp fixture
type getSessionJSON struct {
	Error *model.Error `json:"error,omitempty"`
}

// loadCases loads test cases from fixtures/cases.json
func loadCases(t *testing.T) []sessionDTOCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []sessionDTOCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

// findCase finds a test case by name, fatals if not found
func findCase(t *testing.T, cases []sessionDTOCase, name string) sessionDTOCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return sessionDTOCase{}
}

func TestSessionSummary_JSONSerialization(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "session_summary_roundtrip")

	data, err := sonic.Marshal(tc.SessionSummary)
	if err != nil {
		t.Fatalf("failed to marshal SessionSummary: %v", err)
	}

	var got dto.SessionSummary
	if err := sonic.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal SessionSummary: %v", err)
	}

	if got.ID != tc.SessionSummary.ID {
		t.Errorf("ID = %d, want %d", got.ID, tc.SessionSummary.ID)
	}
	if got.CreatedAt != tc.SessionSummary.CreatedAt {
		t.Errorf("CreatedAt = %q, want %q", got.CreatedAt, tc.SessionSummary.CreatedAt)
	}
	if got.UpdatedAt != tc.SessionSummary.UpdatedAt {
		t.Errorf("UpdatedAt = %q, want %q", got.UpdatedAt, tc.SessionSummary.UpdatedAt)
	}
	if len(got.MessageIDs) != len(tc.SessionSummary.MessageIDs) {
		t.Errorf("MessageIDs length = %d, want %d", len(got.MessageIDs), len(tc.SessionSummary.MessageIDs))
	}
	if len(got.ToolIDs) != len(tc.SessionSummary.ToolIDs) {
		t.Errorf("ToolIDs length = %d, want %d", len(got.ToolIDs), len(tc.SessionSummary.ToolIDs))
	}
}

func TestSessionDetail_JSONSerialization(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "session_detail_roundtrip")

	data, err := sonic.Marshal(tc.SessionDetail)
	if err != nil {
		t.Fatalf("failed to marshal SessionDetail: %v", err)
	}

	var got dto.SessionDetail
	if err := sonic.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal SessionDetail: %v", err)
	}

	if got.ID != tc.SessionDetail.ID {
		t.Errorf("ID = %d, want %d", got.ID, tc.SessionDetail.ID)
	}
	if got.APIKeyName != tc.SessionDetail.APIKeyName {
		t.Errorf("APIKeyName = %q, want %q", got.APIKeyName, tc.SessionDetail.APIKeyName)
	}
	if len(got.Messages) != len(tc.SessionDetail.Messages) {
		t.Fatalf("Messages length = %d, want %d", len(got.Messages), len(tc.SessionDetail.Messages))
	}
	if got.Messages[0].Model != tc.SessionDetail.Messages[0].Model {
		t.Errorf("Messages[0].Model = %q, want %q", got.Messages[0].Model, tc.SessionDetail.Messages[0].Model)
	}
	if len(got.Tools) != len(tc.SessionDetail.Tools) {
		t.Fatalf("Tools length = %d, want %d", len(got.Tools), len(tc.SessionDetail.Tools))
	}
	if got.Tools[0].Tool.Name != tc.SessionDetail.Tools[0].Tool.Name {
		t.Errorf("Tools[0].Tool.Name = %q, want %q", got.Tools[0].Tool.Name, tc.SessionDetail.Tools[0].Tool.Name)
	}
}

func TestListSessionsRsp_EmptySessions(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "list_sessions_empty")

	rsp := &dto.ListSessionsRsp{
		Sessions: tc.ListSessionsRsp.Sessions,
	}

	data, err := sonic.Marshal(rsp)
	if err != nil {
		t.Fatalf("failed to marshal ListSessionsRsp: %v", err)
	}

	var got dto.ListSessionsRsp
	if err := sonic.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal ListSessionsRsp: %v", err)
	}

	if len(got.Sessions) != 0 {
		t.Errorf("Sessions length = %d, want 0", len(got.Sessions))
	}
}

func TestGetSessionRsp_WithError(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "get_session_with_error")

	rsp := &dto.GetSessionRsp{}
	rsp.Error = tc.GetSessionRsp.Error

	data, err := sonic.Marshal(rsp)
	if err != nil {
		t.Fatalf("failed to marshal GetSessionRsp: %v", err)
	}

	var got dto.GetSessionRsp
	if err := sonic.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal GetSessionRsp: %v", err)
	}

	if got.Error == nil {
		t.Fatalf("Error should not be nil")
	}
	if got.Error.Code != tc.GetSessionRsp.Error.Code {
		t.Errorf("Error.Code = %d, want %d", got.Error.Code, tc.GetSessionRsp.Error.Code)
	}
	if got.Session != nil {
		t.Errorf("Session should be nil when error is present")
	}
}
