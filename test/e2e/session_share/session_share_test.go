// Package session_share 验证 Session Share 创建与访问的 sessionID 一致性。
//
// 回归背景（bugfix/session-share-body-2026-05-28）：
//   - CreateShareReq DTO 没有遵循 huma 的 Body 包装规范，
//     导致 POST /api/v1/session/share 时 sessionId 始终是零值 0；
//   - 0 写入 redis 后，GET /api/v1/session/share/metadata?id=xxx 拿到 0，
//     再传给 GORM 由于零值 where 条件被忽略，返回了别人的 session。
//
// 本测试覆盖：
//  1. 用 JWT 调用 POST /api/v1/session/share 携带 sessionId
//  2. 用返回的 shareId 公开访问 GET /api/v1/session/share/metadata?id=xxx
//  3. 断言 response.session.id == 请求传入的 sessionId
//
// 环境变量：
//   - BASE_URL          API 根地址（必填）
//   - JWT_TOKEN         登录后的 JWT，含 user_id（必填）
//   - SHARE_SESSION_ID  发起方拥有的 sessionId（必填，正整数）
package session_share

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

const e2eHTTPTimeout = 30 * time.Second

func mustE2EEnv(t *testing.T) (baseURL, jwtToken string, sessionID uint) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	rawSessionID := os.Getenv("SHARE_SESSION_ID")
	if baseURL == "" || jwtToken == "" || rawSessionID == "" {
		t.Skip("BASE_URL, JWT_TOKEN and SHARE_SESSION_ID are required for e2e test")
	}
	parsed, err := strconv.ParseUint(rawSessionID, 10, 64)
	if err != nil || parsed == 0 {
		t.Skipf("invalid SHARE_SESSION_ID=%q: must be positive integer", rawSessionID)
	}
	return strings.TrimRight(baseURL, "/"), jwtToken, uint(parsed)
}

func newE2EClient() *http.Client {
	return &http.Client{Timeout: e2eHTTPTimeout}
}

// createShareResp 与服务端 dto.CreateShareRsp 对齐（huma 会 unwrap Body 作为响应体）
type createShareResp struct {
	ShareID   string    `json:"shareId"`
	ExpiresAt time.Time `json:"expiresAt"`
	Error     *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type getShareMetadataResp struct {
	Session *struct {
		ID uint `json:"id"`
	} `json:"session,omitempty"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// TestSessionShare_CreateAndAccess_SessionIDConsistency 验证创建与访问的 sessionID 一致。
//
//	@author centonhuang
//	@update 2026-05-28 14:35:00
func TestSessionShare_CreateAndAccess_SessionIDConsistency(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken, sessionID := mustE2EEnv(t)
	client := newE2EClient()

	// Step 1: 创建分享
	createBody, err := sonic.Marshal(map[string]any{"sessionId": sessionID})
	if err != nil {
		t.Fatalf("marshal create body failed: %v", err)
	}
	createReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/v1/session/share", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("build create request failed: %v", err)
	}
	createReq.Header.Set(constant.HTTPTitleHeaderAuthorization, constant.HTTPAuthBearerPrefix+jwtToken)
	createReq.Header.Set("Content-Type", constant.HTTPContentTypeJSON)

	createHTTPResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("send create request failed: %v", err)
	}
	defer func() { _ = createHTTPResp.Body.Close() }()

	if createHTTPResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(createHTTPResp.Body)
		t.Fatalf("create share unexpected status=%d (traceID=%s); body=%s",
			createHTTPResp.StatusCode,
			createHTTPResp.Header.Get(constant.HTTPTitleHeaderTraceID),
			string(body))
	}
	createTraceID := createHTTPResp.Header.Get(constant.HTTPTitleHeaderTraceID)
	t.Logf("Create share traceID=%s", createTraceID)

	createBodyBytes, err := io.ReadAll(createHTTPResp.Body)
	if err != nil {
		t.Fatalf("read create response body failed: %v", err)
	}
	var created createShareResp
	if unmarshalErr := sonic.Unmarshal(createBodyBytes, &created); unmarshalErr != nil {
		t.Fatalf("unmarshal create response failed: %v; body=%s", unmarshalErr, string(createBodyBytes))
	}
	if created.Error != nil {
		t.Fatalf("create share returned error: code=%s, msg=%s (traceID=%s)",
			created.Error.Code, created.Error.Message, createTraceID)
	}
	if created.ShareID == "" {
		t.Fatalf("create share returned empty shareId; body=%s", string(createBodyBytes))
	}
	if created.ExpiresAt.Before(time.Now()) {
		t.Fatalf("create share returned expired expiresAt=%s", created.ExpiresAt)
	}

	// Step 2: 公开访问分享元数据
	getReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/api/v1/session/share/metadata?id="+created.ShareID, http.NoBody)
	if err != nil {
		t.Fatalf("build get request failed: %v", err)
	}
	getHTTPResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("send get request failed: %v", err)
	}
	defer func() { _ = getHTTPResp.Body.Close() }()

	if getHTTPResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getHTTPResp.Body)
		t.Fatalf("get share unexpected status=%d (traceID=%s); body=%s",
			getHTTPResp.StatusCode,
			getHTTPResp.Header.Get(constant.HTTPTitleHeaderTraceID),
			string(body))
	}
	getTraceID := getHTTPResp.Header.Get(constant.HTTPTitleHeaderTraceID)
	t.Logf("Get share traceID=%s", getTraceID)

	getBodyBytes, err := io.ReadAll(getHTTPResp.Body)
	if err != nil {
		t.Fatalf("read get response body failed: %v", err)
	}
	var got getShareMetadataResp
	if unmarshalErr := sonic.Unmarshal(getBodyBytes, &got); unmarshalErr != nil {
		t.Fatalf("unmarshal get response failed: %v; body=%s", unmarshalErr, string(getBodyBytes))
	}
	if got.Error != nil {
		t.Fatalf("get share returned error: code=%s, msg=%s (traceID=%s)",
			got.Error.Code, got.Error.Message, getTraceID)
	}
	if got.Session == nil {
		t.Fatalf("get share metadata returned empty session; body=%s", string(getBodyBytes))
	}

	// Step 3: 核心断言——回归点
	if got.Session.ID != sessionID {
		t.Fatalf("session id mismatch: created with sessionId=%d but accessed session.id=%d (createTraceID=%s, getTraceID=%s)",
			sessionID, got.Session.ID, createTraceID, getTraceID)
	}
	t.Logf("session id consistency verified: sessionId=%d", sessionID)
}

// TestSessionShare_Create_RejectsZeroSessionID 验证 sessionId=0 被拒绝。
//
//	@author centonhuang
//	@update 2026-05-28 14:35:00
func TestSessionShare_Create_RejectsZeroSessionID(t *testing.T) {
	t.Parallel()
	baseURL := os.Getenv("BASE_URL")
	jwtToken := os.Getenv("JWT_TOKEN")
	if baseURL == "" || jwtToken == "" {
		t.Skip("BASE_URL and JWT_TOKEN are required for e2e test")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	body, err := sonic.Marshal(map[string]any{"sessionId": 0})
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/v1/session/share", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set(constant.HTTPTitleHeaderAuthorization, constant.HTTPAuthBearerPrefix+jwtToken)
	req.Header.Set("Content-Type", constant.HTTPContentTypeJSON)

	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("send request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// huma minimum:1 应该在框架层返回 422
	// 即便框架层放行，业务层也应返回非 200 或带 error 字段
	if resp.StatusCode == http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var rsp getShareMetadataResp
		if unmarshalErr := sonic.Unmarshal(bodyBytes, &rsp); unmarshalErr == nil && rsp.Error == nil {
			t.Fatalf("expected error for sessionId=0 but got success; body=%s", string(bodyBytes))
		}
	}
	t.Logf("zero sessionId rejected as expected, status=%d, traceID=%s",
		resp.StatusCode, resp.Header.Get(constant.HTTPTitleHeaderTraceID))
}
