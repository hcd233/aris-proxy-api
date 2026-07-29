package domain_llmproxy

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

func baseCaps() []enum.InputModality {
	return []enum.InputModality{enum.InputModalityText}
}

// 合法集合：text / text+image 均通过
func TestCreateModel_Capabilities_Valid(t *testing.T) {
	t.Parallel()
	for _, caps := range [][]enum.InputModality{
		baseCaps(),
		{enum.InputModalityText, enum.InputModalityImage},
	} {
		m, err := aggregate.CreateModel(1, vo.EndpointAlias("a"), "m", 1, true, 128000, 64000, caps)
		if err != nil {
			t.Fatalf("valid capabilities %v should pass: %v", caps, err)
		}
		if got := m.Capabilities(); len(got) != len(caps) {
			t.Fatalf("capabilities round trip mismatch: got %v want %v", got, caps)
		}
	}
}

// 非法集合：空 / 不含 text / 未知成员 均报错
func TestCreateModel_Capabilities_Invalid(t *testing.T) {
	t.Parallel()
	for _, caps := range [][]enum.InputModality{
		{},
		{enum.InputModalityImage},
		{enum.InputModalityText, "blob"},
	} {
		if _, err := aggregate.CreateModel(1, vo.EndpointAlias("a"), "m", 1, true, 128000, 64000, caps); err == nil {
			t.Fatalf("invalid capabilities %v should fail", caps)
		}
	}
}

// Update：nil 不变更，合法值整体替换，非法值报错
func TestModelUpdate_Capabilities(t *testing.T) {
	t.Parallel()
	m, err := aggregate.CreateModel(1, vo.EndpointAlias("a"), "m", 1, true, 128000, 64000, baseCaps())
	if err != nil {
		t.Fatalf("CreateModel failed: %v", err)
	}
	if uerr := m.Update(nil, nil, nil, nil, nil, nil, nil); uerr != nil {
		t.Fatalf("nil capabilities update should be no-op: %v", uerr)
	}
	if got := m.Capabilities(); len(got) != 1 || got[0] != enum.InputModalityText {
		t.Fatalf("capabilities should stay text-only, got %v", got)
	}
	next := []enum.InputModality{enum.InputModalityText, enum.InputModalityImage}
	if uerr := m.Update(nil, nil, nil, nil, nil, nil, &next); uerr != nil {
		t.Fatalf("valid capabilities update failed: %v", uerr)
	}
	if got := m.Capabilities(); len(got) != 2 {
		t.Fatalf("capabilities should be replaced, got %v", got)
	}
	bad := []enum.InputModality{enum.InputModalityImage}
	if uerr := m.Update(nil, nil, nil, nil, nil, nil, &bad); uerr == nil {
		t.Fatal("capabilities without text must be rejected")
	}
}
