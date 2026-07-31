package trace

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	traceclient "github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestRolloutDedupKey_SessionMetaUsesStableID(t *testing.T) {
	t.Parallel()
	childID := "019f6f10-7524-7891-96a2-fe7aa659430c"
	raw := []byte(`{"type":"session_meta","payload":{"id":"` + childID + `","source":"vscode"}}`)
	meta := traceclient.TranscriptMeta{RecordType: constant.TraceRolloutTypeSessionMeta, SessionID: childID}

	key1 := traceclient.RolloutDedupKey("s1", meta, 1, raw)
	key2 := traceclient.RolloutDedupKey("s1", meta, 205, raw) // 压缩重写后行号变化
	if key1 != key2 {
		t.Fatalf("session_meta dedup key must be line-independent: %q vs %q", key1, key2)
	}
	want := "rollout:s1:session_meta:" + childID
	if key1 != want {
		t.Fatalf("expected %q, got %q", want, key1)
	}
}

func TestRolloutDedupKey_OtherRecordsKeepLineAndHash(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"type":"response_item","payload":{"type":"function_call"}}`)
	meta := traceclient.TranscriptMeta{RecordType: constant.TraceRolloutTypeResponseItem, Event: "function_call"}
	digest := sha256.Sum256(raw)
	want := "rollout:s1:12:" + hex.EncodeToString(digest[:])
	got := traceclient.RolloutDedupKey("s1", meta, 12, raw)
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
