// Package guard_clean_cache 路由扫描防护的进程内负缓存回归测试。
//
// 背景：GuardMiddleware 原先每请求一次 Redis EXISTS 查封禁状态，是轻查询接口的固定成本。
// 新增的 BanCleanCache 缓存「该 IP 未被封禁」的结论（1s），把 Redis 调用摊薄到每 IP
// 每秒 1 次。本测试钉死四条语义：未记录不算干净、记录后命中、主动失效、过期后失效，
// 以及超限逐条淘汰保持内存有界（docs/loadtest/QPS-OPTIMIZATION.md §3.3）。
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

	const limit = 2
	cache := middleware.NewBanCleanCache(time.Minute, limit)
	ips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5"}
	for _, ip := range ips {
		cache.MarkClean(ip)
	}

	// 最新结论必须保留：否则每次溢出都会丢掉当前 IP 的干净结论，等于白查一次 Redis
	if !cache.IsClean(ips[len(ips)-1]) {
		t.Fatal("latest entry must survive overflow")
	}

	// 超限逐条淘汰而非整体清空：存活条数应恰好等于上限（最新一条 + limit-1 条旧条目）
	others := 0
	for _, ip := range ips[:len(ips)-1] {
		if cache.IsClean(ip) {
			others++
		}
	}
	if others != limit-1 {
		t.Fatalf("surviving old entries = %d, want %d (cache must stay bounded at limit)", others, limit-1)
	}
}
