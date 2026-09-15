package transport

import (
	"context"
	"io"
	"sync"
)

// drainCancelBody 把 CancelOnDrain 派生 ctx 的 cancel 绑定到上游 body 的 Close：
// body Close 即上游请求彻底结束，此时结束派生 ctx 与其守护 goroutine。
//
// 背景（2026-09-02 生产 goroutine 泄漏修复）：CancelOnDrain 的守护 goroutine
// 监听 drain 广播与派生 ctx Done，但 fiber/fasthttp 的请求 ctx 不随请求结束
// 而 Done（RequestCtx.Done 返回 nil channel），派生 ctx 无父级取消来源；
// 若无人显式 cancel，每个上游请求泄漏一个 goroutine，随 LLM 代理流量累积。
// bulkhead 槽位与熔断上报已由 leaseBoundBody 绑定同一 Close 时机，drain
// 派生 ctx 的生命周期与之对齐：上游 body 关闭 = 上游连接生命周期结束。
//
// Close 幂等（sync.OnceFunc 记忆首次结果）：上游 body 既会被传输层读循环
// defer Close，也会被 adapter 在流结束后兜底 Close，两条路径命中同一 wrapper，
// 必须保证底层 body 与 cancel 只生效一次。
//
// 顺序：先 cancel 再 Close。cancel 让上游请求立刻结束，随后 Close 清理连接；
// 反过来时，若 body 实现落在 net/http transfer.go 的 drain 分支（未读到 EOF 时 Close
// 会同步读完剩余响应），取消就要等整条上游流读完才生效。
// 实测（Go 1.25，darwin）：HTTP/1.1 客户端 body 带 earlyCloseFn，未读到 EOF 时 Close
// 直接 abort 连接而非 drain（微秒级返回），故这里是防御性顺序，不是已观测缺陷的修复。
//
//	@author centonhuang
//	@update 2026-09-15 10:00:00
type drainCancelBody struct {
	io.ReadCloser
	closeOnce func()
	err       error
}

// newDrainCancelBody 包装 body 并把 cancel 绑定到其 Close；body 为 nil 时立即 cancel 返回 nil。
func newDrainCancelBody(body io.ReadCloser, cancel context.CancelFunc) io.ReadCloser {
	if body == nil {
		cancel()
		return nil
	}
	b := &drainCancelBody{ReadCloser: body}
	b.closeOnce = sync.OnceFunc(func() {
		b.err = body.Close()
		cancel()
	})
	return b
}

// Close 关闭上游 body 并 cancel 派生 ctx；重复调用返回首次结果，可重入安全。
func (b *drainCancelBody) Close() error {
	b.closeOnce()
	return b.err
}
