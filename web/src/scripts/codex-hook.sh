#!/usr/bin/env bash
# aris-proxy-api trace hook for Codex CLI
# fail-open: never blocks or alters the agent; reports best-effort in background.
set -u

TRACE_URL="${TRACE_URL:-http://localhost:8080/api/v1/trace/event}"
API_KEY="${API_KEY:-}"

# Local log sink with 7-day rolling (daily files).
LOG_DIR="${LOG_DIR:-$HOME/.aris/trace/logs}"
LOG_FILE="$LOG_DIR/trace-$(date +%Y-%m-%d).log"

payload="$(cat)"

event_name="$(printf '%s' "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null)"

# Stop expects JSON on stdout; emit empty object, do NOT inject context elsewhere.
if [ "$event_name" = "Stop" ]; then
  printf '{}'
fi

# Best-effort local log (7-day rolling). Never block the agent turn.
(
  mkdir -p "$LOG_DIR" 2>/dev/null
  printf '%s' "$payload" | jq -c --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    '. + {_trace_local_ts: $ts}' >> "$LOG_FILE" 2>/dev/null
  # Roll: delete local trace logs older than 7 days.
  find "$LOG_DIR" -name 'trace-*.log' -mtime +7 -delete 2>/dev/null
) >/dev/null 2>&1 &

# Best-effort background report; never block the agent turn.
if [ -n "$API_KEY" ]; then
  printf '%s' "$payload" | curl -sS -X POST "$TRACE_URL" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d @- >/dev/null 2>&1 &
fi

exit 0
