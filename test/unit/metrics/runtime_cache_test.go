// Package metrics_test 验证 runtime metrics 首屏（since=0）结果的短 TTL 缓存。
//
// 背景（2026-10-23 压测，BENCHMARK §3.6）：metrics/runtime 的 since=0 请求要读回
// range 窗口内全部快照（1h = 720 条/实例）并逐条 JSON 解码，实测 13.9ms CPU/req、
// 49 req/s；而前端增量轮询（since>0）只读尾部 2 个桶，本来就很便宜。
// 因此只缓存 since=0 的首屏结果（TTL = 采集间隔 5s），增量请求直接回源。
package metrics_test

import (
	"context"
	"testing"
	"time"

	metricsquery "github.com/hcd233/aris-proxy-api/internal/application/metrics/query"
)

type countingSnapshotReader struct {
	listCalls int
	readCalls int
}

func (r *countingSnapshotReader) ListInstances(_ context.Context, _ int64) ([]string, error) {
	r.listCalls++
	return []string{"pod-a"}, nil
}

func (r *countingSnapshotReader) ReadSnapshots(_ context.Context, _ string, _, _ int64) ([][]byte, error) {
	r.readCalls++
	return nil, nil
}

func TestRuntimeMetrics_InitialWindowCached(t *testing.T) {
	t.Parallel()

	reader := &countingSnapshotReader{}
	svc := metricsquery.NewRuntimeMetricsHandler(reader)
	ctx := context.Background()

	for range 2 {
		if _, _, err := svc.RuntimeMetrics(ctx, "1h", 0); err != nil {
			t.Fatalf("RuntimeMetrics initial window failed: %v", err)
		}
	}
	if reader.listCalls != 1 {
		t.Errorf("second since=0 request must hit the 5s cache; listCalls=%d", reader.listCalls)
	}

	if _, _, err := svc.RuntimeMetrics(ctx, "15m", 0); err != nil {
		t.Fatalf("RuntimeMetrics 15m failed: %v", err)
	}
	if reader.listCalls != 2 {
		t.Errorf("different range must have an independent cache entry; listCalls=%d", reader.listCalls)
	}

	// since>0 是增量轮询（只读尾部桶），不走结果缓存，也不把增量结果写进首屏缓存。
	if _, _, err := svc.RuntimeMetrics(ctx, "1h", time.Now().Unix()-10); err != nil {
		t.Fatalf("RuntimeMetrics incremental failed: %v", err)
	}
	if reader.listCalls != 3 {
		t.Errorf("incremental request must bypass the result cache; listCalls=%d", reader.listCalls)
	}
	if _, _, err := svc.RuntimeMetrics(ctx, "1h", 0); err != nil {
		t.Fatalf("RuntimeMetrics initial after incremental failed: %v", err)
	}
	if reader.listCalls != 3 {
		t.Errorf("incremental request must not overwrite the initial-window cache; listCalls=%d", reader.listCalls)
	}
}
