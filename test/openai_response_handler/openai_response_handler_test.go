// Package openai_response_handler verifies that Huma's schema-based request
// validation for the OpenAI Response API (and Chat Completions) does not
// reject forward-compatible unknown fields such as Codex Desktop's
// `client_metadata`, which previously produced a 422 Unprocessable Entity.
package openai_response_handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	humago "github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// handlerCase mirrors fixtures/cases.json
type handlerCase struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Path        string                 `json:"path"`
	RequestBody sonic.NoCopyRawMessage `json:"request_body"`
	WantStatus  int                    `json:"want_status"`
}

func loadHandlerCases(t *testing.T) []handlerCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []handlerCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

// buildTestAPI registers /responses and /chat/completions handlers that return
// 200 OK on successful body binding. The handlers do not perform any upstream
// I/O; they only assert that Huma's schema validation accepts the payload.
func buildTestAPI(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Aris Test", "1.0"))

	huma.Register(api, huma.Operation{
		OperationID: "createResponse",
		Method:      http.MethodPost,
		Path:        "/api/openai/v1/responses",
	}, func(_ context.Context, _ *dto.OpenAICreateResponseRequest) (*struct {
		Body string
	}, error) {
		return &struct{ Body string }{Body: "ok"}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createChatCompletion",
		Method:      http.MethodPost,
		Path:        "/api/openai/v1/chat/completions",
	}, func(_ context.Context, _ *dto.OpenAIChatCompletionRequest) (*struct {
		Body string
	}, error) {
		return &struct{ Body string }{Body: "ok"}, nil
	})

	return mux
}

// TestCreateResponse_AcceptsUnknownFields drives every handler case from
// fixtures/cases.json and asserts the returned HTTP status matches the
// fixture expectation. The regression scenario is a Codex Desktop payload
// containing `client_metadata`, which must NOT trigger a 422.
func TestCreateResponse_AcceptsUnknownFields(t *testing.T) {
	mux := buildTestAPI(t)
	cases := loadHandlerCases(t)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.Path, strings.NewReader(string(tc.RequestBody)))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.WantStatus {
				t.Errorf("status = %d, want %d\nbody: %s", rec.Code, tc.WantStatus, rec.Body.String())
			}
		})
	}
}
