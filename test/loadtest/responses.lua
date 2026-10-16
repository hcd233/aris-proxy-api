-- responses.lua 压测 POST /api/openai/v1/responses（mock 模型随机字符流）
--
-- 执行：
--   export TARGET_URL=https://api.lvlvko.top
--   export LT_KEYS="$K1,$K2,..."（多 Key 轮换，绕过 100 req/min per Key 限流）
--   export MODEL=mock-response
--   wrk -t4 -c100 -d5m --latency -s test/loadtest/responses.lua "$TARGET_URL/api/openai/v1/responses"

local model   = os.getenv("MODEL") or "mock-response"
local max_tok = tonumber(os.getenv("MAX_TOKENS") or "0") or 0

-- Key 池：LT_KEYS 优先（逗号分隔），回退单 Key API_KEY
local keys = {}
local keys_raw = os.getenv("LT_KEYS")
if keys_raw and #keys_raw > 0 then
  for k in string.gmatch(keys_raw, "([^,]+)") do keys[#keys + 1] = k end
else
  keys[1] = os.getenv("API_KEY") or ""
end

-- 请求体固定（__LOADTEST__ 前缀保证 checksum 不与任何真实消息相同，防止清理时误删真实数据）
local body = string.format(
  '{"model":"%s","stream":true,"max_output_tokens":%d,"input":"__LOADTEST__ perf probe %s"}',
  model, max_tok, tostring(os.time()))

-- 每请求轮换 Key（wrk 每线程独立 Lua 状态，故各线程独立轮换）
local idx = 0
function request()
  idx = idx + 1
  local headers = {
    ["Content-Type"]  = "application/json",
    ["Authorization"] = keys[(idx % #keys) + 1],
    ["User-Agent"]    = "aris-loadtest/1.0",
    ["Accept"]        = "text/event-stream",
  }
  return wrk.format("POST", nil, headers, body)
end

local status_count = {}
local body_bytes = 0

function response(status, headers, body)
  status_count[status] = (status_count[status] or 0) + 1
  body_bytes = body_bytes + (body and #body or 0)
end

function done(summary, latency, requests)
  print("\n===== responses loadtest summary =====")
  print(string.format("duration(s)     : %.2f", summary.duration / 1e6))
  print(string.format("requests        : %d", summary.requests))
  print(string.format("errors          : %d", summary.errors.connect + summary.errors.read + summary.errors.write + summary.errors.status))
  print(string.format("latency avg(ms) : %.2f", latency.mean / 1000))
  print(string.format("latency p50(ms) : %.2f", latency:percentile(50) / 1000))
  print(string.format("latency p90(ms) : %.2f", latency:percentile(90) / 1000))
  print(string.format("latency p99(ms) : %.2f", latency:percentile(99) / 1000))
  print(string.format("body bytes      : %d", body_bytes))
  print("status codes:")
  for code, n in pairs(status_count) do
    print(string.format("  %s : %d", tostring(code), n))
  end
  print("core metric : 并发 SSE 长连接数 = wrk -c 值，逐档加压找稳定拐点")
  print("ttft/stream : 从审计表 model_call_audits.first_token_latency_ms / stream_duration_ms 统计")
  print("========================================")
end
