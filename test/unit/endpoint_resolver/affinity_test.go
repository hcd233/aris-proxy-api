package endpoint_resolver

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

// fakeAffinity 内存亲和映射。
type fakeAffinity struct {
	byKey map[string]uint
}

func (f *fakeAffinity) Get(_ context.Context, _ uint, alias, key string) (uint, bool) {
	id, ok := f.byKey[alias+"|"+key]
	return id, ok
}

func TestResolveCandidatesAffinityFirst(t *testing.T) {
	t.Parallel()
	r := newTestResolver(t,
		cand{alias: "gpt-4", endpoint: "ep-a", priority: 0, weight: 1},
		cand{alias: "gpt-4", endpoint: "ep-b", priority: 0, weight: 1},
	)

	// 先无亲和跑一次拿到两个候选的端点 ID 映射
	base, err := r.ResolveCandidates(context.Background(), 1, "gpt-4", nil)
	if err != nil {
		t.Fatalf("ResolveCandidates: %v", err)
	}
	idByName := map[string]uint{}
	for _, c := range base {
		idByName[c.Endpoint.Name()] = c.Endpoint.AggregateID()
	}

	aff := &fakeAffinity{byKey: map[string]uint{"gpt-4|k1": idByName["ep-b"]}}
	r2 := newTestResolverWithAffinity(t, aff,
		cand{alias: "gpt-4", endpoint: "ep-a", priority: 0, weight: 1},
		cand{alias: "gpt-4", endpoint: "ep-b", priority: 0, weight: 1},
	)

	got, err := r2.ResolveCandidatesWithAffinity(context.Background(), 1, "gpt-4", "k1", nil)
	if err != nil {
		t.Fatalf("ResolveCandidatesWithAffinity: %v", err)
	}
	if got[0].Endpoint.Name() != "ep-b" {
		t.Fatalf("affinity first = %s, want ep-b", got[0].Endpoint.Name())
	}

	// 亲和端点被 matcher 过滤时按序返回，不置顶
	got, err = r2.ResolveCandidatesWithAffinity(context.Background(), 1, "gpt-4", "k1", func(ep *aggregate.Endpoint) bool {
		return ep.Name() != "ep-b"
	})
	if err != nil {
		t.Fatalf("filtered: %v", err)
	}
	if got[0].Endpoint.Name() == "ep-b" {
		t.Fatal("被过滤的亲和端点不应置顶")
	}

	// 空 key 等价 ResolveCandidates（不查亲和）
	got, err = r2.ResolveCandidatesWithAffinity(context.Background(), 1, "gpt-4", "", nil)
	if err != nil {
		t.Fatalf("empty key: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	// 亲和指向不存在的端点 ID 时忽略亲和
	affMissing := &fakeAffinity{byKey: map[string]uint{"gpt-4|k1": 99999}}
	r3 := newTestResolverWithAffinity(t, affMissing,
		cand{alias: "gpt-4", endpoint: "ep-a", priority: 0, weight: 1},
		cand{alias: "gpt-4", endpoint: "ep-b", priority: 0, weight: 1},
	)
	got, err = r3.ResolveCandidatesWithAffinity(context.Background(), 1, "gpt-4", "k1", nil)
	if err != nil {
		t.Fatalf("missing pin: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}
