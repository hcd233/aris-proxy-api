// Package session_keyword_filter（追加）验证两段式 keyword 查询的纯函数。
package session_keyword_filter

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/util"
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

// TestEscapeLikePattern 断言 LIKE/ILIKE 通配符按字面转义（\ % _）。
//
//	@author centonhuang
//	@update 2026-10-16 16:00:00
func TestEscapeLikePattern(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain stays plain", "shadcn", "shadcn"},
		{"percent escaped", "100%", `100\%`},
		{"underscore escaped", "a_b", `a\_b`},
		{"backslash escaped", `a\b`, `a\\b`},
		{"all together", `%_\`, `\%\_\\`},
		{"no double escaping", `\%`, `\\\%`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := util.EscapeLikePattern(tc.in); got != tc.want {
				t.Errorf("EscapeLikePattern(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestKeywordLikePattern 断言 LIKE 模式与缓存 key 同源：归一化 + 通配符转义 + 两侧 %。
//
//	@author centonhuang
//	@update 2026-10-17 22:00:00
func TestKeywordLikePattern(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"wraps and normalizes", "  Shadcn ", "%shadcn%"},
		{"percent escaped", "100%", `%100\%%`},
		{"underscore escaped", "a_b", `%a\_b%`},
		{"backslash escaped", `a\b`, `%a\\b%`},
		{"empty stays wildcard", "", "%%"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := repository.KeywordLikePattern(tc.in); got != tc.want {
				t.Errorf("KeywordLikePattern(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	t.Run("normalized variants share pattern and key", func(t *testing.T) {
		t.Parallel()
		if repository.KeywordLikePattern("Shadcn") != repository.KeywordLikePattern("  shadcn ") {
			t.Error("normalized variants must share like pattern")
		}
	})
}

// TestKeywordCacheKeyFragment 断言缓存 key 片段：摘要稳定、归一化后大小写
// 与空白变体命中同一 key（对齐 ILIKE 大小写不敏感语义）、用户输入不原样进 key。
//
//	@author centonhuang
//	@update 2026-10-16 16:00:00
func TestKeywordCacheKeyFragment(t *testing.T) {
	t.Parallel()
	base := repository.KeywordCacheKeyFragment("Shadcn")
	if got := repository.KeywordCacheKeyFragment("  shadcn "); got != base {
		t.Errorf("normalized variants must share key: %q != %q", got, base)
	}
	if got := repository.KeywordCacheKeyFragment("Shadcn"); got != base {
		t.Errorf("fragment must be stable, got %q want %q", got, base)
	}
	if got := repository.KeywordCacheKeyFragment("Task:"); got == "Task:" || got == "task:" {
		t.Errorf("raw keyword must not appear in fragment, got %q", got)
	}
	if len(base) != 64 {
		t.Errorf("fragment must be sha256 hex (64 chars), got %d", len(base))
	}
}
