// Package modelsdev models.dev 公开模型规格客户端的单元测试
package modelsdev

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/modelsdev"
)

// newTestClient 用固定文档启动客户端
func newTestClient(t *testing.T, doc string) *modelsdev.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(doc))
	}))
	t.Cleanup(srv.Close)
	c := modelsdev.NewClient(srv.Client(), nil)
	c.Source = srv.URL
	return c
}

// docNested 真实形态：provider → models → modelId（含官方与中转、string/number 价、分档）
const docNested = `{
  "nano-gpt": {"models": {"gemini-2.5-pro": {"cost": {"input": 1.25, "output": 10, "cache_read": "0.125"}}}},
  "google": {"models": {"gemini-2.5-pro": {"cost": {"input": 1.25, "output": 10, "cache_read": 0.125, "tiers": [
      {"input": 2.5, "output": 15, "cache_read": 0.25, "tier": {"type": "context", "size": 200000}}]}}}},
  "abacus": {"models": {"gemini-2.5-pro": {"cost": {"input": 9, "output": 9}}, "other": {"cost": {"input": 1}}}}
}`

func TestDescribeNestedStructureOfficialProviderWins(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, docNested)
	spec, ok, err := c.Describe(t.Context(), "gemini-2.5-pro")
	if err != nil || !ok {
		t.Fatalf("describe: ok=%v err=%v", ok, err)
	}
	q := spec.Quote
	// 官方 google 命中（nano-gpt/abacus 同名被跳过），分档 0 + 200000
	if len(q.Tiers) != 2 {
		t.Fatalf("tiers = %+v", q.Tiers)
	}
	if q.Tiers[0].ContextMin != 0 || q.Tiers[0].Input != 1.25 || q.Tiers[0].CacheRead != 0.125 {
		t.Fatalf("tier0 = %+v", q.Tiers[0])
	}
	if q.Tiers[1].ContextMin != 200000 || q.Tiers[1].Input != 2.5 || q.Tiers[1].Output != 15 {
		t.Fatalf("tier1 = %+v", q.Tiers[1])
	}
}

func TestDescribeFallbackToNonOfficial(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, docNested)
	spec, ok, err := c.Describe(t.Context(), "other")
	if err != nil || !ok {
		t.Fatalf("describe: ok=%v err=%v", ok, err)
	}
	if len(spec.Quote.Tiers) != 1 || spec.Quote.Tiers[0].Input != 1 {
		t.Fatalf("tiers = %+v", spec.Quote.Tiers)
	}
	if _, ok, _ := c.Describe(t.Context(), "missing"); ok {
		t.Fatal("unknown model must miss")
	}
}

func TestDescribeTiersSortedAndDedup(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, `{"p": {"models": {"m": {"cost": {"input": 2.5, "tiers": [
		{"input": 6.25, "tier": {"type": "context", "size": 128000}},
		{"input": 5, "tier": {"type": "context", "size": 32000}},
		{"input": 3, "tier": {"type": "context", "size": 0}}]}}}}}`)
	spec, ok, err := c.Describe(t.Context(), "m")
	if err != nil || !ok {
		t.Fatalf("describe: ok=%v err=%v", ok, err)
	}
	q := spec.Quote
	// size=0 档覆盖平铺价；升序去重后 0 / 32000 / 128000
	if len(q.Tiers) != 3 {
		t.Fatalf("tiers = %+v", q.Tiers)
	}
	if q.Tiers[0].ContextMin != 0 || q.Tiers[0].Input != 3 {
		t.Fatalf("tier0 = %+v", q.Tiers[0])
	}
	if q.Tiers[1].ContextMin != 32000 || q.Tiers[2].ContextMin != 128000 {
		t.Fatalf("order = %+v", q.Tiers)
	}
}

func TestDescribeFetchErrorSurfaces(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	c := modelsdev.NewClient(srv.Client(), nil)
	c.Source = srv.URL
	if _, ok, err := c.Describe(t.Context(), "x"); ok || err == nil {
		t.Fatalf("fetch failure must return err and ok=false, got ok=%v err=%v", ok, err)
	}
}

func TestDescribeLimitAndModalities(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, `{"anthropic":{"models":{"claude-sonnet-4-5":{
		"limit":{"context":200000,"output":64000},
		"modalities":{"input":["text","image","pdf","hologram"],"output":["text"]},
		"cost":{"input":1,"output":5}}}}}`)
	spec, ok, err := c.Describe(t.Context(), "claude-sonnet-4-5")
	if err != nil || !ok {
		t.Fatalf("describe: ok=%v err=%v", ok, err)
	}
	if spec.ContextLength != 200000 || spec.MaxOutputTokens != 64000 {
		t.Fatalf("limit = %+v", spec)
	}
	// 未知模态 hologram 静默丢弃；输出按枚举序 text/image/pdf
	want := []enum.InputModality{enum.InputModalityText, enum.InputModalityImage, enum.InputModalityPDF}
	if !slices.Equal(spec.InputModalities, want) {
		t.Fatalf("modalities = %v", spec.InputModalities)
	}
	if spec.Quote.Tiers[0].Input != 1 {
		t.Fatalf("quote lost: %+v", spec.Quote)
	}
}

func TestDescribeMissingLimitAndModalities(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, `{"openai":{"models":{"m":{"cost":{"input":3}}}}}`)
	spec, ok, err := c.Describe(t.Context(), "m")
	if err != nil || !ok {
		t.Fatalf("describe: ok=%v err=%v", ok, err)
	}
	if spec.ContextLength != 0 || spec.MaxOutputTokens != 0 || len(spec.InputModalities) != 0 {
		t.Fatalf("missing fields must be zero: %+v", spec)
	}
}
