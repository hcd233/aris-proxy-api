#!/usr/bin/env bash
# aris-proxy-api trace hook for Codex CLI
# fail-open: never blocks or alters the agent; reports synchronously before exit.
set -u

TRACE_URL="${TRACE_URL:-http://localhost:8080/api/v1/trace/event}"
API_KEY="${API_KEY:-}"

# Local log sink with 7-day rolling (daily files).
LOG_DIR="${LOG_DIR:-$HOME/.aris/trace/logs}"
LOG_FILE="$LOG_DIR/trace-$(date +%Y-%m-%d).log"
TRACE_ROOT="${TRACE_ROOT:-$HOME/.aris/trace}"

payload="$(cat)"

event_name="$(printf '%s' "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null)"

# Stop expects JSON on stdout; emit empty object, do NOT inject context elsewhere.
if [ "$event_name" = "Stop" ]; then
  printf '{}'
fi

# Best-effort local log (7-day rolling).
(
  mkdir -p "$LOG_DIR" 2>/dev/null
  printf '%s' "$payload" | jq -c --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    '. + {_trace_local_ts: $ts}' >> "$LOG_FILE" 2>/dev/null
  # Roll: delete local trace logs older than 7 days.
  find "$LOG_DIR" -name 'trace-*.log' -mtime +7 -delete 2>/dev/null
) >/dev/null 2>&1 &

# Report synchronously. Background curl can be killed when Codex reclaims the hook process.
if [ -n "$API_KEY" ]; then
  printf '%s' "$payload" | curl -sS --connect-timeout 2 --max-time 5 -X POST "$TRACE_URL" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d @- >/dev/null 2>&1 || true
fi

exit 0
