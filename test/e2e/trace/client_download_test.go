package trace_e2e

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	tracecommand "github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/traceclient"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
)

func TestTraceClientDownload_TicketLimitAndSingleUse(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	artifactDir := t.TempDir()
	artifactBody := []byte("trace-client-binary")
	artifactPath := filepath.Join(artifactDir, constant.TraceClientArtifactDarwinARM64)
	if err := os.WriteFile(artifactPath, artifactBody, 0o600); err != nil {
		t.Fatal(err)
	}
	store := cache.NewTraceClientTicketStore(redisClient)
	traceHandler := handler.NewTraceHandler(handler.TraceDependencies{
		IssueTicket:      tracecommand.NewIssueTraceClientTicketHandler(store),
		ArtifactResolver: traceclient.NewArtifactResolver(artifactDir),
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Trace Client Test", "1.0"))
	userMiddleware := func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, uint(7))
		ctx = huma.WithValue(ctx, constant.CtxKeyPermission, enum.PermissionUser)
		next(ctx)
	}
	huma.Register(api, huma.Operation{
		OperationID: "issueTraceClientTicketTest",
		Method:      http.MethodPost,
		Path:        "/ticket",
		Middlewares: huma.Middlewares{
			userMiddleware,
			middleware.TokenBucketRateLimiterMiddleware(
				redisClient,
				constant.TraceClientTicketRateLimitService,
				constant.CtxKeyUserID,
				constant.PeriodIssueTraceClientTicket,
				constant.LimitIssueTraceClientTicket,
			),
		},
	}, traceHandler.HandleIssueTraceClientTicket)
	huma.Register(api, huma.Operation{
		OperationID: "downloadTraceClientTest",
		Method:      http.MethodGet,
		Path:        "/client",
		Middlewares: huma.Middlewares{middleware.TraceClientTicketMiddleware(store)},
	}, traceHandler.HandleDownloadTraceClient)

	var firstTicket string
	for i := range 4 {
		resp := request(t, app, http.MethodPost, "/ticket", "")
		if i == 3 {
			if resp.StatusCode != fiber.StatusTooManyRequests {
				t.Fatalf("fourth ticket status = %d", resp.StatusCode)
			}
			_ = resp.Body.Close()
			continue
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("ticket %d status = %d", i, resp.StatusCode)
		}
		var body dto.IssueTraceClientTicketRsp
		if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if i == 0 {
			firstTicket = body.Ticket
		}
	}

	path := "/client?os=darwin&arch=arm64"
	resp := request(t, app, http.MethodGet, path, firstTicket)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("download status = %d", resp.StatusCode)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if !bytes.Equal(got, artifactBody) {
		t.Fatalf("download body = %q", got)
	}
	if resp.Header.Get(constant.HTTPHeaderContentDisposition) != `attachment; filename="aris"` {
		t.Fatalf("content disposition = %q", resp.Header.Get(constant.HTTPHeaderContentDisposition))
	}
	if resp.Header.Get(constant.HTTPHeaderCacheControl) != constant.HTTPCacheControlNoStore {
		t.Fatalf("cache control = %q", resp.Header.Get(constant.HTTPHeaderCacheControl))
	}

	resp = request(t, app, http.MethodGet, path, firstTicket)
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("reused ticket status = %d", resp.StatusCode)
	}
}

func TestTraceClientInstall_ReturnsScriptWithoutConsumingTicket(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	artifactDir := t.TempDir()
	artifactPath := filepath.Join(artifactDir, constant.TraceClientArtifactDarwinARM64)
	if err := os.WriteFile(artifactPath, []byte("trace-client-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := cache.NewTraceClientTicketStore(redisClient)
	traceHandler := handler.NewTraceHandler(handler.TraceDependencies{
		IssueTicket:      tracecommand.NewIssueTraceClientTicketHandler(store),
		ArtifactResolver: traceclient.NewArtifactResolver(artifactDir),
		TicketStore:      store,
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Trace Install Test", "1.0"))
	userMiddleware := func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, uint(7))
		ctx = huma.WithValue(ctx, constant.CtxKeyPermission, enum.PermissionUser)
		next(ctx)
	}
	huma.Register(api, huma.Operation{
		OperationID: "issueTraceClientTicketInstall",
		Method:      http.MethodPost,
		Path:        "/ticket",
		Middlewares: huma.Middlewares{userMiddleware},
	}, traceHandler.HandleIssueTraceClientTicket)
	huma.Register(api, huma.Operation{
		OperationID: "installTraceClientTest",
		Method:      http.MethodGet,
		Path:        "/client/install",
	}, traceHandler.HandleInstallTraceClient)
	huma.Register(api, huma.Operation{
		OperationID: "downloadTraceClientInstallTest",
		Method:      http.MethodGet,
		Path:        "/client",
		Middlewares: huma.Middlewares{middleware.TraceClientTicketMiddleware(store)},
	}, traceHandler.HandleDownloadTraceClient)

	issueResp := request(t, app, http.MethodPost, "/ticket", "")
	if issueResp.StatusCode != fiber.StatusOK {
		t.Fatalf("issue status = %d", issueResp.StatusCode)
	}
	var issueBody dto.IssueTraceClientTicketRsp
	if err := sonic.ConfigDefault.NewDecoder(issueResp.Body).Decode(&issueBody); err != nil {
		t.Fatal(err)
	}
	_ = issueResp.Body.Close()
	ticket := issueBody.Ticket
	if ticket == "" {
		t.Fatal("empty ticket")
	}

	installReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/client/install",
		http.NoBody,
	)
	installReq.Host = "trace.example.com"
	installReq.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+ticket)
	installReq.Header.Set(constant.HTTPHeaderXForwardedProto, "https")
	installResp, err := app.Test(installReq, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	if installResp.StatusCode != fiber.StatusOK {
		t.Fatalf("install status = %d", installResp.StatusCode)
	}
	script, err := io.ReadAll(installResp.Body)
	_ = installResp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	scriptStr := string(script)

	if !strings.Contains(scriptStr, ticket) {
		t.Errorf("script does not contain ticket")
	}
	if !strings.Contains(scriptStr, "https://trace.example.com") {
		t.Errorf("script does not contain origin: %s", scriptStr)
	}
	if ct := installResp.Header.Get(constant.HTTPHeaderContentType); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content type = %q", ct)
	}

	downloadResp := request(t, app, http.MethodGet, "/client?os=darwin&arch=arm64", ticket)
	defer downloadResp.Body.Close()
	if downloadResp.StatusCode != fiber.StatusOK {
		t.Fatalf("download after install status = %d (ticket should not be consumed)", downloadResp.StatusCode)
	}
}

func TestTraceClientInstall_InvalidTicketReturnsBashErrorScript(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	store := cache.NewTraceClientTicketStore(redisClient)
	traceHandler := handler.NewTraceHandler(handler.TraceDependencies{
		TicketStore: store,
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Trace Install Error Test", "1.0"))
	huma.Register(api, huma.Operation{
		OperationID: "installTraceClientInvalidTest",
		Method:      http.MethodGet,
		Path:        "/client/install",
	}, traceHandler.HandleInstallTraceClient)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/client/install",
		http.NoBody,
	)
	req.Host = "trace.example.com"
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+"invalid-ticket-value")
	req.Header.Set(constant.HTTPHeaderXForwardedProto, "https")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200 (bash error script)", resp.StatusCode)
	}
	if ct := resp.Header.Get(constant.HTTPHeaderContentType); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content type = %q, want text/plain", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	bodyStr := string(body)
	if !strings.HasPrefix(bodyStr, "#!/usr/bin/env bash") {
		t.Errorf("response is not a bash script: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, ">&2") {
		t.Errorf("error script should echo to stderr: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "exit 1") {
		t.Errorf("error script should exit 1: %s", bodyStr)
	}
}

func request(t *testing.T, app *fiber.App, method, path, ticket string) *http.Response {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, http.NoBody)
	if ticket != "" {
		req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+ticket)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
