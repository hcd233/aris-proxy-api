package blocked_matcher_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/blocked"
)

func TestACmatcher_SingleWord(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{1: "hello"})
	ids := m.Match("say hello world")
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("expected [1], got %v", ids)
	}
}

func TestACmatcher_NoMatch(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{1: "forbidden"})
	ids := m.Match("say hello world")
	if len(ids) != 0 {
		t.Fatalf("expected [], got %v", ids)
	}
}

func TestACmatcher_MultipleWords(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{
		1: "bad",
		2: "evil",
		3: "wicked",
	})
	ids := m.Match("this is bad and evil")
	found1, found2 := false, false
	for _, id := range ids {
		if id == 1 {
			found1 = true
		}
		if id == 2 {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatalf("expected to find ids 1 and 2, got %v", ids)
	}
}

func TestACmatcher_CaseInsensitive(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{1: "Hello"})
	ids := m.Match("say HELLO world")
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("expected [1] (case-insensitive), got %v", ids)
	}
}

func TestACmatcher_CaseInsensitiveMixed(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{1: "badword"})
	for _, text := range []string{"badword", "BADWORD", "BadWord", "bAdWoRd"} {
		ids := m.Match(text)
		if len(ids) != 1 || ids[0] != 1 {
			t.Fatalf("expected [1] for %q, got %v", text, ids)
		}
	}
}

func TestACmatcher_EmptyInput(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{1: "hello"})
	ids := m.Match("")
	if len(ids) != 0 {
		t.Fatalf("expected [], got %v", ids)
	}
}

func TestACmatcher_EmptyWords(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{})
	ids := m.Match("anything")
	if len(ids) != 0 {
		t.Fatalf("expected [], got %v", ids)
	}
}

func TestACmatcher_OverlappingWords(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{
		1: "abc",
		2: "bc",
		3: "c",
	})
	ids := m.Match("abc")
	found := make(map[uint]bool)
	for _, id := range ids {
		found[id] = true
	}
	if !found[1] || !found[2] || !found[3] {
		t.Fatalf("expected all 3 matches for overlapping words, got %v", ids)
	}
}

func TestACmatcher_SubstringMatch(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{1: "bad"})
	ids := m.Match("this is badness")
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("expected [1] for substring match, got %v", ids)
	}
}

func TestACmatcher_UnicodeText(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{1: "敏感"})
	ids := m.Match("这是一段敏感内容")
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("expected [1] for unicode match, got %v", ids)
	}
}

func TestACmatcher_DuplicateDedup(t *testing.T) {
	t.Parallel()
	m := blocked.NewACmatcher(map[uint]string{1: "aa"})
	ids := m.Match("aaa")
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("expected deduped [1], got %v", ids)
	}
}
