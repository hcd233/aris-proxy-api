#!/usr/bin/env bash
# aris-proxy-api trace hook for Codex CLI
# fail-open: never blocks or alters the agent; reports best-effort in background.
set -u

TRACE_URL="${TRACE_URL:-http://localhost:8080/api/v1/trace/event}"
API_KEY="${API_KEY:-}"

payload="$(cat)"

event_name="$(printf '%s' "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null)"

# Stop expects JSON on stdout; emit empty object, do NOT inject context elsewhere.
if [ "$event_name" = "Stop" ]; then
  printf '{}'
fi

# Best-effort background report; never block the agent turn.
if [ -n "$API_KEY" ]; then
  printf '%s' "$payload" | curl -sS -X POST "$TRACE_URL" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d @- >/dev/null 2>&1 &
fi

exit 0
