// Package guard_clean_cache 路由扫描防护的进程内负缓存回归测试。
//
// 背景：GuardMiddleware 原先每请求一次 Redis EXISTS 查封禁状态，是轻查询接口的固定成本。
// 新增的 BanCleanCache 缓存「该 IP 未被封禁」的结论（1s），把 Redis 调用摊薄到每 IP
// 每秒 1 次。本测试钉死四条语义：未记录不算干净、记录后命中、主动失效、过期后失效，
// 以及超限清空保持内存有界（docs/loadtest/QPS-OPTIMIZATION.md §3.3）。
package guard_clean_cache

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func TestBanCleanCacheUnknownIP(t *testing.T) {
	t.Parallel()

	cache := middleware.NewBanCleanCache(time.Minute, 8)
	if cache.IsClean("10.0.0.1") {
		t.Fatal("unknown ip must not be reported as clean")
	}
}

func TestBanCleanCacheMarkAndInvalidate(t *testing.T) {
	t.Parallel()

	cache := middleware.NewBanCleanCache(time.Minute, 8)
	cache.MarkClean("10.0.0.1")
	if !cache.IsClean("10.0.0.1") {
		t.Fatal("marked ip should be clean")
	}

	cache.Invalidate("10.0.0.1")
	if cache.IsClean("10.0.0.1") {
		t.Fatal("invalidated ip must not be clean")
	}
}

func TestBanCleanCacheExpires(t *testing.T) {
	t.Parallel()

	// TTL 取 1ns：两次 time.Now() 之间必然已过期，避免用 sleep 等待真实时间。
	cache := middleware.NewBanCleanCache(time.Nanosecond, 8)
	cache.MarkClean("10.0.0.1")
	if cache.IsClean("10.0.0.1") {
		t.Fatal("expired conclusion must not be clean")
	}
}

func TestBanCleanCacheOverflowKeepsBounded(t *testing.T) {
	t.Parallel()

	cache := middleware.NewBanCleanCache(time.Minute, 2)
	cache.MarkClean("10.0.0.1")
	cache.MarkClean("10.0.0.2")
	// 第 3 条触发整体清空：前两条结论被丢弃，只保留最新一条。
	cache.MarkClean("10.0.0.3")

	if cache.IsClean("10.0.0.1") {
		t.Fatal("overflow should clear older entries")
	}
	if !cache.IsClean("10.0.0.3") {
		t.Fatal("latest entry should survive overflow")
	}
}
