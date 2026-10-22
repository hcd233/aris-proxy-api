package session_dedup

import (
	"testing"

	repository "github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

// sessionTargetCase 会话快照承接目标选择用例
type sessionTargetCase struct {
	name       string
	sessions   []sessionFixture
	messageIDs []uint
	// expectedID 期望承接的会话 ID；0 表示同组无承接者，应保留新行
	expectedID uint
}

// TestFindSessionTarget 覆盖续写、补发旧轮次、对话分叉与多快照选择
func TestFindSessionTarget(t *testing.T) {
	t.Parallel()

	testCases := []sessionTargetCase{
		{
			name:       "continuation reuses existing session",
			sessions:   []sessionFixture{{ID: 1, MessageIDs: []uint{1, 2}}},
			messageIDs: []uint{1, 2, 3},
			expectedID: 1,
		},
		{
			name: "longest prefix wins among snapshots",
			sessions: []sessionFixture{
				{ID: 1, MessageIDs: []uint{1, 2}},
				{ID: 2, MessageIDs: []uint{1, 2, 3}},
			},
			messageIDs: []uint{1, 2, 3, 4},
			expectedID: 2,
		},
		{
			name: "equal length tie prefers smallest id",
			sessions: []sessionFixture{
				{ID: 2, MessageIDs: []uint{1, 2}},
				{ID: 1, MessageIDs: []uint{1, 2}},
			},
			messageIDs: []uint{1, 2, 3},
			expectedID: 1,
		},
		{
			name:       "shorter replay has no continuation target",
			sessions:   []sessionFixture{{ID: 1, MessageIDs: []uint{1, 2, 3}}},
			messageIDs: []uint{1, 2},
			expectedID: 0,
		},
		{
			name: "fork branch does not shadow continuation",
			sessions: []sessionFixture{
				{ID: 1, MessageIDs: []uint{1, 2}},
				{ID: 2, MessageIDs: []uint{1, 2, 4}},
			},
			messageIDs: []uint{1, 2, 3},
			expectedID: 1,
		},
		{
			name:       "fork without prefix relation keeps new session",
			sessions:   []sessionFixture{{ID: 1, MessageIDs: []uint{1, 2, 3}}},
			messageIDs: []uint{1, 2, 4},
			expectedID: 0,
		},
		{
			name:       "empty group keeps new session",
			sessions:   []sessionFixture{},
			messageIDs: []uint{1, 2},
			expectedID: 0,
		},
		{
			name:       "empty message ids keeps new session",
			sessions:   []sessionFixture{{ID: 1, MessageIDs: []uint{1, 2}}},
			messageIDs: []uint{},
			expectedID: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			target := repository.FindSessionTarget(toDBSessions(tc.sessions), tc.messageIDs)
			gotID := uint(0)
			if target != nil {
				gotID = target.ID
			}
			if gotID != tc.expectedID {
				t.Fatalf("FindSessionTarget() id = %d, want %d", gotID, tc.expectedID)
			}
		})
	}
}
