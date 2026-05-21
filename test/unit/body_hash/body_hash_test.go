package bodyhash

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/util"
)

func TestHashJSONBodyExcludingTopLevelModel_IgnoresModelAndObjectKeyOrder(t *testing.T) {
	left := []byte(`{"model":"exposed-model","stream":true,"messages":[{"role":"user","content":"hi"}],"metadata":{"team":"core","env":"prod"}}`)
	right := []byte(`{"metadata":{"env":"prod","team":"core"},"messages":[{"content":"hi","role":"user"}],"stream":true,"model":"upstream-model"}`)

	leftHash := util.HashJSONBodyExcludingTopLevelModel(left)
	rightHash := util.HashJSONBodyExcludingTopLevelModel(right)

	if leftHash == "" {
		t.Fatal("expected non-empty hash")
	}
	if leftHash != rightHash {
		t.Fatalf("hash should ignore only top-level model and object key order, got %s and %s", leftHash, rightHash)
	}
}

func TestHashJSONBodyExcludingTopLevelModel_DetectsNonModelDifference(t *testing.T) {
	left := []byte(`{"model":"exposed-model","messages":[{"role":"user","content":"hi"}],"max_tokens":1024}`)
	right := []byte(`{"model":"upstream-model","messages":[{"role":"user","content":"hi"}],"max_tokens":2048}`)

	if util.HashJSONBodyExcludingTopLevelModel(left) == util.HashJSONBodyExcludingTopLevelModel(right) {
		t.Fatal("hash should change when a non-model field changes")
	}
}

func TestHashJSONBodyExcludingTopLevelModel_KeepsNestedModelFields(t *testing.T) {
	left := []byte(`{"model":"exposed-model","metadata":{"model":"nested-a"}}`)
	right := []byte(`{"model":"upstream-model","metadata":{"model":"nested-b"}}`)

	if util.HashJSONBodyExcludingTopLevelModel(left) == util.HashJSONBodyExcludingTopLevelModel(right) {
		t.Fatal("hash should include nested model fields")
	}
}
