// Package client_models 验证客户端模型列表接口行为。
//
// 离线单测直连 handler（fake 端口），验证响应结构与字段裁剪；
// 在线 e2e 由 BASE_URL / API_KEY 环境变量门控。
package client_models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"

	llmproxyport "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
)

const e2eHTTPTimeout = 30 * time.Second

// fakeListClientModels 端口 fake：返回一条含完整字段的模型
type fakeListClientModels struct{}

func (f *fakeListClientModels) Handle(_ context.Context) (*dto.ClientModelsRsp, error) {
	return &dto.ClientModelsRsp{Models: []*dto.ClientModelItem{{
		Alias:           "gpt-4o",
		UpstreamModel:   "gpt-4o-2024",
		ContextLength:   128000,
		MaxOutputTokens: 16384,
		Capabilities:    []enum.InputModality{enum.InputModalityText, enum.InputModalityImage},
	}}}, nil
}

var _ llmproxyport.ListClientModelsHandler = (*fakeListClientModels)(nil)

func TestClientListModels_ReturnsTrimmedItems(t *testing.T) {
	t.Parallel()
	h := handler.NewClientHandler(handler.ClientDependencies{List: &fakeListClientModels{}})
	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("client models test", "1.0"))
	huma.Register(api, huma.Operation{
		OperationID: "listClientModels", Method: http.MethodGet, Path: constant.ClientModelsListPath,
		Tags: []string{constant.TagClient},
	}, h.HandleListModels)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, constant.ClientModelsListPath, http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Models []struct {
			Alias           string   `json:"alias"`
			UpstreamModel   string   `json:"upstreamModel"`
			ContextLength   int      `json:"contextLength"`
			MaxOutputTokens int      `json:"maxOutputTokens"`
			Capabilities    []string `json:"capabilities"`
		} `json:"models"`
	}
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	models := body.Models
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	m := models[0]
	if m.Alias != "gpt-4o" || m.UpstreamModel != "gpt-4o-2024" || m.ContextLength != 128000 || m.MaxOutputTokens != 16384 {
		t.Fatalf("unexpected model fields: %+v", m)
	}
	hasText := false
	for _, c := range m.Capabilities {
		if c == "text" {
			hasText = true
		}
	}
	if !hasText {
		t.Fatalf("capabilities must contain text: %v", m.Capabilities)
	}
}

func TestClientListModels_HandlerError(t *testing.T) {
	t.Parallel()
	errHandler := func(context.Context) (*dto.ClientModelsRsp, error) { return nil, context.Canceled }
	h := handler.NewClientHandler(handler.ClientDependencies{List: errFuncPort(errHandler)})
	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("t", "1"))
	huma.Register(api, huma.Operation{
		OperationID: "listClientModels", Method: http.MethodGet, Path: constant.ClientModelsListPath,
	}, h.HandleListModels)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, constant.ClientModelsListPath, http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		t.Fatal("expected non-200 on handler error")
	}
}

type errFuncPort func(context.Context) (*dto.ClientModelsRsp, error)

func (f errFuncPort) Handle(ctx context.Context) (*dto.ClientModelsRsp, error) { return f(ctx) }

func TestClientModels_E2E(t *testing.T) {
	t.Parallel()
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL / API_KEY not set; skipping live e2e")
	}
	ctx, cancel := context.WithTimeout(t.Context(), e2eHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+constant.ClientModelsListPath, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Models []struct {
			Alias        string   `json:"alias"`
			Capabilities []string `json:"capabilities"`
		} `json:"models"`
	}
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for _, m := range body.Models {
		if m.Alias == "" {
			t.Fatal("model alias must not be empty")
		}
		hasText := false
		for _, c := range m.Capabilities {
			if c == "text" {
				hasText = true
			}
		}
		if !hasText {
			t.Fatalf("model %q capabilities must contain text: %v", m.Alias, m.Capabilities)
		}
	}
}
