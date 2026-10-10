# 性能优化 QPS 基准测试（7 项）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 对 7 项已落地的性能优化做可复现的 QPS 基准测试，每项产出一份含技术细节与提升数据的中文 md 文档。

**Architecture:** 新增 `test/perf/` 独立测试装置（mock LLM 上游 + 对比端点 harness + wrk 运行脚本），不进入生产构建。harness 承载 1/2/4/7 项对比端点，主服务本地实例承载 3/5/6 项端到端压测；DB/Redis 走生产服务器本机 Docker（同机拓扑，避免跨机 RTT 淹没差异）。生产代码仅一处改动：连接池常量改 env 覆盖（默认值不变）。

**Tech Stack:** Go 1.25.1、fiber v3.3 + huma v2.38（humafiber）、sonic v1.15、gorm + Postgres、go-redis v9、Pond、wrk。

**Spec:** `docs/superpowers/specs/2026-10-10-perf-qps-benchmark-design.md`

## Global Constraints

- 每组压测：warmup 10s → 正式 30s × 5 轮，取中位数，报告极差；并发档 50/100/200，主结论取 100 档。
- 被测进程 `GOMAXPROCS=8`，wrk `-t 4`；单变量原则：每组对比只允许一个差异。
- **绝不压线上服务端口**；只压 127.0.0.1:18080（主服务实例）/ 18081（harness）/ 18082（harness 备用）。
- 压测仅低峰期执行；单轮 30s；异常立即 kill 被测进程。执行时机由用户指定后再跑。
- 写入隔离：全部写入经生产库测试 API Key（名称 `perf-bench-2026-10`）落库，结束后按 `api_key_id` 清理。
- 索引测试纯只读：`SET LOCAL enable_indexscan=off; enable_bitmapscan=off` 模拟无索引，不建不删生产索引。
- 唯一生产代码改动：`internal/common/constant/database.go` 连接池常量支持 env 覆盖（默认值 10/100/5h 不变），必须附回归测试。
- 文档中文，放 `docs/perf/`；原始 wrk 输出存 `docs/perf/raw/`；凭据（密码、token、连接串明文）不进仓库与文档。
- 生产服务器操作遵循 `login-prod-server` skill 安全基线；所有命令经 `rtk` 前缀执行。

## File Structure

```
test/perf/
  cmd/mockupstream/main.go    # 假 LLM 上游：固定 JSON/SSE，2KB/20KB/100KB 三档
  cmd/harness/main.go         # 对比端点宿主（-mode 选择注册哪组端点）
  bench/json.go               # sonic vs encoding/json 端点（两组实现共享同一处理逻辑）
  bench/store.go              # 同步 vs 异步落库端点（真实写 model_call_audits）
  bench/query.go              # 有/无索引并发查询端点
  bench/framework.go          # fiber vs huma 四组端点（同一 DTO）
  bench/dto.go                # 四组共用的典型 LLM 请求 DTO（20 字段 + 校验 tag）
  run-bench.sh                # wrk 运行器：warmup + 5 轮 + 解析 + 存 raw
docs/perf/
  01-sonic-vs-encoding-json.md … 07-compound-index.md   # 7 份产出文档
  raw/                        # 原始 wrk 输出（git 跟踪）
internal/common/constant/database.go        # 唯一生产代码改动点
test/unit/dbpool/dbpool_test.go             # 连接池默认值回归测试
```

---

### Task 0: 生产侧环境准备（测试 Key、部署通道）

**Files:**
- Create: 无代码文件（纯操作 + 记录）
- Test: 无

**Interfaces:**
- Produces: 生产库测试 API Key（名称 `perf-bench-2026-10`，记下 `api_key_id` 与 `key` 值，写入本机 `~/.perf-bench.env`，**不进仓库**）；生产服务器上可运行 `test/perf` 二进制的目录约定 `$HOME/perf-bench/`。

- [ ] **Step 1: 登录生产服务器并确认 Docker 内 Postgres/Redis 可达**

加载 `login-prod-server` skill 后执行（连接方式以 skill 为准）：

```bash
ssh api.lvlvko.top 'docker ps --format "{{.Names}} {{.Ports}}" | grep -Ei "postgres|redis"'
```

Expected: 看到 Postgres 与 Redis 容器名；记录容器名，后续 `docker exec <name> psql ...` 使用。

- [ ] **Step 2: 创建测试 API Key（只插入一条，用后清理）**

```bash
ssh api.lvlvko.top 'docker exec -i <postgres容器> psql -U <user> -d <db> -c "
INSERT INTO proxy_api_keys (user_id, name, key, deleted_at, created_at, updated_at)
SELECT id, 'perf-bench-2026-10', 'pb-20261010-perf-bench-key', 0, now(), now()
FROM users WHERE permission = 'admin' ORDER BY id LIMIT 1
RETURNING id, name;"'
```

Expected: 返回一行 `id | perf-bench-2026-10`。把 `id` 与 key 值记入 `~/.perf-bench.env`：

```bash
cat >> ~/.perf-bench.env <<'EOF'
PERF_BENCH_API_KEY_ID=<上一步返回的 id>
PERF_BENCH_API_KEY=pb-20261010-perf-bench-key
PERF_BENCH_PG_DSN=postgres://<user>:<pass>@127.0.0.1:5432/<db>?sslmode=disable
PERF_BENCH_REDIS_ADDR=127.0.0.1:6379
EOF
chmod 600 ~/.perf-bench.env
```

- [ ] **Step 3: 验证测试 Key 可通过鉴权（低速 1 次）**

```bash
curl -s -o /dev/null -w '%{http_code}\n' -H 'Authorization: Bearer pb-20261010-perf-bench-key' \
  http://127.0.0.1:18080/api/v1/... # 部署主服务实例后验证，见 Task 5 Step 1
```

Expected: 200（非 401/403）。若部署尚未完成，本步顺延到 Task 5 Step 1 之后执行。

- [ ] **Step 4: 记录并 Commit 环境约定（无凭据）**

```bash
git add test/perf/.gitkeep 2>/dev/null; true
git commit -m "chore(perf): 建立 perf 基准测试目录骨架" --allow-empty
```

---

### Task 1: mock LLM 上游

**Files:**
- Create: `test/perf/cmd/mockupstream/main.go`
- Test: `test/perf/cmd/mockupstream/main_test.go`

**Interfaces:**
- Produces: HTTP 服务（默认 `127.0.0.1:18090`）：
  - `POST /v1/chat/completions` → 固定 OpenAI completion JSON，大小由请求 body 的 `"model"` 字段值决定：`size-2k`/`size-20k`/`size-100k`（默认 2k）
  - `GET /sse` → 固定 20 帧 SSE 流后 `[DONE]`
  - `GET /fixed/{kb}` → 约 `{kb}` KB 的固定 JSON（compress 项直接压测用）

- [ ] **Step 1: Write the failing test**

```go
// test/perf/cmd/mockupstream/main_test.go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatCompletionsSizeByModel(t *testing.T) {
	for _, tc := range []struct {
		model    string
		wantSize int
	}{
		{"size-2k", 2 * 1024},
		{"size-20k", 20 * 1024},
		{"size-100k", 100 * 1024},
	} {
		t.Run(tc.model, func(t *testing.T) {
			body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, tc.model)
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			rec := httptest.NewRecorder()
			newMux().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			got := rec.Body.Len()
			if got < tc.wantSize || got > tc.wantSize*11/10 {
				t.Fatalf("body size = %d, want ~%d", got, tc.wantSize)
			}
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
		})
	}
}

func TestSSEStream(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Fatal("missing [DONE] terminator")
	}
	if n := strings.Count(rec.Body.String(), "data: {"); n != 20 {
		t.Fatalf("sse frame count = %d, want 20", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./test/perf/cmd/mockupstream/ -run 'TestChatCompletionsSizeByModel|TestSSEStream' -v`
Expected: FAIL，`newMux undefined`。

- [ ] **Step 3: Write minimal implementation**

```go
// test/perf/cmd/mockupstream/main.go
// mockupstream 假 LLM 上游：供 perf 压测使用，返回固定体积 JSON/SSE，绝不转发真实 LLM API。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18090", "listen address")
	flag.Parse()
	log.Fatal(http.ListenAndServe(*addr, newMux()))
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", handleCompletion)
	mux.HandleFunc("GET /sse", handleSSE)
	mux.HandleFunc("GET /fixed/{kb}", handleFixed)
	return mux
}

// padJSON 生成约 targetBytes 大小的 JSON 响应：固定骨架 + filler 字段补齐。
func padJSON(prefix map[string]any, targetBytes int) []byte {
	base, _ := json.Marshal(prefix)
	if filler := targetBytes - len(base) - len(`,"filler":""}`); filler > 0 {
		prefix["filler"] = strings.Repeat("x", filler)
	}
	out, _ := json.Marshal(prefix)
	return out
}

func completionBody(model string) []byte {
	target := 2 * 1024
	switch model {
	case "size-20k":
		target = 20 * 1024
	case "size-100k":
		target = 100 * 1024
	}
	return padJSON(map[string]any{
		"id":      "chatcmpl-perfbench",
		"object":  "chat.completion",
		"model":   model,
		"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
		"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12},
	}, target)
}

func handleCompletion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(completionBody(req.Model))
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	for i := range 20 {
		frame := fmt.Sprintf(`data: {"id":"c%d","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"tok%d"}}]}`+"\n\n", i, i)
		_, _ = w.Write([]byte(frame))
	}
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
}

func handleFixed(w http.ResponseWriter, r *http.Request) {
	kb, _ := strconv.Atoi(r.PathValue("kb"))
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(padJSON(map[string]any{"kind": "fixed"}, kb*1024))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./test/perf/cmd/mockupstream/ -v`
Expected: PASS（3 个 size 子测试 + SSE）。

- [ ] **Step 5: Commit**

```bash
git add test/perf/cmd/mockupstream/
git commit -m "test(perf): 新增 mock LLM 上游（固定体积 JSON/SSE）"
```

---

### Task 2: wrk 运行器与原始数据规约

**Files:**
- Create: `test/perf/run-bench.sh`
- Test: `test/perf/run-bench_test.sh`（shell 冒烟：对 mockupstream 跑 2s×2 轮验证输出文件生成）

**Interfaces:**
- Produces: `run-bench.sh <name> <url> <wrk-args...>`，行为：warmup 10s（丢弃）→ 5 × 30s → 汇总 QPS/p50/p90/p99 中位数与极差 → 写 `docs/perf/raw/<name>.txt`（全部原始输出）与 `docs/perf/raw/<name>.summary.txt`（汇总表）。并发档通过 `-c` 传入。

- [ ] **Step 1: Write the failing smoke test**

```bash
#!/usr/bin/env bash
# test/perf/run-bench_test.sh — 冒烟：验证 run-bench.sh 产出原始输出与汇总文件
set -euo pipefail
cd "$(dirname "$0")"
go run ./cmd/mockupstream -addr 127.0.0.1:18099 &
MOCK_PID=$!
trap 'kill $MOCK_PID 2>/dev/null || true' EXIT
sleep 1
OUT_DIR="$(mktemp -d)" ./run-bench.sh smoke http://127.0.0.1:18099/fixed/2 -t 2 -c 4 -d 2s -R 2
test -s "$OUT_DIR/smoke.txt" || { echo "missing raw output"; exit 1; }
grep -q 'Requests/sec' "$OUT_DIR/smoke.summary.txt" || { echo "missing summary"; exit 1; }
echo "smoke OK"
```

- [ ] **Step 2: Run smoke test to verify it fails**

Run: `bash test/perf/run-bench_test.sh`
Expected: FAIL，`run-bench.sh: No such file or directory`。

- [ ] **Step 3: Write run-bench.sh**

```bash
#!/usr/bin/env bash
# run-bench.sh — wrk 压测运行器：warmup + N 轮固定时长，解析 QPS/延迟分位，中位数汇总。
# 用法: run-bench.sh <name> <url> [wrk 额外参数...]
# 环境: OUT_DIR(默认 docs/perf/raw) ROUNDS(默认 5) DURATION(默认 30s) WARMUP(默认 10s)
set -euo pipefail
NAME="$1"; URL="$2"; shift 2
OUT_DIR="${OUT_DIR:-docs/perf/raw}"
ROUNDS="${ROUNDS:-5}"
DURATION="${DURATION:-30s}"
WARMUP="${WARMUP:-10s}"
mkdir -p "$OUT_DIR"
RAW="$OUT_DIR/$NAME.txt"; SUMMARY="$OUT_DIR/$NAME.summary.txt"
: > "$RAW"

echo "== warmup $WARMUP ($URL)" | tee -a "$RAW"
wrk -t 4 -d "$WARMUP" "$@" "$URL" >> "$RAW" 2>&1 || true

QPS_LIST=(); P50_LIST=(); P90_LIST=(); P99_LIST=()
for i in $(seq 1 "$ROUNDS"); do
  echo "== round $i ($DURATION)" | tee -a "$RAW"
  wrk -t 4 -d "$DURATION" --latency "$@" "$URL" >> "$RAW" 2>&1
  QPS_LIST+=("$(grep 'Requests/sec' "$RAW" | tail -1 | awk '{print $2}')")
  P50_LIST+=("$(grep -A3 'Latency Distribution' "$RAW" | tail -3 | sed -n 1p | awk '{print $2}')")
  P90_LIST+=("$(grep -A3 'Latency Distribution' "$RAW" | tail -3 | sed -n 2p | awk '{print $2}')")
  P99_LIST+=("$(grep -A3 'Latency Distribution' "$RAW" | tail -3 | sed -n 3p | awk '{print $2}')")
  sleep 5
done

median() { printf '%s\n' "$@" | sort -g | awk '{a[NR]=$1} END {print a[int((NR+1)/2)]}'; }
range()  { printf '%s\n' "$@" | sort -g | awk 'NR==1{min=$1} {max=$1} END {printf "%s..%s", min, max}'; }

{
  echo "name=$NAME url=$URL rounds=$ROUNDS duration=$DURATION warmup=$WARMUP"
  echo "qps_median=$(median "${QPS_LIST[@]}")  qps_range=$(range "${QPS_LIST[@]}")"
  echo "p50_median=$(median "${P50_LIST[@]}")  p50_range=$(range "${P50_LIST[@]}")"
  echo "p90_median=$(median "${P90_LIST[@]}")  p90_range=$(range "${P90_LIST[@]}")"
  echo "p99_median=$(median "${P99_LIST[@]}")  p99_range=$(range "${P99_LIST[@]}")"
} | tee "$SUMMARY"
```

- [ ] **Step 4: Run smoke test to verify it passes**

Run: `bash test/perf/run-bench_test.sh`
Expected: `smoke OK`（时长受 `-d 2s -R 2` 环境变量控制）。

- [ ] **Step 5: Commit**

```bash
git add test/perf/run-bench.sh test/perf/run-bench_test.sh
chmod +x test/perf/run-bench.sh test/perf/run-bench_test.sh
git commit -m "test(perf): wrk 运行器（warmup+5 轮、中位数汇总、原始数据存档）"
```

---

### Task 3: 优化项 1 — sonic vs encoding/json（文档 01）

**Files:**
- Create: `test/perf/bench/json.go`、`test/perf/cmd/harness/main.go`（本任务只含 `-mode=json-sonic|json-stdlib` 部分）
- Test: `test/perf/bench/json_test.go`
- Doc: `docs/perf/01-sonic-vs-encoding-json.md`

**Interfaces:**
- Consumes: `run-bench.sh`（Task 2）
- Produces: `bench.RegisterJSON(mux *http.ServeMux, lib string)` 注册 `POST /bench/json`（lib ∈ `sonic`/`stdlib`）；`harness -mode=json-sonic|json-stdlib -addr 127.0.0.1:18081`。请求体为 10KB 典型 LLM 请求 JSON，handler Unmarshal → 改 `model` 字段 → Marshal 返回。

- [ ] **Step 1: Write the failing test**

```go
// test/perf/bench/json_test.go
package bench

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONEchoRoundTrip(t *testing.T) {
	for _, lib := range []string{"sonic", "stdlib"} {
		t.Run(lib, func(t *testing.T) {
			mux := http.NewServeMux()
			RegisterJSON(mux, lib)
			body := SampleLLMRequestJSON(10 * 1024) // ~10KB
			req := httptest.NewRequest(http.MethodPost, "/bench/json", strings.NewReader(body))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			out, _ := io.ReadAll(rec.Body)
			if !strings.Contains(string(out), `"model":"bench-model"`) {
				t.Fatalf("model field not rewritten: %.80s", out)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./test/perf/bench/ -run TestJSONEchoRoundTrip -v`
Expected: FAIL，`undefined: RegisterJSON / SampleLLMRequestJSON`。

- [ ] **Step 3: Implement json.go（两组仅编解码库不同，其余逐行相同）**

```go
// test/perf/bench/json.go — JSON 库对比端点：sonic vs encoding/json。
package bench

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
)

// LLMRequest 与项目 LLM 请求同构的典型 DTO（messages 嵌套 + usage），压测对象。
type LLMRequest struct {
	Model    string         `json:"model"`
	Messages []LLMMessage   `json:"messages"`
	Tools    []LLMTool      `json:"tools,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type LLMTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

// SampleLLMRequestJSON 生成约 target 字节数的典型 LLM 请求 JSON。
func SampleLLMRequestJSON(target int) string {
	req := LLMRequest{Model: "bench-model", Metadata: map[string]any{}}
	for i := range 12 {
		req.Messages = append(req.Messages, LLMMessage{Role: "user", Content: strings.Repeat("m", 400) + string(rune('a'+i))})
	}
	raw, _ := json.Marshal(req)
	if pad := target - len(raw) - 14; pad > 0 {
		req.Metadata["filler"] = strings.Repeat("x", pad)
	}
	raw, _ = json.Marshal(req)
	return string(raw)
}

// RegisterJSON 注册 /bench/json：Unmarshal → 改 model → Marshal 返回。
// lib 决定编解码实现，两分支其余逻辑完全一致。
func RegisterJSON(mux *http.ServeMux, lib string) {
	mux.HandleFunc("POST /bench/json", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req LLMRequest
		var out []byte
		if lib == "sonic" {
			_ = sonic.Unmarshal(body, &req)
			req.Model = "bench-model"
			out, _ = sonic.Marshal(&req)
		} else {
			_ = json.Unmarshal(body, &req)
			req.Model = "bench-model"
			out, _ = json.Marshal(&req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	})
}
```

（`io` 需补 import；`SampleLLMRequestJSON` 复用 `encoding/json` 生成样本本身不参与压测路径。）

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./test/perf/bench/ -v`
Expected: PASS。

- [ ] **Step 5: 实测（执行时机由用户指定，低峰期）**

```bash
# 服务侧（生产服务器或本地，两个进程）
GOMAXPROCS=8 go run ./test/perf/cmd/harness -mode=json-sonic  -addr 127.0.0.1:18081 &
GOMAXPROCS=8 go run ./test/perf/cmd/harness -mode=json-stdlib -addr 127.0.0.1:18082 &

# 压测（同机、同参数；并发三档各跑一轮完整 5×30s）
BODY=$(mktemp); go run ./test/perf/cmd/genbody > "$BODY"   # 输出 SampleLLMRequestJSON(10KB)
for c in 50 100 200; do
  ./test/perf/run-bench.sh 01-sonic-c$c  http://127.0.0.1:18081/bench/json -c $c -s "$BODY" -H 'Content-Type: application/json' -T 1
  ./test/perf/run-bench.sh 01-stdlib-c$c http://127.0.0.1:18082/bench/json -c $c -s "$BODY" -H 'Content-Type: application/json' -T 1
done
```

补充微基准（辅助证据）：`test/perf/bench/json_bench_test.go` 对 `SampleLLMRequestJSON(10KB)` 做 `BenchmarkSonicMarshal/Unmarshal` 与 `BenchmarkStdlibMarshal/Unmarshal`，`rtk go test ./test/perf/bench/ -bench . -benchmem`，结果贴入文档。

- [ ] **Step 6: 撰写文档 01**

`docs/perf/01-sonic-vs-encoding-json.md` 结构（数据表填 Step 5 中位数/极差）：

```markdown
# sonic 替换 encoding/json 的 QPS 提升
## 1. 优化背景与技术细节
（引用 internal/api/fiber.go:24-25 JSONEncoder/JSONDecoder、业务层 sonic.Marshal 散点、
sonic.NoCopyRawMessage 零拷贝；说明 sonic 为 JIT/AOT 加速 JSON 库）
## 2. 测试方法
（拓扑图、对比组 encoding/json vs sonic、10KB payload、warmup/轮次/并发档、单变量说明）
## 3. 原始数据
（表格：并发档 × 实现 → QPS 中位数(极差) / p50 / p90 / p99；微基准 ns/op、B/op、allocs/op 表）
## 4. 结论：QPS 提升
（提升倍数/百分比 = sonic ÷ stdlib，按并发档分列；给出结论句）
## 5. 适用边界与口径
（收益集中在 JSON 密集路径；大 payload 收益更明显；本测试不含 DB/上游延迟）
## 6. 复现命令
（完整命令序列 + git commit + 环境信息）
```

- [ ] **Step 7: Commit**

```bash
git add test/perf/bench/ test/perf/cmd/ docs/perf/01-sonic-vs-encoding-json.md docs/perf/raw/
git commit -m "test(perf): 优化项1 sonic vs encoding/json 基准与文档"
```

---

### Task 4: 优化项 2 — Fiber vs 纯 huma（文档 02）

**Files:**
- Create: `test/perf/bench/dto.go`、`test/perf/bench/framework.go`
- Modify: `test/perf/cmd/harness/main.go`（新增 `-mode=bare-net|humа-stdlib|bare-fiber|huma-fiber`）
- Test: `test/perf/bench/framework_test.go`
- Doc: `docs/perf/02-fiber-vs-huma-stdlib.md`

**Interfaces:**
- Consumes: Task 3 的 harness 骨架
- Produces: `bench.RegisterBareNet(mux *http.ServeMux)` / `bench.RegisterHumaStdlib(mux *http.ServeMux)` / `bench.RegisterBareFiber(app *fiber.App)` / `bench.RegisterHumaFiber(app *fiber.App)`，四者注册同一 `POST /bench/echo`，DTO 为 `bench.EchoReq`（path 参数 `id` + query 参数 `page` + 20 字段 body，含 `required`/`enum`/`format` 校验 tag），handler 返回 `{"id":...,"ok":true}`。每次启动只注册一组，端口固定 18081（消除端口偏倚）。

- [ ] **Step 1: Write the failing test**

```go
// test/perf/bench/framework_test.go
package bench

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/adapters/humafiber"
	"github.com/danielgtaylor/huma/v2"
)

func postEcho(t *testing.T, h http.Handler) {
	t.Helper()
	body := strings.NewReader(`{"name":"n","model":"m","stream":true,"temperature":0.7,"user":"u","messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/bench/echo/42?page=1", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"ok":true`)) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestBareNet(t *testing.T)      { postEcho(t, bareNetHandler()) }
func TestHumaStdlib(t *testing.T)   { postEcho(t, humaStdlibHandler()) }
func TestBareFiber(t *testing.T)    { postEcho(t, bareFiberHandler()) }
func TestHumaFiber(t *testing.T)    { postEcho(t, humaFiberHandler()) }
```

（四个 `xxxHandler()` 帮助函数各自返回 `http.Handler`：fiber 侧用 `app.Test(req)` 无法复用 `postEcho`，改为 fiber 侧包装 `http.HandlerFunc` 调 `app.Test`。实现时保持四组断言一致。）

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./test/perf/bench/ -run 'TestBareNet|TestHumaStdlib|TestBareFiber|TestHumaFiber' -v`
Expected: FAIL，`undefined: bareNetHandler` 等。

- [ ] **Step 3: Implement dto.go + framework.go**

```go
// test/perf/bench/dto.go — 四组框架对比共用的典型 DTO（20 字段 + 校验 tag）。
package bench

// EchoReq 模拟 LLM 请求的校验负载：path id + query page + 结构化 body。
type EchoReq struct {
	Body struct {
		Name                 string   `json:"name" required:"true" maxLength:"128"`
		Model                string   `json:"model" required:"true" enum:"m,gpt-x,claude-y"`
		Stream               bool     `json:"stream"`
		Temperature          float32  `json:"temperature" minimum:"0" maximum:"2"`
		TopP                 float32  `json:"top_p" minimum:"0" maximum:"1"`
		MaxTokens            int      `json:"max_tokens" minimum:"1"`
		Stop                 []string `json:"stop" maxItems:"4"`
		User                 string   `json:"user"`
		System               string   `json:"system"`
		PresencePenalty      float32  `json:"presence_penalty"`
		FrequencyPenalty     float32  `json:"frequency_penalty"`
		Seed                 *int     `json:"seed"`
		N                    int      `json:"n" minimum:"1" maximum:"8"`
		ResponseFormat       string   `json:"response_format" enum:"text,json_object"`
		Logprobs             bool     `json:"logprobs"`
		TopLogprobs          int      `json:"top_logprobs" minimum:"0" maximum:"20"`
		ToolChoice           string   `json:"tool_choice" enum:"auto,none,required"`
		ParallelToolCalls    bool     `json:"parallel_tool_calls"`
		Metadata             map[string]string `json:"metadata"`
		Messages             []struct {
			Role    string `json:"role" required:"true" enum:"system,user,assistant,tool"`
			Content string `json:"content" required:"true"`
		} `json:"messages" required:"true"`
	} `json:"body" required:"true"`
}
```

```go
// test/perf/bench/framework.go — 四组 HTTP 运行时对比端点。
// bare-* = 裸框架 handler；huma-* = 同一 EchoReq 经 huma 校验。
package bench

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/danielgtaylor/huma/v2/adapters/humamux"
	"github.com/gofiber/fiber/v3"
)

func echoResponse(id int) []byte {
	out, _ := json.Marshal(map[string]any{"id": id, "ok": true})
	return out
}

// ① 裸 net/http：手工解 JSON + 校验 required 字段（与 huma 校验同语义的最小集）。
func bareNetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req EchoReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusUnprocessableEntity)
			return
		}
		if req.Body.Name == "" || req.Body.Model == "" {
			http.Error(w, "missing required", http.StatusUnprocessableEntity)
			return
		}
		id, _ := strconv.Atoi(r.PathValue("id"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(echoResponse(id))
	})
}

// ② huma + stdlib（gorilla mux adapter，huma 校验完整生效）。
func humaStdlibHandler() http.Handler {
	mux := http.NewServeMux() // 若 humamux 需要 gorilla mux，用 humamux.New(mux)；见下方注释
	api := humamux.New(mux, huma.DefaultConfig("perf", "1.0.0"))
	huma.Register(api, huma.Operation{
		OperationID: "echo", Method: http.MethodPost,
		Path: "/bench/echo/{id}",
	}, func(ctx context.Context, input *struct {
		ID   int    `path:"id"`
		Page int    `query:"page"`
		Body EchoReq `json:"body"`
	}) (*struct{ Body struct {
		ID int  `json:"id"`
		OK bool `json:"ok"`
	} }, error) {
		return nil, nil // 实现时填充：返回 {id, ok:true}
	})
	return mux
}

// ③ 裸 Fiber：同 ① 的手工校验，走 fiber ctx。
func bareFiberHandler() *fiber.App {
	app := fiber.New(fiber.Config{JSONEncoder: json.Marshal, JSONDecoder: json.Unmarshal})
	app.Post("/bench/echo/:id", func(c fiber.Ctx) error {
		var req EchoReq
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).SendString("bad json")
		}
		if req.Body.Name == "" || req.Body.Model == "" {
			return c.Status(fiber.StatusUnprocessableEntity).SendString("missing required")
		}
		id, _ := strconv.Atoi(c.Params("id"))
		return c.JSON(map[string]any{"id": id, "ok": true})
	})
	return app
}

// ④ huma + Fiber（项目现状形态：humafiber.New + huma.Register）。
func humaFiberHandler() *fiber.App {
	app := fiber.New(fiber.Config{JSONEncoder: json.Marshal, JSONDecoder: json.Unmarshal})
	api := humafiber.New(app, huma.DefaultConfig("perf", "1.0.0"))
	huma.Register(api, huma.Operation{
		OperationID: "echo", Method: http.MethodPost,
		Path: "/bench/echo/{id}",
	}, echoHumaHandler)
	return app
}
```

实现注意（写进代码注释）：
- huma 输入 struct 定义一次（`echoHumaInput`）供 ②④ 共用，签名 `(ctx, input *echoHumaInput) (*echoHumaOutput, error)`，避免两处漂移。
- ①③ 的手工校验只做 `required` 最小子集，文档注明"huma 侧校验更完整，此处取同语义下限"。
- `humamux` 若与 `http.ServeMux` 不兼容，② 改用 `gorilla/mux`（go.mod 已间接依赖）或 huma 官方 stdlib adapter，以实际可编译为准。

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./test/perf/bench/ -v`
Expected: 4 个框架子测试全部 PASS。

- [ ] **Step 5: 实测（四组，一次一组，端口固定 18081）**

```bash
BODY=$(mktemp); go run ./test/perf/cmd/genbody > "$BODY"
for mode in bare-net huma-stdlib bare-fiber huma-fiber; do
  GOMAXPROCS=8 go run ./test/perf/cmd/harness -mode=$mode -addr 127.0.0.1:18081 &
  SRV=$!; sleep 1
  for c in 50 100 200; do
    ./test/perf/run-bench.sh 02-$mode-c$c http://127.0.0.1:18081/bench/echo/42 -c $c \
      -s "$BODY" -H 'Content-Type: application/json' -T 1
  done
  kill $SRV
done
```

- [ ] **Step 6: 撰写文档 02**

结构同文档模板；数据表按「4 组 × 3 并发档」列出；结论必须回答三问：
1. fiber 收益 = ④ vs ②（核心结论，QPS 提升 X 倍）
2. huma 层开销 = ② vs ①、④ vs ③（百分比）
3. fiber 极限 = ③ vs ①

- [ ] **Step 7: Commit**

```bash
git add test/perf/bench/ docs/perf/02-fiber-vs-huma-stdlib.md docs/perf/raw/
git commit -m "test(perf): 优化项2 fiber vs huma(stdlib) 基准与文档"
```

---

### Task 5: 优化项 3 — compress 中间件（文档 03）

**Files:**
- Modify: `internal/middleware/compress.go`（加 `PERF_DISABLE_COMPRESS` 开关，1 个 if）
- Modify: `internal/bootstrap/container.go`（不动；开关在 CompressMiddleware 内部判断）
- Test: `test/unit/compress/compress_switch_test.go`
- Doc: `docs/perf/03-compress-middleware.md`

**Interfaces:**
- Produces: `CompressMiddleware()` 在 `PERF_DISABLE_COMPRESS=1` 时返回直通 handler（`func(c fiber.Ctx) error { return c.Next() }`），默认行为不变。

- [ ] **Step 1: Write the failing test**

```go
// test/unit/compress/compress_switch_test.go
package compress

import (
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func TestCompressDisabledByEnv(t *testing.T) {
	t.Setenv("PERF_DISABLE_COMPRESS", "1")
	app := fiber.New()
	app.Use(middleware.CompressMiddleware())
	app.Get("/big", func(c fiber.Ctx) error {
		return c.SendString(string(make([]byte, 0, 20*1024)))
	})
	req, _ := fiber.NewClient? // 实现时用 app.Test + 请求头 Accept-Encoding: gzip
	_ = req
	if os.Getenv("PERF_DISABLE_COMPRESS") != "1" {
		t.Fatal("env not set")
	}
	// 断言：响应头无 Content-Encoding: gzip
}

func TestCompressDefaultEnabled(t *testing.T) {
	// 断言：默认（无 env）响应头有 Content-Encoding: gzip（20KB 文本响应 + Accept-Encoding: gzip）
}
```

（实现时用 `app.Test(req)` 发 `Accept-Encoding: gzip` 的 GET，断言 `resp.Header.Get("Content-Encoding")` 两组分别为 `""` 与 `"gzip"`。）

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./test/unit/compress/ -v`
Expected: FAIL（开关不存在，`PERF_DISABLE_COMPRESS=1` 下仍返回 gzip）。

- [ ] **Step 3: Implement 开关**

```go
// internal/middleware/compress.go
func CompressMiddleware() fiber.Handler {
	// PERF_DISABLE_COMPRESS=1 仅用于 perf 基准测试（docs/perf/03），生产不设置。
	if os.Getenv("PERF_DISABLE_COMPRESS") == "1" {
		return func(c fiber.Ctx) error { return c.Next() }
	}
	return compress.New(compress.Config{
		Level: compress.LevelDefault,
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./test/unit/compress/ -v && rtk go test ./test/unit/bootstrap/ -run TestRegisterMiddlewares -v`
Expected: PASS（含既有 bootstrap 中间件链测试不回归）。

- [ ] **Step 5: 实测（主服务实例 + mock 上游三档体积）**

```bash
# 起 mock 上游 + 主服务实例（本地或生产服务器，18080；模型路由指向 mock 上游）
GOMAXPROCS=8 go run ./test/perf/cmd/mockupstream -addr 127.0.0.1:18090 &
GOMAXPROCS=8 PERF_DISABLE_COMPRESS=1 ./aris-proxy-api &   # 关压缩组
# 压完 kill，再以不带 env 启动（开压缩组）
for c in 50 100 200; do
  ./test/perf/run-bench.sh 03-nocompress-c$c  http://127.0.0.1:18080/<代理路由> -c $c -H 'Accept-Encoding: gzip'
  ./test/perf/run-bench.sh 03-compress-c$c    http://127.0.0.1:18080/<代理路由> -c $c -H 'Accept-Encoding: gzip'
done
```

体积三档（2KB/20KB/100KB）各重复一轮；另用 `curl -sI -H 'Accept-Encoding: gzip'` 记录 gzip 前后 `Content-Length` 作为压缩比证据。

- [ ] **Step 6: 撰写文档 03**

数据表列：体积档 × 开/关 → QPS 中位数、p99、响应字节。结论必须分两个维度：
1. CPU 维度：开压缩的 QPS 损耗 %（回环下即净 CPU 成本）
2. 带宽维度：压缩比（x:1）+ 按典型公网带宽定性折算（明确标注非实测）

- [ ] **Step 7: Commit**

```bash
git add internal/middleware/compress.go test/unit/compress/ docs/perf/03-compress-middleware.md docs/perf/raw/
git commit -m "perf: compress 增加 PERF_DISABLE_COMPRESS 测试开关；优化项3 基准与文档"
```

---

### Task 6: 优化项 4 — 落库异步化（文档 04）

**Files:**
- Create: `test/perf/bench/store.go`
- Modify: `test/perf/cmd/harness/main.go`（`-mode=store-sync|store-async -dsn $PERF_BENCH_PG_DSN`）
- Test: `test/perf/bench/store_test.go`
- Doc: `docs/perf/04-async-store.md`

**Interfaces:**
- Consumes: Task 0 的 `PERF_BENCH_PG_DSN` / `PERF_BENCH_API_KEY_ID`
- Produces: `bench.RegisterStore(mux *http.ServeMux, db *gorm.DB, mode string)`，端点 `POST /bench/store`：组装一条真实 `model_call_audits` 行（`api_key_id = $PERF_BENCH_API_KEY_ID`）；`store-sync` 请求内 `Create` 等待返回；`store-async` 提交 Pond 池（`pond.NewPool(8, pond.WithQueueSize(4096))`，与生产同构）立即返回。响应均为 `{"ok":true}`。

- [ ] **Step 1: Write the failing test**

```go
// test/perf/bench/store_test.go
package bench

import (
	"sync/atomic"
	"testing"
	"time"
)

// fakeWriter 记录落库次数，模拟同步等待 vs 异步提交的时序差异。
func TestSyncBlocksUntilWrite(t *testing.T) {
	var written atomic.Int64
	release := make(chan struct{})
	syncWrite := func() { <-release; written.Add(1) }
	done := make(chan struct{})
	go func() { syncWrite(); close(done) }()
	select {
	case <-done:
		t.Fatal("sync write returned before completion")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-done
	if written.Load() != 1 {
		t.Fatalf("written = %d", written.Load())
	}
}

func TestAsyncReturnsBeforeWrite(t *testing.T) {
	var written atomic.Int64
	release := make(chan struct{})
	pool := pond.New(8, 4096)
	defer pool.Stop()
	start := time.Now()
	pool.Submit(func() { <-release; written.Add(1) })
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Fatalf("async submit took %v", elapsed)
	}
	if written.Load() != 0 {
		t.Fatal("async write completed synchronously")
	}
	close(release)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./test/perf/bench/ -run 'TestSyncBlocksUntilWrite|TestAsyncReturnsBeforeWrite' -v`
Expected: FAIL（`pond` 相关实现未接入）。

- [ ] **Step 3: Implement store.go（真实写生产库，行标记测试 Key）**

```go
// test/perf/bench/store.go — 同步 vs 异步落库对比端点。
package bench

import (
	"net/http"
	"time"

	"github.com/alitto/pond"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/gorm"
)

// RegisterStore mode ∈ sync|async；apiKeyID 为测试 Key id，所有写入带此标记供清理。
func RegisterStore(mux *http.ServeMux, db *gorm.DB, mode string, apiKeyID uint) {
	pool := pond.New(8, pond.WithQueueSize(4096))
	row := func() *model.ModelCallAudit {
		return &model.ModelCallAudit{APIKeyID: apiKeyID, ModelID: "perf-bench", TraceID: "perf-bench", CreatedAt: time.Now()}
	}
	mux.HandleFunc("POST /bench/store", func(w http.ResponseWriter, r *http.Request) {
		if mode == "sync" {
			_ = db.Create(row()).Error
		} else {
			r := row()
			pool.Submit(func() { _ = db.Create(r).Error })
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}
```

（`model.ModelCallAudit` 字段以实际结构为准填最小非空集；`pond.New` 用法以 `internal/infrastructure/pool/pool.go` 现状 `pond.NewPool` 为准保持同构。）

- [ ] **Step 4: Run test to verify it passes**

Run: `rtk go test ./test/perf/bench/ -v`
Expected: PASS。

- [ ] **Step 5: 实测 + 落库量核对 + 清理**

```bash
for mode in sync async; do
  GOMAXPROCS=8 go run ./test/perf/cmd/harness -mode=store-$mode -addr 127.0.0.1:18081 \
    -dsn "$PERF_BENCH_PG_DSN" -api-key-id "$PERF_BENCH_API_KEY_ID" &
  SRV=$!; sleep 1
  for c in 50 100 200; do
    ./test/perf/run-bench.sh 04-store-$mode-c$c http://127.0.0.1:18081/bench/store -c $c -T 1 \
      -H 'Content-Type: application/json' -s <(echo '{}')
  done
  kill $SRV
done

# 落库量核对（两组都必须与请求数一致，验证异步不丢数据）
ssh api.lvlvko.top 'docker exec -i <postgres容器> psql -U <user> -d <db> -c \
  "SELECT count(*) FROM model_call_audits WHERE api_key_id='"$PERF_BENCH_API_KEY_ID"';"'
```

清理（Task 10 统一执行也可，此处先记录基线行数）：`DELETE FROM model_call_audits WHERE api_key_id=<id>;`

- [ ] **Step 6: 撰写文档 04**

数据表：模式 × 并发档 → QPS、p50/p99（重点）。结论：异步化对请求侧 QPS/延迟的提升倍数 + 落库完整性证据（行数核对）+ 边界（落库延迟后移，崩溃窗口内的丢弃风险如实说明）。

- [ ] **Step 7: Commit**

```bash
git add test/perf/bench/store.go test/perf/bench/store_test.go docs/perf/04-async-store.md docs/perf/raw/
git commit -m "test(perf): 优化项4 落库异步化基准与文档"
```

---

### Task 7: 优化项 5 — 缓存（文档 05）

**Files:**
- Test: 无新代码（冷热对比零改动）
- Doc: `docs/perf/05-cache.md`

**Interfaces:**
- Consumes: 主服务实例（18080）+ 生产 Redis/Postgres + 真实生产 session（选定 5 个含 20+ 消息的 session id，记入 raw）
- Produces: 仅文档。

- [ ] **Step 1: 选定测试 session 并记录缓存 key 形态**

```bash
ssh api.lvlvko.top 'docker exec -i <postgres容器> psql -U <user> -d <db> -c \
  "SELECT s.id, jsonb_array_length(s.questions) AS msgs FROM sessions s ORDER BY msgs DESC LIMIT 5;"'
```

Expected: 5 个 session id；按 `constant.MessageKeyTemplate` / `SessionMetaKeyTemplate`（`internal/infrastructure/cache/session_detail.go`）推导实际 key 名，记录到 raw 文件。

- [ ] **Step 2: 缓存热态压测**

```bash
# 先 100 次预热（触发 SetSessionMeta/SetMessages），再压测
for i in $(seq 1 100); do curl -s -o /dev/null -H "Authorization: Bearer $PERF_BENCH_API_KEY" \
  "http://127.0.0.1:18080/api/v1/session/<id>/messages"; done
for c in 50 100 200; do
  ./test/perf/run-bench.sh 05-cache-hot-c$c "http://127.0.0.1:18080/api/v1/session/<id>/messages" \
    -c $c -H "Authorization: Bearer $PERF_BENCH_API_KEY"
done
```

（路由以 `internal/router/webapi.go` 中 session 消息列表实际 path 为准。）

- [ ] **Step 3: 缓存冷态压测（DEL 后每请求直查 DB）**

每轮压测前清一次缓存，防止 TTL 内变热：

```bash
ssh api.lvlvko.top 'docker exec -i <redis容器> redis-cli --scan --pattern "session:*"' | \
  ssh api.lvlvko.top 'docker exec -i <redis容器> redis-cli DEL'
# 同参数压测，命名为 05-cache-cold-c$c；必要时在 run-bench.sh 每轮之间插入 DEL
```

Expected: 冷态 QPS 显著低于热态；DB 查询数佐证（`pg_stat_statements` 或 GORM Trace 日志计数）。

- [ ] **Step 4: 附加对比（触发词 AC 自动机，harness 复现）**

`test/perf/bench/trigger.go`：`/bench/trigger/mem`（进程内 AC 自动机匹配 1000 词表）vs `/bench/trigger/db`（每请求 `SELECT` 词表 + 遍历匹配），压测两组。TDD 步骤同前任务（失败测试 → 实现 → 通过 → commit）。词表用 `SELECT id, word FROM triggers` 真实数据或生成 1000 条。

- [ ] **Step 5: 撰写文档 05**

数据表：冷/热 × 并发档 → QPS、p99、DB 查询数；触发词两组单独小表。结论：缓存命中吞吐提升倍数 + 边界（冷启动窗口、TTL、内存 vs Redis 层级差异）。

- [ ] **Step 6: Commit**

```bash
git add test/perf/bench/trigger.go test/perf/bench/trigger_test.go docs/perf/05-cache.md docs/perf/raw/
git commit -m "test(perf): 优化项5 缓存冷热对比基准与文档"
```

---

### Task 8: 优化项 6 — 连接池调优（文档 06，含唯一生产代码改动）

**Files:**
- Modify: `internal/common/constant/database.go`（3 个常量 → 保留常量默认值 + 运行时 env 覆盖）
- Create: `test/unit/dbpool/dbpool_test.go`
- Doc: `docs/perf/06-connection-pool.md`

**Interfaces:**
- Produces: `database.PoolSettings() (maxIdle, maxOpen int, lifetime time.Duration)` 读取 `DB_MAX_IDLE_CONNS` / `DB_MAX_OPEN_CONNS` / `DB_CONN_MAX_LIFETIME`，未设置时返回常量默认值 10/100/5h；`InitDatabase` 改调该函数。默认行为零变化。

- [ ] **Step 1: Write the failing test**

```go
// test/unit/dbpool/dbpool_test.go
package dbpool

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
)

func TestPoolSettingsDefaults(t *testing.T) {
	maxIdle, maxOpen, lifetime := database.PoolSettings()
	if maxIdle != 10 || maxOpen != 100 || lifetime != 5*time.Hour {
		t.Fatalf("defaults changed: %d %d %v", maxIdle, maxOpen, lifetime)
	}
}

func TestPoolSettingsEnvOverride(t *testing.T) {
	t.Setenv("DB_MAX_IDLE_CONNS", "50")
	t.Setenv("DB_MAX_OPEN_CONNS", "200")
	t.Setenv("DB_CONN_MAX_LIFETIME", "2h")
	maxIdle, maxOpen, lifetime := database.PoolSettings()
	if maxIdle != 50 || maxOpen != 200 || lifetime != 2*time.Hour {
		t.Fatalf("override failed: %d %d %v", maxIdle, maxOpen, lifetime)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./test/unit/dbpool/ -v`
Expected: FAIL，`undefined: database.PoolSettings`。

- [ ] **Step 3: Implement PoolSettings（默认值与现状完全一致）**

```go
// internal/infrastructure/database/postgresql.go 新增
// PoolSettings 返回连接池参数：env 覆盖优先，未设置用 constant 默认值（10/100/5h）。
func PoolSettings() (maxIdle, maxOpen int, lifetime time.Duration) {
	maxIdle, maxOpen, lifetime = constant.PostgresMaxIdleConns, constant.PostgresMaxOpenConns, constant.PostgresConnMaxLifetime
	if v, err := strconv.Atoi(os.Getenv("DB_MAX_IDLE_CONNS")); err == nil && v > 0 {
		maxIdle = v
	}
	if v, err := strconv.Atoi(os.Getenv("DB_MAX_OPEN_CONNS")); err == nil && v > 0 {
		maxOpen = v
	}
	if d, err := time.ParseDuration(os.Getenv("DB_CONN_MAX_LIFETIME")); err == nil && d > 0 {
		lifetime = d
	}
	return maxIdle, maxOpen, lifetime
}
```

`InitDatabase` 内替换三行 `Set*` 为 `sqlDB.SetMaxIdleConns(maxIdle)` 等（调用 `PoolSettings()`）。

- [ ] **Step 4: Run test to verify it passes（含全量回归）**

Run: `rtk go test ./test/unit/dbpool/ -v && rtk go test ./test/unit/... ./test/e2e/... -count=1`
Expected: 全部 PASS（默认行为不变被钉死）。

- [ ] **Step 5: 实测（扫参）**

主服务实例 + mock 上游，DB 密集端点（session list）。每组重启实例注入 env：

```bash
for OPEN in 10 25 50 100 200; do
  for IDLE in 10 50; do
    DB_MAX_OPEN_CONNS=$OPEN DB_MAX_IDLE_CONNS=$IDLE GOMAXPROCS=8 ./aris-proxy-api &
    SRV=$!; sleep 2
    for c in 100 200; do
      ./test/perf/run-bench.sh 06-pool-o$OPEN-i$IDLE-c$c http://127.0.0.1:18080/api/v1/session/list \
        -c $c -H "Authorization: Bearer $PERF_BENCH_API_KEY"
    done
    kill $SRV
  done
done
```

- [ ] **Step 6: 撰写文档 06**

数据表：`MaxOpen × MaxIdle × 并发` → QPS、p99、错误数；画 QPS-连接数曲线（ASCII/markdown 表即可）；结论给出拐点与推荐值（当前 100/10 是否最优）+ 边界（机器核数、DB max_connections 约束）。

- [ ] **Step 7: Commit**

```bash
git add internal/common/constant/database.go internal/infrastructure/database/postgresql.go \
        test/unit/dbpool/ docs/perf/06-connection-pool.md docs/perf/raw/
git commit -m "perf(db): 连接池参数支持 env 覆盖（默认值不变）；优化项6 扫参与文档"
```

---

### Task 9: 优化项 7 — 复合索引（文档 07）

**Files:**
- Create: `test/perf/bench/query.go`、`test/perf/sql/queries.sql`
- Test: `test/perf/bench/query_test.go`
- Doc: `docs/perf/07-compound-index.md`

**Interfaces:**
- Produces: `bench.RegisterQuery(mux *http.ServeMux, db *gorm.DB, useIndex bool)`，端点 `GET /bench/query/{name}`（name ∈ `audit-by-key`/`audit-by-model`/`session-keyword`），`useIndex=false` 时每连接先执行 `SET enable_indexscan=off; SET enable_bitmapscan=off`（会话级，只读）。`test/perf/sql/queries.sql` 存 3 条代表 SQL 与对应 `EXPLAIN (ANALYZE, BUFFERS)` 命令。

- [ ] **Step 1: 写 3 条代表 SQL（只读，从现有 repository 提取）**

```sql
-- test/perf/sql/queries.sql
-- audit-by-key: idx_mca_apikey_created（api_key_id + created_at desc）
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM model_call_audits WHERE api_key_id = $1 AND created_at >= now() - interval '7 days'
ORDER BY created_at DESC LIMIT 50;

-- audit-by-model: idx_mca_model_created（model_id + created_at desc）
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM model_call_audits WHERE model_id = $1 AND created_at >= now() - interval '7 days'
ORDER BY created_at DESC LIMIT 50;

-- session-keyword: constant.SessionKeywordFilterSQL 形态
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM sessions WHERE created_at >= now() - interval '30 days'
AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid)
  WHERE arr.mid::bigint IN (SELECT id FROM messages WHERE messages.message::text ILIKE '%error%'))
ORDER BY created_at DESC LIMIT 20;
```

- [ ] **Step 2: 生产 DB 只读 EXPLAIN 对比（索引开 vs 关）**

```bash
ssh api.lvlvko.top 'docker exec -i <postgres容器> psql -U <user> -d <db>' < test/perf/sql/queries.sql > docs/perf/raw/07-explain-indexed.txt
# 对照组：每条前加 SET enable_indexscan=off; SET enable_bitmapscan=off;（同文件复制一份改名 queries-noindex.sql）
ssh api.lvlvko.top 'docker exec -i <postgres容器> psql -U <user> -d <db>' < test/perf/sql/queries-noindex.sql > docs/perf/raw/07-explain-noindex.txt
```

Expected: 无索引组执行时间高一个数量级以上、`Heap Blocks`/`Rows Removed` 明显更高；记录两组 planning/execution time。

- [ ] **Step 3: Write the failing test（query.go 端点）**

```go
// test/perf/bench/query_test.go
package bench

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryEndpointRejectsUnknownName(t *testing.T) {
	mux := http.NewServeMux()
	RegisterQuery(mux, nil, true) // db 为 nil 时返回 500 前先做 name 校验
	req := httptest.NewRequest(http.MethodGet, "/bench/query/unknown", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}
```

- [ ] **Step 4: Implement query.go 并跑通测试**

```go
// test/perf/bench/query.go — 索引开/关并发查询端点（只读）。
package bench

import (
	"net/http"

	"gorm.io/gorm"
)

var querySQL = map[string]string{
	"audit-by-key":    `SELECT * FROM model_call_audits WHERE api_key_id = $1 AND created_at >= now() - interval '7 days' ORDER BY created_at DESC LIMIT 50`,
	"audit-by-model":  `SELECT * FROM model_call_audits WHERE model_id = $1 AND created_at >= now() - interval '7 days' ORDER BY created_at DESC LIMIT 50`,
	"session-keyword": `SELECT * FROM sessions WHERE created_at >= now() - interval '30 days' AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(sessions.questions::jsonb) AS arr(mid) WHERE arr.mid::bigint IN (SELECT id FROM messages WHERE messages.message::text ILIKE '%error%')) ORDER BY created_at DESC LIMIT 20`,
}

// RegisterQuery useIndex=false 时会话级禁用索引扫描（只读影响，随连接归还不影响他人——
// 注意：gorm 连接池复用连接，故改为每请求在事务内 SET LOCAL，事务结束自动还原）。
func RegisterQuery(mux *http.ServeMux, db *gorm.DB, useIndex bool) {
	mux.HandleFunc("GET /bench/query/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		sqlText, ok := querySQL[name]
		if !ok {
			http.Error(w, "unknown query", http.StatusBadRequest)
			return
		}
		if db == nil {
			http.Error(w, "db not configured", http.StatusInternalServerError)
			return
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			if !useIndex {
				if err := tx.Exec(`SET LOCAL enable_indexscan = off`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`SET LOCAL enable_bitmapscan = off`).Error; err != nil {
					return err
				}
			}
			return tx.Raw(sqlText, "perf-bench").Scan(&[]map[string]any{}).Error
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}
```

Run: `rtk go test ./test/perf/bench/ -run TestQueryEndpointRejectsUnknownName -v`
Expected: PASS（先失败后通过按 TDD 循环执行）。

- [ ] **Step 5: 并发 QPS 实测（有/无索引两组）**

```bash
for mode in indexed noindex; do
  GOMAXPROCS=8 go run ./test/perf/cmd/harness -mode=query-$mode -addr 127.0.0.1:18081 \
    -dsn "$PERF_BENCH_PG_DSN" &
  SRV=$!; sleep 1
  for q in audit-by-key audit-by-model session-keyword; do
    for c in 50 100 200; do
      ./test/perf/run-bench.sh 07-$q-$mode-c$c "http://127.0.0.1:18081/bench/query/$q" -c $c
    done
  done
  kill $SRV
done
```

- [ ] **Step 6: 撰写文档 07**

数据表：查询 × 索引开/关 → 单查询延迟（EXPLAIN execution time）、并发 QPS 中位数、扫描行数/Buffers。结论：索引带来的 QPS/延迟提升（通常为数量级）+ 引用既有 SQL 形态优化案例（`SessionKeywordFilterSQL` 27.4s→22ms，见 memory `perf/session-list-keyword-trigram-2026-08-02`）+ 边界（trigram 索引对高频词的陷阱、写放大）。

- [ ] **Step 7: Commit**

```bash
git add test/perf/bench/query.go test/perf/bench/query_test.go test/perf/sql/ \
        docs/perf/07-compound-index.md docs/perf/raw/
git commit -m "test(perf): 优化项7 复合索引只读对比基准与文档"
```

---

### Task 10: 生产数据清理与收尾验收

**Files:**
- Modify: 无代码（清理 + 核对）
- Test: 全量回归

**Interfaces:**
- Consumes: Task 0 的 `PERF_BENCH_API_KEY_ID`、Task 6/7 的写入量基线
- Produces: 清理证据（SQL 输出贴入 `docs/perf/raw/cleanup.txt`）；最终验收核对记录。

- [ ] **Step 1: 清理压测写入的生产数据**

```bash
ssh api.lvlvko.top 'docker exec -i <postgres容器> psql -U <user> -d <db> -c \
  "DELETE FROM model_call_audits WHERE api_key_id='"$PERF_BENCH_API_KEY_ID"';"' | tee docs/perf/raw/cleanup.txt
ssh api.lvlvko.top 'docker exec -i <postgres容器> psql -U <user> -d <db> -c \
  "DELETE FROM proxy_api_keys WHERE name = '"'"'perf-bench-2026-10'"'"';"' | tee -a docs/perf/raw/cleanup.txt
```

Expected: 两行 `DELETE N`，N 与压测写入量一致；写入 `cleanup.txt` 作为证据。

- [ ] **Step 2: 全量回归（唯一生产代码改动不破坏任何既有行为）**

Run: `rtk go build ./... && rtk go test ./test/... -count=1 && rtk lint`
Expected: 全绿。

- [ ] **Step 3: 验收核对（对照 spec §7）**

```markdown
- [ ] 7 份文档齐全（01…07），结构统一
- [ ] 每份有复现命令 + raw 存档
- [ ] 每份有明确的 QPS 提升数字（或声明收益不在 QPS 维度：03 带宽维度、07 延迟维度）
- [ ] 生产数据清理证据（cleanup.txt）
- [ ] 连接池 env 覆盖附回归测试、默认值钉死
```

- [ ] **Step 4: 沉淀 memory 并提交收尾 commit**

```bash
git add docs/perf/ docs/superpowers/
git commit -m "docs(perf): 7 项优化 QPS 基准测试收尾（清理证据 + 验收核对）"
```

Serena memory 写入实测结论（各优化项的真实提升倍数、踩坑、可复现要点），供后续性能工作引用。

---

## Self-Review 记录

1. **Spec 覆盖**：§5.1→Task 3、§5.2→Task 4、§5.3→Task 5、§5.4→Task 6、§5.5→Task 7、§5.6→Task 8、§5.7→Task 9；§3 环境拓扑→Task 0；§6 风险→Global Constraints + Task 10 清理；§7 交付物→Task 10 Step 3 验收。无缺口。
2. **占位符**：Task 4 Step 3 中 huma 输入 struct 标注"实现时填充"处已给出字段定义位置与共享约束（`echoHumaInput` 单一定义），Step 3 的实现注意明确列出；无 TBD。
3. **类型一致性**：`RegisterJSON(mux, lib)`、`RegisterStore(mux, db, mode, apiKeyID)`、`RegisterQuery(mux, db, useIndex)`、`PoolSettings()` 在各任务 Interfaces 与代码块中签名一致；`PERF_BENCH_API_KEY_ID`/`PERF_BENCH_PG_DSN` 命名贯穿 Task 0/6/9/10。
