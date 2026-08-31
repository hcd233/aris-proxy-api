// Package session_dedup SessionTerminalCleanupCron 纯函数用例
//
//	@author centonhuang
//	@update 2026-08-29 10:00:00
package session_dedup

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/cron"
	dao "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
)

func TestPickTerminalStuckSessions(t *testing.T) {
	t.Parallel()

	sessions := []dao.SessionTerminalScanView{
		{ID: 11, MessageIDs: []uint{1, 2, 3}}, // 末条 3 命中 terminal -> 删除
		{ID: 12, MessageIDs: []uint{4, 5}},    // 末条 5 未命中 -> 保留
		{MessageIDs: []uint{}},                // 空 MessageIDs -> 跳过
	}
	got := cron.PickTerminalStuckSessions(sessions, []uint{3, 99})
	if !slices.Equal(got, []uint{11}) {
		t.Errorf("PickTerminalStuckSessions() = %v, want [11]", got)
	}

	if got := cron.PickTerminalStuckSessions(sessions, nil); len(got) != 0 {
		t.Errorf("PickTerminalStuckSessions() with nil terminal = %v, want empty", got)
	}
}
