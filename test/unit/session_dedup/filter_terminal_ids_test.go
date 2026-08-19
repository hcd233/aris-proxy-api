package session_dedup

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
)

// TestFilterTerminalToolCallIDsEmptyInput 验证空输入直接短路，不触发数据库查询。
//
// 传入 nil db：若实现没有短路，会因 nil 指针解引用 panic；短路则安全返回。
// 这同时保证了不会对空 ID 列表发出 WHERE id IN () 查询。
//
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func TestFilterTerminalToolCallIDsEmptyInput(t *testing.T) {
	t.Parallel()

	for _, ids := range [][]uint{nil, {}} {
		got, err := dao.GetMessageDAO().FilterTerminalToolCallIDs(nil, ids)
		if err != nil {
			t.Fatalf("FilterTerminalToolCallIDs(nil, %v) error = %v, want nil", ids, err)
		}
		if len(got) != 0 {
			t.Errorf("FilterTerminalToolCallIDs(nil, %v) = %v, want empty", ids, got)
		}
	}
}
