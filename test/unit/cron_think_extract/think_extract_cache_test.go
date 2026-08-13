// Package cron_think_extract 白盒测试 ThinkExtractCron.extract 的缓存失效行为。
//
// 背景（Major，2026-08-12 review 发现）：
//   - extract 更新消息内容（剥离 <think> 标签）后不失效 session 详情中的
//     message:{id} 缓存，用户在 TTL（60min）内看到的是带 <think> 标签的旧内容。
//   - 修复：UpdateMessageContent 成功后删除 message:{id} 缓存 key。
package cron_think_extract

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation"
)

// fakeThinkRepo 可配置的 ThinkExtractRepository 假实现。
type fakeThinkRepo struct {
	candidates []*conversation.ThinkExtractMessage
	updated    map[uint]bool
}

func (f *fakeThinkRepo) FindThinkExtractCandidates(_ context.Context, _ uint, _, _ time.Time, _ int) ([]*conversation.ThinkExtractMessage, error) {
	return f.candidates, nil
}

func (f *fakeThinkRepo) UpdateMessageContent(_ context.Context, id uint, _ *vo.UnifiedMessage) error {
	f.updated[id] = true
	return nil
}

func newMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	return mr
}

func newRealClient(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// waitUntil 以 ticker 轮询等待条件成立（避免固定 time.Sleep）。
func waitUntil(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting: %s", msg)
		}
		<-ticker.C
	}
}

// TestThinkExtract_InvalidatesMessageCache
// 消息被成功更新（<think> 提取完成）后，message:{id} 缓存必须被删除。
func TestThinkExtract_InvalidatesMessageCache(t *testing.T) {
	t.Parallel()
	mr := newMiniredis(t)
	rdb := newRealClient(t, mr)

	const msgID = uint(42)
	// 预置 message:{id} 缓存（模拟用户已浏览过会话详情）
	cacheKey := fmt.Sprintf(constant.MessageKeyTemplate, msgID)
	mr.Set(cacheKey, `{"id":42,"content":{"text":"旧内容<think>推理</think>"}}`)

	repo := &fakeThinkRepo{
		candidates: []*conversation.ThinkExtractMessage{{
			ID: msgID,
			Message: &vo.UnifiedMessage{
				Role:    enum.RoleAssistant,
				Content: &vo.UnifiedContent{Text: "旧内容<think>推理</think>"},
			},
		}},
		updated: map[uint]bool{},
	}

	// 白盒调用 extract（不经 cron 调度与分布式锁）
	c := cron.NewThinkExtractCron(repo, rdb)
	if err := c.Start("*/5 * * * *"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer c.Stop()

	if !c.Trigger() {
		t.Fatal("Trigger returned false")
	}

	// 等待异步 goroutine 完成（更新 + 缓存删除）
	waitUntil(t, 3*time.Second, func() bool { return repo.updated[msgID] }, "message content updated")
	waitUntil(t, 3*time.Second, func() bool { return !mr.Exists(cacheKey) }, "message cache deleted")
}

// TestThinkExtract_UpdateErrorKeepsCache
// 更新失败时不得删除缓存（数据未变，缓存仍有效）。
func TestThinkExtract_UpdateErrorKeepsCache(t *testing.T) {
	t.Parallel()
	mr := newMiniredis(t)
	rdb := newRealClient(t, mr)

	const msgID = uint(7)
	cacheKey := fmt.Sprintf(constant.MessageKeyTemplate, msgID)
	mr.Set(cacheKey, `{"id":7}`)

	// 模拟 UpdateMessageContent 失败：总是失败的 repo 变体
	failRepo := &failThinkRepo{candidates: []*conversation.ThinkExtractMessage{{
		ID: msgID,
		Message: &vo.UnifiedMessage{
			Role:    enum.RoleAssistant,
			Content: &vo.UnifiedContent{Text: "x<think>y</think>"},
		},
	}}}

	c := cron.NewThinkExtractCron(failRepo, rdb)
	if err := c.Start("*/5 * * * *"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer c.Stop()

	if !c.Trigger() {
		t.Fatal("Trigger should acquire lock and start goroutine")
	}

	// 等待异步 goroutine 完成（更新失败路径），缓存应保留
	waitUntil(t, 3*time.Second, func() bool { return failRepo.called }, "extract finished")
	if !mr.Exists(cacheKey) {
		t.Fatal("expected cache key to remain when update fails")
	}
}

type failThinkRepo struct {
	candidates []*conversation.ThinkExtractMessage
	called     bool
}

func (f *failThinkRepo) FindThinkExtractCandidates(_ context.Context, _ uint, _, _ time.Time, _ int) ([]*conversation.ThinkExtractMessage, error) {
	return f.candidates, nil
}

func (f *failThinkRepo) UpdateMessageContent(_ context.Context, _ uint, _ *vo.UnifiedMessage) error {
	f.called = true
	return ierr.New(ierr.ErrInternal, "boom")
}
