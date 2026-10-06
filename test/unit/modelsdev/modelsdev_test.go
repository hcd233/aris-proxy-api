// Package modelsdev models.dev 公开定价客户端的单元测试
package modelsdev

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/modelsdev"
)

func TestQuoteExactMatchAndMiss(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"gpt-4":{"cost":{"input":"3","output":"15","cache_read":"0.3","cache_write":"3.75"}}}`))
	}))
	t.Cleanup(srv.Close)

	c := modelsdev.NewClient(srv.Client(), nil)
	c.Source = srv.URL
	q, ok, err := c.Quote(t.Context(), "gpt-4")
	if err != nil || !ok {
		t.Fatalf("quote: ok=%v err=%v", ok, err)
	}
	if q.Input != 3 || q.Output != 15 || q.CacheCreation != 3.75 || q.CacheRead != 0.3 {
		t.Fatalf("quote = %+v", q)
	}
	if _, ok, _ := c.Quote(t.Context(), "gpt-4-turbo"); ok {
		t.Fatal("unknown model must miss")
	}
}

func TestQuoteFetchErrorSurfaces(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	c := modelsdev.NewClient(srv.Client(), nil)
	c.Source = srv.URL
	if _, ok, err := c.Quote(t.Context(), "x"); ok || err == nil {
		t.Fatalf("fetch failure must return err and ok=false, got ok=%v err=%v", ok, err)
	}
}

func TestQuoteNumberCostValues(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"m":{"cost":{"input":1.5,"output":2}}}`))
	}))
	t.Cleanup(srv.Close)

	c := modelsdev.NewClient(srv.Client(), nil)
	c.Source = srv.URL
	q, ok, err := c.Quote(t.Context(), "m")
	if err != nil || !ok {
		t.Fatalf("quote: ok=%v err=%v", ok, err)
	}
	if q.Input != 1.5 || q.Output != 2 {
		t.Fatalf("quote = %+v", q)
	}
}
