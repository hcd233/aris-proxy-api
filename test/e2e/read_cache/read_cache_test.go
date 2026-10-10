package read_cache

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const (
	webModelListPath   = "/api/web/v1/model/list?page=1&pageSize=50"
	webModelListAbsent = "/api/web/v1/model/list?page=1&pageSize=50&query=absent-keyword-xyz"
	openAIModelsPath   = "/api/openai/v1/models"
	clientModelsPath   = "/api/cli/v1/model/list"
)

// TestReadCache_ModelListHitsCache 命中缓存后不再回源：同参数重复拉列表，SQL 查询数严格下降
//
// 首次请求 = 分页 count + 行查询 + 端点回填 + 用户回填；缓存命中后只剩端点/用户回填，
// 分页行不再查库。
func TestReadCache_ModelListHitsCache(t *testing.T) { //nolint:paralleltest // 共享 sqlite 内存库与夹具，子测试必须串行
	f := newFixture(t, true, 20)

	first := f.doJSON(t, http.MethodGet, webModelListPath, f.jwt, "")
	jsonHas(t, first, "model-000")
	firstQueries := f.queryCount()

	second := f.doJSON(t, http.MethodGet, webModelListPath, f.jwt, "")
	jsonHas(t, second, "model-000")
	cachedQueries := f.queryCount() - firstQueries

	if cachedQueries == 0 {
		t.Fatalf("cached re-read issued no SQL query at all, fixture wiring is wrong")
	}
	if cachedQueries >= firstQueries {
		t.Fatalf("cache ineffective: first request %d queries, cached re-read %d queries", firstQueries, cachedQueries)
	}
}

// TestReadCache_EmptyResultNotRequeried 防缓存穿透：空结果写空值标记后，重复查询不存在的数据不再打库
func TestReadCache_EmptyResultNotRequeried(t *testing.T) { //nolint:paralleltest // 共享 sqlite 内存库与夹具，子测试必须串行
	f := newFixture(t, true, 20)

	first := f.doJSON(t, http.MethodGet, webModelListAbsent, f.jwt, "")
	jsonHas(t, first, `"total":0`)
	if f.queryCount() == 0 {
		t.Fatalf("first request for absent query issued no SQL query, fixture wiring is wrong")
	}

	for i := range 3 {
		start := f.queryCount()
		rsp := f.doJSON(t, http.MethodGet, webModelListAbsent, f.jwt, "")
		jsonHas(t, rsp, `"total":0`)
		if delta := f.queryCount() - start; delta != 0 {
			t.Fatalf("repeat %d for absent query still issued %d SQL queries (penetration)", i, delta)
		}
	}
}

// TestReadCache_EmptyResultUncachedControl 对照组：无缓存装配时空结果每次都查库
func TestReadCache_EmptyResultUncachedControl(t *testing.T) { //nolint:paralleltest // 共享 sqlite 内存库与夹具，子测试必须串行
	f := newFixture(t, false, 20)

	for i := range 2 {
		start := f.queryCount()
		f.doJSON(t, http.MethodGet, webModelListAbsent, f.jwt, "")
		if delta := f.queryCount() - start; delta == 0 {
			t.Fatalf("repeat %d without cache issued no SQL query, fixture wiring is wrong", i)
		}
	}
}

// TestReadCache_GatewayCatalogCached 网关/客户端模型目录接口命中缓存后不再查模型表
//
// API Key 中间件每请求仍有鉴权查询（2 条），故断言「缓存后查询数 = 鉴权查询数」。
func TestReadCache_GatewayCatalogCached(t *testing.T) { //nolint:paralleltest // 共享 sqlite 内存库与夹具，子测试必须串行
	f := newFixture(t, true, 20)

	authBaseline := int64(0)
	for _, tc := range []struct { //nolint:paralleltest // 夹具串行，子测试不并行
		name string
		path string
		want string
	}{
		{"openai models", openAIModelsPath, "model-000"},
		{"anthropic models", "/api/anthropic/v1/models", "model-000"},
		{"client model list", clientModelsPath, "model-000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first := f.doJSON(t, http.MethodGet, tc.path, f.apiKey, "")
			jsonHas(t, first, tc.want)
			firstQueries := f.queryCount()

			f.doJSON(t, http.MethodGet, tc.path, f.apiKey, "")
			cachedQueries := f.queryCount() - firstQueries

			start := f.queryCount()
			f.doJSON(t, http.MethodGet, tc.path, f.apiKey, "")
			againQueries := f.queryCount() - start

			if againQueries != cachedQueries {
				t.Fatalf("%s cached query count unstable: %d vs %d", tc.name, againQueries, cachedQueries)
			}
			if cachedQueries >= firstQueries {
				t.Fatalf("%s cache ineffective: first request %d queries, cached re-read %d queries", tc.name, firstQueries, cachedQueries)
			}
			if authBaseline == 0 {
				authBaseline = cachedQueries
			}
			if cachedQueries != authBaseline {
				t.Fatalf("%s cached re-read %d queries, want auth-only baseline %d", tc.name, cachedQueries, authBaseline)
			}
		})
	}
}

// TestReadCache_WriteInvalidates 写后失效：新建模型立即对全部读接口可见（不脏读）
func TestReadCache_WriteInvalidates(t *testing.T) { //nolint:paralleltest // 共享 sqlite 内存库与夹具，子测试必须串行
	f := newFixture(t, true, 5)

	data := f.doJSON(t, http.MethodGet, webModelListPath, f.jwt, "")
	jsonHas(t, data, "model-000")
	if strings.Contains(string(data), "model-999") {
		t.Fatalf("unexpected model before create: %s", data)
	}

	body := fmt.Sprintf(`{"alias":"model-999","upstreamModel":"up-999","endpointID":%d}`, f.endpointID)
	f.doJSON(t, http.MethodPost, "/api/web/v1/model", f.jwt, body)

	data = f.doJSON(t, http.MethodGet, webModelListPath, f.jwt, "")
	jsonHas(t, data, "model-999")

	aliases := f.doJSON(t, http.MethodGet, openAIModelsPath, f.apiKey, "")
	jsonHas(t, aliases, "model-999")

	details := f.doJSON(t, http.MethodGet, clientModelsPath, f.apiKey, "")
	jsonHas(t, details, "model-999")
}

// TestReadCache_TTLJitter 防缓存雪崩：同批缓存条目 TTL 互不相同（随机抖动打散过期时刻）
func TestReadCache_TTLJitter(t *testing.T) { //nolint:paralleltest // 共享 sqlite 内存库与夹具，子测试必须串行
	f := newFixture(t, true, 5)

	// 触发多条不同 key 的缓存写入：alias / detail / 两个不同查询签名的模型列表
	f.doJSON(t, http.MethodGet, openAIModelsPath, f.apiKey, "")
	f.doJSON(t, http.MethodGet, clientModelsPath, f.apiKey, "")
	f.doJSON(t, http.MethodGet, "/api/web/v1/model/list?page=1&pageSize=10", f.jwt, "")
	f.doJSON(t, http.MethodGet, "/api/web/v1/model/list?page=1&pageSize=11", f.jwt, "")

	var keys []string
	for _, key := range f.mr.Keys() {
		if strings.HasPrefix(key, "cache:rd:") {
			keys = append(keys, key)
		}
	}
	if len(keys) < 2 {
		t.Fatalf("expected multiple cached keys, got %v", keys)
	}
	distinct := map[int64]bool{}
	for _, key := range keys {
		distinct[int64(f.mr.TTL(key))] = true
	}
	if len(distinct) < 2 {
		t.Fatalf("all cached keys share one TTL %v, jitter not applied (avalanche risk)", f.mr.TTL(keys[0]))
	}
}
