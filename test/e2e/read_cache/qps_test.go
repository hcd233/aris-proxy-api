package read_cache

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// QPS 对比压测：同一条端到端链路（RegisterAPIRouter + APIKey/JWT 中间件 + 真实仓储/用例）
// 分别以「无缓存装配（= 加缓存前）」与「有缓存装配（= 加缓存后）」压测高频读接口。
//
// 口径说明：
//   - 网关/客户端模型目录接口（openai/anthropic/cli）无限流，直接测 QPS。
//   - Web 管理接口受 TokenBucket 限流（constant.LimitManageAPIKey = 20 次/分钟/用户），
//     吞吐上限由限流器决定，改测顺序请求的平均延迟。
//   - sqlite 内存库 + miniredis 均在进程内，测得的是受控环境下的相对值；生产 PostgreSQL
//     往返（网络 + 解析）远高于内存 sqlite，真实收益不低于该比值。
//   - DB 连接固定 1 条（fixture 内 SetMaxOpenConns(1)），模拟生产 DB 连接池瓶颈，
//     让「少查库」直接转化为吞吐提升。

const (
	qpsModelCount = 200
	qpsWorkers    = 8
	qpsDuration   = 2 * time.Second
	qpsLatSamples = 10
)

type qpsTarget struct {
	name   string
	path   string
	useJWT bool
}

var qpsCatalogTargets = []qpsTarget{
	{name: "GET /api/openai/v1/models", path: openAIModelsPath},
	{name: "GET /api/anthropic/v1/models", path: "/api/anthropic/v1/models"},
	{name: "GET /api/cli/v1/model/list", path: clientModelsPath},
}

var qpsLatencyTarget = qpsTarget{name: "GET /api/web/v1/model/list", path: webModelListPath, useJWT: true}

func TestReadCacheQPSBeforeAfter(t *testing.T) { //nolint:paralleltest // 独占压测夹具，避免并行干扰吞吐测量
	before := newFixture(t, false, qpsModelCount)
	after := newFixture(t, true, qpsModelCount)

	type qpsRow struct {
		name      string
		beforeQPS float64
		afterQPS  float64
		speedup   float64
	}
	rows := make([]qpsRow, 0, len(qpsCatalogTargets))
	for _, target := range qpsCatalogTargets {
		beforeQPS := measureQPS(t, before, target)
		afterQPS := measureQPS(t, after, target)
		rows = append(rows, qpsRow{name: target.name, beforeQPS: beforeQPS, afterQPS: afterQPS, speedup: afterQPS / beforeQPS})
	}

	beforeLat := measureAvgLatency(t, before, qpsLatencyTarget)
	afterLat := measureAvgLatency(t, after, qpsLatencyTarget)

	t.Log("==== 高频读接口加缓存前后对比（workers=8, 2s/项, sqlite+miniredis 进程内环境）====")
	for _, row := range rows {
		t.Logf("%-34s QPS 前 %8.0f → 后 %8.0f（提升 %.2fx）", row.name, row.beforeQPS, row.afterQPS, row.speedup)
	}
	t.Logf("%-34s 平均延迟 前 %8.3fms → 后 %8.3fms（降低 %.2fx，manage API 限流 20/min 不测吞吐）",
		qpsLatencyTarget.name, float64(beforeLat.Microseconds())/1000, float64(afterLat.Microseconds())/1000,
		float64(beforeLat.Microseconds())/float64(afterLat.Microseconds()))

	for _, row := range rows {
		if row.afterQPS <= row.beforeQPS {
			t.Errorf("%s: cache must improve throughput, before %.0f QPS vs after %.0f QPS",
				row.name, row.beforeQPS, row.afterQPS)
		}
	}
	if afterLat >= beforeLat {
		t.Errorf("%s: cache must reduce latency, before %v vs after %v", qpsLatencyTarget.name, beforeLat, afterLat)
	}
}

// measureQPS 在给定夹具上对目标接口持续压测，返回 QPS
func measureQPS(t *testing.T, f *fixture, target qpsTarget) float64 {
	t.Helper()
	// 稳态预热：排除首次回源与连接建立
	f.doJSON(t, http.MethodGet, target.path, target.token(f), "")

	var total atomic.Int64
	var failed atomic.Int64
	start := time.Now()
	deadline := start.Add(qpsDuration)
	var wg sync.WaitGroup
	for range qpsWorkers {
		wg.Go(func() {
			for time.Now().Before(deadline) {
				status, err := f.get(target.path, target.token(f))
				if err != nil || status != http.StatusOK {
					failed.Add(1)
					continue
				}
				total.Add(1)
			}
		})
	}
	wg.Wait()
	elapsed := time.Since(start)
	if failed.Load() > 0 {
		t.Fatalf("%s: %d requests failed under load", target.name, failed.Load())
	}
	qps := float64(total.Load()) / elapsed.Seconds()
	t.Logf("[压测] %-34s %d requests / %.2fs (workers=%d) → %.0f QPS",
		target.name, total.Load(), elapsed.Seconds(), qpsWorkers, qps)
	return qps
}

// measureAvgLatency 顺序请求取平均延迟（用于受 manage API 限流的接口）
func measureAvgLatency(t *testing.T, f *fixture, target qpsTarget) time.Duration {
	t.Helper()
	f.doJSON(t, http.MethodGet, target.path, target.token(f), "")

	var total time.Duration
	for range qpsLatSamples {
		start := time.Now()
		status, err := f.get(target.path, target.token(f))
		if err != nil || status != http.StatusOK {
			t.Fatalf("%s: request failed (status=%d err=%v)", target.name, status, err)
		}
		total += time.Since(start)
	}
	avg := total / qpsLatSamples
	t.Logf("[延迟] %-34s %d 次顺序请求平均 %v", target.name, qpsLatSamples, avg)
	return avg
}

// token 返回目标接口所需凭据（API Key 或 JWT）
func (t qpsTarget) token(f *fixture) string {
	if t.useJWT {
		return f.jwt
	}
	return f.apiKey
}
