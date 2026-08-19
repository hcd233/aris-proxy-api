package session_dedup

import (
	"os"
	"slices"
	"sort"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/cron"
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
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Sessions    []sessionFixture `json:"sessions"`
	// TerminalMsgIDs 已由 SQL 判定为 assistant+tool_calls 的 message ID，缺省为空
	TerminalMsgIDs       []uint `json:"terminal_msg_ids"`
	ExpectedRedundantIDs []uint `json:"expected_redundant_ids"`
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
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Sessions    []sessionFixture `json:"sessions"`
	// TerminalMsgIDs 已由 SQL 判定为 assistant+tool_calls 的 message ID
	TerminalMsgIDs       []uint          `json:"terminal_msg_ids"`
	ExcludeIDs           []uint          `json:"exclude_ids"`
	ExpectedRedundantIDs []uint          `json:"expected_redundant_ids"`
	ExpectedMergeMapping map[uint][]uint `json:"expected_merge_mapping"`
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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
				// 只有前缀 session 2 并入；session 3 [3,4,5] 是尾部子数组，
				// 首个 message id 不同即属另一对话根，保持独立
				1: {1, 2},
			},
		},
		{
			name: "two_prefix_members_same_keeper",
			expectedMergedToolIDs: map[uint][]uint{
				// 两个前缀成员并入同一 keeper 必须累积而非覆盖
				1: {20, 30, 100},
			},
		},
		{
			name: "forked_conversation",
			expectedMergedToolIDs: map[uint][]uint{
				// [1,2] 并入首个匹配的 keeper（最长、ID 最小）= session 1；
				// 分叉分支 session 2 不接收任何合并
				1: {10, 30},
			},
		},
	}

	for _, tc := range testCases {
		fixtureCase := findFindRedundantSessionsCase(t, allCases, tc.name)

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sessions := toDBSessions(fixtureCase.Sessions)
			result := cron.FindRedundantSessions(sessions, nil)

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
	t.Parallel()
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
		"cross_group_not_redundant",
		"forked_conversation",
		"two_prefix_members_same_keeper",
		"group_keeper_protected_without_tools",
		"forked_keeper_not_protected",
		"singleton_terminal_tool_call",
	}

	for _, caseName := range caseNames {
		tc := findFindRedundantSessionsCase(t, allCases, caseName)

		t.Run(caseName, func(t *testing.T) {
			t.Parallel()
			sessions := toDBSessions(tc.Sessions)
			got := cron.FindRedundantSessions(sessions, tc.TerminalMsgIDs).RedundantIDs

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
	t.Parallel()
	allCases := loadTerminalToolCallCases(t)

	caseNames := []string{
		"terminal_tool_call_basic",
		"terminal_tool_call_no_parent",
		"terminal_tool_call_excluded_session",
		"terminal_tool_call_multiple_parents_picks_longest",
		"terminal_tool_call_no_tool_ids",
		"terminal_tool_call_last_msg_not_assistant",
		"terminal_tool_call_empty_sessions",
		"terminal_tool_call_merge_target_excluded",
		"terminal_tool_call_two_children_same_parent",
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
			t.Parallel()
			sessions := toDBSessions(tc.Sessions)
			result := cron.FindTerminalToolCallSessions(sessions, tc.TerminalMsgIDs, tc.ExcludeIDs)

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
	t.Parallel()
	sessions := toDBSessions([]sessionFixture{
		{ID: 1, MessageIDs: []uint{1, 2, 3, 4, 5, 6}, ToolIDs: []uint{100}},
		{ID: 2, MessageIDs: []uint{1, 2, 3, 4, 5}, ToolIDs: []uint{200}},
		{ID: 3, MessageIDs: []uint{1, 2, 3}, ToolIDs: []uint{30}},
		{ID: 4, MessageIDs: []uint{10, 20}, ToolIDs: []uint{}},
	})

	// Session 3 [1,2,3] is subarray of both session 1 [1,2,3,4,5,6] and session 2 [1,2,3,4,5]
	// findParentSessionID should pick session 1 (longest MessageIDs)
	result := cron.FindTerminalToolCallSessions(sessions, []uint{3}, nil)

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

// TestMergeTargetProtectedFromTerminalRule 回归测试：吸收过冗余成员的 merge target
// 不能被终端 tool_call 规则删除，否则并入它的 ToolIDs 会随之丢失。
//
// 场景来自生产 trace 77a87daf：
//   - session 3 ([10,20,30]) 是 session 2 ([10,20,30,40,50]) 的前缀，被并入 session 2
//   - session 2 的末条消息 50 是 assistant+tool_calls，会命中终端规则
//   - 旧实现用 MergeMapping 的 key 作保护代理，需要 deduplicate 手工把 merge target
//     拼进 excludeIDs，漏拼即导致 session 2 既被更新又被删除（#85）
//   - 现在保护由单一入口内部的 absorbed 集合表达，调用方无需关心，
//     且不再受「tool_ids 是否为空」影响
//
// session 1 与 session 2 在 [10,20] 之后分别走 70.. 与 30..，构成对话分叉，
// 因此 session 2 既是分叉分支又是 merge target。
//
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func TestMergeTargetProtectedFromTerminalRule(t *testing.T) {
	t.Parallel()

	sessions := toDBSessions([]sessionFixture{
		{ID: 1, MessageIDs: []uint{10, 20, 70, 80, 90, 100}, ToolIDs: []uint{100}},
		{ID: 2, MessageIDs: []uint{10, 20, 30, 40, 50}, ToolIDs: []uint{200}},
		{ID: 3, MessageIDs: []uint{10, 20, 30}, ToolIDs: []uint{30}},
	})

	// message 50 是 session 2 的末条消息，已由 SQL 判定为 assistant+tool_calls
	result := cron.FindRedundantSessions(sessions, []uint{50})

	t.Logf("redundantIDs=%v, mergeMapping=%v", result.RedundantIDs, result.MergeMapping)

	if slices.Contains(result.RedundantIDs, 2) {
		t.Errorf("session 2 is a merge target and must not be deleted by the terminal rule, got redundant=%v", result.RedundantIDs)
	}
	if !slices.Contains(result.RedundantIDs, 3) {
		t.Errorf("session 3 ([10,20,30] is a prefix of session 2) should be redundant, got %v", result.RedundantIDs)
	}
	if _, ok := result.MergeMapping[2]; !ok {
		t.Errorf("session 3's ToolIDs should merge into session 2, got mapping=%v", result.MergeMapping)
	}
}
