// Package session_keyword_filter（追加）验证两段式 keyword 查询的纯函数。
package session_keyword_filter

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

// TestFormatBigIntArray 断言 id 数组序列化为 PG bigint[] 字面量。
// 纯数字拼接，无注入面（禁止用 %s 直接拼用户输入）。
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestFormatBigIntArray(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ids  []uint
		want string
	}{
		{"empty", nil, "{}"},
		{"single", []uint{42}, "{42}"},
		{"multi", []uint{210213, 1, 168194}, "{210213,1,168194}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := repository.FormatBigIntArray(tc.ids); got != tc.want {
				t.Errorf("FormatBigIntArray(%v) = %q, want %q", tc.ids, got, tc.want)
			}
		})
	}
}

// TestNormalizeKeyword 断言 keyword 归一化（trim + 小写）用于缓存 key。
//
//	@author centonhuang
//	@update 2026-10-15 16:00:00
func TestNormalizeKeyword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"trims and lowers", "  Shadcn ", "shadcn"},
		{"keeps symbol case", "Task:", "task:"},
		{"empty stays empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := repository.NormalizeKeyword(tc.in); got != tc.want {
				t.Errorf("NormalizeKeyword(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
