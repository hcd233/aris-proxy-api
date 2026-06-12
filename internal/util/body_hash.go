package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/samber/lo"
)

func HashJSONBodyExcludingTopLevelModel(body []byte) string {
	canonical := canonicalizeJSONBody(body, true)
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func canonicalizeJSONBody(raw []byte, skipTopLevelModel bool) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	if trimmed[0] == '{' {
		var obj map[string]sonic.NoCopyRawMessage
		if err := sonic.Unmarshal(trimmed, &obj); err == nil {
			return canonicalizeJSONObject(obj, skipTopLevelModel)
		}
	}
	if trimmed[0] == '[' {
		var arr []sonic.NoCopyRawMessage
		if err := sonic.Unmarshal(trimmed, &arr); err == nil {
			return canonicalizeJSONArray(arr)
		}
	}
	return trimmed
}

func canonicalizeJSONObject(obj map[string]sonic.NoCopyRawMessage, skipTopLevelModel bool) []byte {
	keys := lo.Filter(lo.Keys(obj), func(key string, _ int) bool {
		return !skipTopLevelModel || key != constant.FieldNameModel
	})
	sort.Strings(keys)

	out := make([]byte, 0, len(obj)*16)
	out = append(out, '{')
	for index, key := range keys {
		if index > 0 {
			out = append(out, ',')
		}
		keyBytes, err := sonic.Marshal(key)
		if err != nil {
			return nil
		}
		out = append(out, keyBytes...)
		out = append(out, ':')
		out = append(out, canonicalizeJSONBody(obj[key], false)...)
	}
	out = append(out, '}')
	return out
}

func canonicalizeJSONArray(arr []sonic.NoCopyRawMessage) []byte {
	out := make([]byte, 0, len(arr)*16)
	out = append(out, '[')
	for index, item := range arr {
		if index > 0 {
			out = append(out, ',')
		}
		out = append(out, canonicalizeJSONBody(item, false)...)
	}
	out = append(out, ']')
	return out
}
