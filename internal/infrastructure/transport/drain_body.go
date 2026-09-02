package transport

import (
	"context"
	"io"
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
//	@author centonhuang
//	@update 2026-09-02 10:00:00
type drainCancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

// newDrainCancelBody 包装 body 并绑定 cancel 到其 Close；body 为 nil 时立即 cancel 返回 nil。
func newDrainCancelBody(body io.ReadCloser, cancel context.CancelFunc) io.ReadCloser {
	if body == nil {
		cancel()
		return nil
	}
	return &drainCancelBody{ReadCloser: body, cancel: cancel}
}

func (b *drainCancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
