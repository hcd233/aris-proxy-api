package session_dedup

import (
	"os"
	"sort"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	vo "github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// isSubArrayCase represents a test case for IsSubArray loaded from fixtures
type isSubArrayCase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Sub         []uint `json:"sub"`
	Arr         []uint `json:"arr"`
	Expected    bool   `json:"expected"`
}

// sessionFixture represents session data in fixture JSON
type sessionFixture struct {
	ID         uint   `json:"id"`
	MessageIDs []uint `json:"message_ids"`
	ToolIDs    []uint `json:"tool_ids"`
}

// findRedundantSessionsCase represents a test case for FindRedundantSessions
type findRedundantSessionsCase struct {
	Name                 string           `json:"name"`
	Description          string           `json:"description"`
	Sessions             []sessionFixture `json:"sessions"`
	ExpectedRedundantIDs []uint           `json:"expected_redundant_ids"`
}

// loadIsSubArrayCases loads IsSubArray test cases from fixtures
func loadIsSubArrayCases(t *testing.T) []isSubArrayCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/is_sub_array_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/is_sub_array_cases.json: %v", err)
	}
	var cases []isSubArrayCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/is_sub_array_cases.json: %v", err)
	}
	return cases
}

// loadFindRedundantSessionsCases loads FindRedundantSessions test cases from fixtures
func loadFindRedundantSessionsCases(t *testing.T) []findRedundantSessionsCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/find_redundant_sessions_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/find_redundant_sessions_cases.json: %v", err)
	}
	var cases []findRedundantSessionsCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/find_redundant_sessions_cases.json: %v", err)
	}
	return cases
}

// findIsSubArrayCase finds an IsSubArray test case by name
func findIsSubArrayCase(t *testing.T, cases []isSubArrayCase, name string) isSubArrayCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return isSubArrayCase{}
}

// findFindRedundantSessionsCase finds a FindRedundantSessions test case by name
func findFindRedundantSessionsCase(t *testing.T, cases []findRedundantSessionsCase, name string) findRedundantSessionsCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return findRedundantSessionsCase{}
}

// terminalToolCallCase represents a test case for FindTerminalToolCallSessions
type terminalToolCallCase struct {
	Name                 string                       `json:"name"`
	Description          string                       `json:"description"`
	Sessions             []sessionFixture             `json:"sessions"`
	Messages             []terminalToolCallMessageFix `json:"messages"`
	ExcludeIDs           []uint                       `json:"exclude_ids"`
	ExpectedRedundantIDs []uint                       `json:"expected_redundant_ids"`
	ExpectedMergeMapping map[uint][]uint              `json:"expected_merge_mapping"`
}

// terminalToolCallMessageFix represents a message fixture for terminal tool call tests
type terminalToolCallMessageFix struct {
	ID        uint                  `json:"id"`
	Role      string                `json:"role"`
	ToolCalls []terminalToolCallFix `json:"tool_calls"`
}

// terminalToolCallFix represents a tool call fixture
type terminalToolCallFix struct {
	ID       string `json:"id"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

// loadTerminalToolCallCases loads FindTerminalToolCallSessions test cases from fixtures
func loadTerminalToolCallCases(t *testing.T) []terminalToolCallCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/terminal_tool_call_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/terminal_tool_call_cases.json: %v", err)
	}
	var cases []terminalToolCallCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/terminal_tool_call_cases.json: %v", err)
	}
	return cases
}

// toDBMessages converts fixture messages to database model messages
func toDBMessages(fixtures []terminalToolCallMessageFix) []*dbmodel.Message {
	messages := make([]*dbmodel.Message, 0, len(fixtures))
	for _, f := range fixtures {
		toolCalls := make([]*vo.UnifiedToolCall, 0, len(f.ToolCalls))
		for _, tc := range f.ToolCalls {
			toolCalls = append(toolCalls, &vo.UnifiedToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
			})
		}
		m := &dbmodel.Message{
			Message: &vo.UnifiedMessage{
				Role:      enum.Role(f.Role),
				ToolCalls: toolCalls,
			},
		}
		m.ID = f.ID
		messages = append(messages, m)
	}
	return messages
}

// toDBSessions converts fixture sessions to database model sessions
func toDBSessions(fixtures []sessionFixture) []*dbmodel.Session {
	sessions := make([]*dbmodel.Session, 0, len(fixtures))
	for _, f := range fixtures {
		s := &dbmodel.Session{
			MessageIDs: f.MessageIDs,
			ToolIDs:    f.ToolIDs,
		}
		s.ID = f.ID
		sessions = append(sessions, s)
	}
	return sessions
}

// TestIsSubArray runs all IsSubArray fixture cases
func TestIsSubArray(t *testing.T) {
	allCases := loadIsSubArrayCases(t)

	caseNames := []string{
		"basic_subarray_match",
		"prefix_match",
		"suffix_match",
		"exact_match",
		"empty_sub",
		"sub_longer_than_arr",
		"non_contiguous_elements",
		"no_common_elements",
		"single_element_found",
		"single_element_not_found",
		"partial_overlap_not_subarray",
		"repeated_elements_match",
		"both_empty",
	}

	for _, caseName := range caseNames {
		tc := findIsSubArrayCase(t, allCases, caseName)

		t.Run(caseName, func(t *testing.T) {
			got := cron.IsSubArray(tc.Sub, tc.Arr)

			t.Logf("description: %s", tc.Description)
			t.Logf("sub=%v, arr=%v, got=%v, expected=%v", tc.Sub, tc.Arr, got, tc.Expected)

			if got != tc.Expected {
				t.Errorf("IsSubArray(%v, %v) = %v, want %v", tc.Sub, tc.Arr, got, tc.Expected)
			}
		})
	}
}

// TestFindRedundantSessionsWithMerge tests the tool_ids merging functionality
func TestFindRedundantSessionsWithMerge(t *testing.T) {
	allCases := loadFindRedundantSessionsCases(t)

	testCases := []struct {
		name                  string
		expectedMergedToolIDs map[uint][]uint // session ID -> expected merged tool IDs
	}{
		{
			name: "merge_tool_ids",
			expectedMergedToolIDs: map[uint][]uint{
				1: {1, 2, 3}, // Session 1 should have tool_ids [1, 2, 3] (union of [1,2] and [2,3])
			},
		},
		{
			name: "merge_multiple_tool_ids",
			expectedMergedToolIDs: map[uint][]uint{
				1: {1, 2, 3}, // Session 1 should have tool_ids [1, 2, 3] (union of [1], [2], and [3])
			},
		},
	}

	for _, tc := range testCases {
		fixtureCase := findFindRedundantSessionsCase(t, allCases, tc.name)

		t.Run(tc.name, func(t *testing.T) {
			sessions := toDBSessions(fixtureCase.Sessions)
			result := cron.FindRedundantSessionsWithMerge(sessions)

			t.Logf("description: %s", fixtureCase.Description)
			t.Logf("merge mapping: %v", result.MergeMapping)

			// Check that the merge mapping contains the expected tool IDs
			for sessionID, expectedToolIDs := range tc.expectedMergedToolIDs {
				toolIDSet, exists := result.MergeMapping[sessionID]
				if !exists {
					t.Errorf("Expected merge mapping for session %d, but not found", sessionID)
					continue
				}

				// Convert set to sorted slice
				actualToolIDs := make([]uint, 0, len(toolIDSet))
				for tid := range toolIDSet {
					actualToolIDs = append(actualToolIDs, tid)
				}
				sort.Slice(actualToolIDs, func(i, j int) bool { return actualToolIDs[i] < actualToolIDs[j] })

				if len(actualToolIDs) != len(expectedToolIDs) {
					t.Errorf("Session %d: expected %d tool IDs, got %d; got=%v, want=%v",
						sessionID, len(expectedToolIDs), len(actualToolIDs), actualToolIDs, expectedToolIDs)
					continue
				}

				for i := range actualToolIDs {
					if actualToolIDs[i] != expectedToolIDs[i] {
						t.Errorf("Session %d: tool ID mismatch at index %d: got %d, want %d; full got=%v, want=%v",
							sessionID, i, actualToolIDs[i], expectedToolIDs[i], actualToolIDs, expectedToolIDs)
					}
				}
			}
		})
	}
}

// TestFindRedundantSessions runs all FindRedundantSessions fixture cases
func TestFindRedundantSessions(t *testing.T) {
	allCases := loadFindRedundantSessionsCases(t)

	caseNames := []string{
		"basic_subarray_containment",
		"tail_subarray",
		"middle_subarray",
		"no_containment",
		"identical_sessions_keep_earlier",
		"chain_containment",
		"single_element_subarray",
		"non_contiguous_not_subarray",
		"multiple_subarrays_of_same_parent",
		"empty_message_ids_ignored",
		"single_session",
		"three_identical_sessions",
	}

	for _, caseName := range caseNames {
		tc := findFindRedundantSessionsCase(t, allCases, caseName)

		t.Run(caseName, func(t *testing.T) {
			sessions := toDBSessions(tc.Sessions)
			got := cron.FindRedundantSessions(sessions)

			t.Logf("description: %s", tc.Description)
			t.Logf("input sessions: %d, got redundant IDs: %v, expected: %v",
				len(tc.Sessions), got, tc.ExpectedRedundantIDs)

			// Sort both slices for comparison
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
			expected := make([]uint, len(tc.ExpectedRedundantIDs))
			copy(expected, tc.ExpectedRedundantIDs)
			sort.Slice(expected, func(i, j int) bool { return expected[i] < expected[j] })

			if len(got) != len(expected) {
				t.Fatalf("FindRedundantSessions() returned %d IDs, want %d; got=%v, want=%v",
					len(got), len(expected), got, expected)
			}

			for i := range got {
				if got[i] != expected[i] {
					t.Errorf("FindRedundantSessions() IDs mismatch at index %d: got %d, want %d; full got=%v, want=%v",
						i, got[i], expected[i], got, expected)
				}
			}
		})
	}
}

// TestFindTerminalToolCallSessions tests the FindTerminalToolCallSessions function
func TestFindTerminalToolCallSessions(t *testing.T) {
	allCases := loadTerminalToolCallCases(t)

	caseNames := []string{
		"terminal_tool_call_basic",
		"terminal_tool_call_no_parent",
		"terminal_tool_call_excluded_session",
		"terminal_tool_call_multiple_parents_picks_longest",
		"terminal_tool_call_no_tool_ids",
		"terminal_tool_call_last_msg_not_assistant",
		"terminal_tool_call_empty_sessions",
	}

	for _, caseName := range caseNames {
		var tc terminalToolCallCase
		for _, c := range allCases {
			if c.Name == caseName {
				tc = c
				break
			}
		}

		t.Run(caseName, func(t *testing.T) {
			sessions := toDBSessions(tc.Sessions)
			messages := toDBMessages(tc.Messages)
			result := cron.FindTerminalToolCallSessions(sessions, messages, tc.ExcludeIDs)

			t.Logf("description: %s", tc.Description)
			t.Logf("redundant IDs: %v, expected: %v", result.RedundantIDs, tc.ExpectedRedundantIDs)
			t.Logf("merge mapping: %v", result.MergeMapping)

			// Check RedundantIDs
			got := result.RedundantIDs
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
			expected := make([]uint, len(tc.ExpectedRedundantIDs))
			copy(expected, tc.ExpectedRedundantIDs)
			sort.Slice(expected, func(i, j int) bool { return expected[i] < expected[j] })

			if len(got) != len(expected) {
				t.Fatalf("FindTerminalToolCallSessions() returned %d IDs, want %d; got=%v, want=%v",
					len(got), len(expected), got, expected)
			}
			for i := range got {
				if got[i] != expected[i] {
					t.Errorf("FindTerminalToolCallSessions() IDs mismatch at index %d: got %d, want %d",
						i, got[i], expected[i])
				}
			}

			// Check MergeMapping
			if len(tc.ExpectedMergeMapping) == 0 && len(result.MergeMapping) > 0 {
				t.Errorf("Expected empty merge mapping, got %v", result.MergeMapping)
			}
			for sessionID, expectedToolIDs := range tc.ExpectedMergeMapping {
				toolIDSet, exists := result.MergeMapping[sessionID]
				if !exists {
					t.Errorf("Expected merge mapping for session %d, but not found", sessionID)
					continue
				}

				actualToolIDs := make([]uint, 0, len(toolIDSet))
				for tid := range toolIDSet {
					actualToolIDs = append(actualToolIDs, tid)
				}
				sort.Slice(actualToolIDs, func(i, j int) bool { return actualToolIDs[i] < actualToolIDs[j] })

				if len(actualToolIDs) != len(expectedToolIDs) {
					t.Errorf("Session %d: expected %d tool IDs, got %d; got=%v, want=%v",
						sessionID, len(expectedToolIDs), len(actualToolIDs), actualToolIDs, expectedToolIDs)
					continue
				}
				for i := range actualToolIDs {
					if actualToolIDs[i] != expectedToolIDs[i] {
						t.Errorf("Session %d: tool ID mismatch at index %d: got %d, want %d; full got=%v, want=%v",
							sessionID, i, actualToolIDs[i], expectedToolIDs[i], actualToolIDs, expectedToolIDs)
					}
				}
			}
		})
	}
}

// TestFindParentSessionID tests the findParentSessionID function (exported via FindTerminalToolCallSessions)
func TestFindParentSessionID(t *testing.T) {
	sessions := toDBSessions([]sessionFixture{
		{ID: 1, MessageIDs: []uint{1, 2, 3, 4, 5, 6}, ToolIDs: []uint{100}},
		{ID: 2, MessageIDs: []uint{1, 2, 3, 4, 5}, ToolIDs: []uint{200}},
		{ID: 3, MessageIDs: []uint{1, 2, 3}, ToolIDs: []uint{30}},
		{ID: 4, MessageIDs: []uint{10, 20}, ToolIDs: []uint{}},
	})

	// Session 3 [1,2,3] is subarray of both session 1 [1,2,3,4,5,6] and session 2 [1,2,3,4,5]
	// findParentSessionID should pick session 1 (longest MessageIDs)
	messages := toDBMessages([]terminalToolCallMessageFix{
		{ID: 3, Role: "assistant", ToolCalls: []terminalToolCallFix{
			{ID: "tc1", Function: struct {
				Name string `json:"name"`
			}{Name: "search"}},
		}},
	})

	result := cron.FindTerminalToolCallSessions(sessions, messages, nil)

	found := false
	for _, id := range result.RedundantIDs {
		if id == 3 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected session 3 to be marked redundant, got %v", result.RedundantIDs)
	}

	// Should merge to session 1 (longest), not session 2
	if _, ok := result.MergeMapping[1]; !ok {
		t.Errorf("Expected merge mapping for session 1 (longest parent), got mapping: %v", result.MergeMapping)
	}
	if _, ok := result.MergeMapping[2]; ok {
		t.Errorf("Should NOT merge to session 2 (shorter parent), got mapping: %v", result.MergeMapping)
	}

	// Verify tool IDs are merged correctly (set is unordered, sorted comparison)
	mergedSet := result.MergeMapping[1]
	expectedToolIDs := []uint{30, 100} // sorted order
	actualToolIDs := make([]uint, 0, len(mergedSet))
	for tid := range mergedSet {
		actualToolIDs = append(actualToolIDs, tid)
	}
	sort.Slice(actualToolIDs, func(i, j int) bool { return actualToolIDs[i] < actualToolIDs[j] })
	if len(actualToolIDs) != len(expectedToolIDs) {
		t.Errorf("Expected tool IDs %v, got %v", expectedToolIDs, actualToolIDs)
	}
	for i := range actualToolIDs {
		if actualToolIDs[i] != expectedToolIDs[i] {
			t.Errorf("Tool ID mismatch at index %d: got %d, want %d", i, actualToolIDs[i], expectedToolIDs[i])
		}
	}
}
