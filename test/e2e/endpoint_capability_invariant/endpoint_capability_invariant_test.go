// Package endpoint_capability_invariant 回归 2026-10-12 tokenhub 事故：
// 「打开协议能力却不给对应 baseURL」的 PATCH 必须在写库前被拒，
// 且该请求之后 upstream 列表仍可读、该端点的能力标记保持原值。
//
// 事故链路（生产 traceId f2646f81-6b07-4f29-967c-32e391661d1b）：
//  1. PATCH /api/web/v1/endpoint?id=6 打开 supportAnthropicMessage，请求体不带
//     anthropicBaseURL（库中本来就是空），写路径不做不变量校验 → 落库成
//     「能力已开、baseURL 为空」；
//  2. 读路径恢复聚合根时跑校验型工厂 → 该行读不出来：GET /upstream/list 与代理
//     路由连续报 10006，连 PATCH 修复入口自身也失败（必须先 FindByID），
//     只能手改数据库。
//
// 环境变量：
//   - BASE_URL   API 根地址（必填）
//   - JWT_TOKEN  管理员 JWT
//
// 本用例只调管理端接口，不产生任何上游流量；baseURL 必须是可解析的域名
// （服务端 SSRF 校验会做 DNS 解析）。
package endpoint_capability_invariant

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/test/e2e/e2eguard"
)

const (
	e2eHTTPTimeout   = 30 * time.Second
	openaiBaseURL    = "https://api.openai.com/v1"
	anthropicBaseURL = "https://api.anthropic.com/v1"
)

type bizError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type commandRsp struct {
	Error *bizError `json:"error,omitempty"`
}

type endpointView struct {
	ID                      uint   `json:"id"`
	Name                    string `json:"name"`
	AnthropicBaseURL        string `json:"anthropicBaseURL"`
	SupportAnthropicMessage bool   `json:"supportAnthropicMessage"`
}

type upstreamGroup struct {
	Endpoint endpointView `json:"endpoint"`
}

type upstreamRsp struct {
	Groups []upstreamGroup `json:"groups"`
	Error  *bizError       `json:"error,omitempty"`
}

func mustE2EEnv(t *testing.T) (baseURL, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || jwtToken == "" {
		t.Skip("BASE_URL or JWT_TOKEN not set, skip e2e")
	}
	e2eguard.GuardLiveTarget(t, baseURL)
	return baseURL, jwtToken
}

func doJSON(t *testing.T, method, url, token string, body []byte) (status int, data []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), e2eHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	if body != nil {
		req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, url, err)
	}
	defer func() { _ = rsp.Body.Close() }()
	data, err = io.ReadAll(rsp.Body)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	return rsp.StatusCode, data
}

// mustEndpointByName 从 upstream 列表按名称取端点；列表读不出来即失败
// （事故期间这里返回 10006，正是被脏行拖垮的那条读路径）。
func mustEndpointByName(t *testing.T, baseURL, token, name string) endpointView {
	t.Helper()
	status, data := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/web/v1/upstream/list?page=1&pageSize=100&query=%s", baseURL, name), token, nil)
	if status != http.StatusOK {
		t.Fatalf("list upstream: status=%d body=%s", status, data)
	}
	var rsp upstreamRsp
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("unmarshal upstream rsp: %v", err)
	}
	if rsp.Error != nil {
		t.Fatalf("upstream list must stay readable, biz error: %+v", rsp.Error)
	}
	for _, g := range rsp.Groups {
		if g.Endpoint.Name == name {
			return g.Endpoint
		}
	}
	t.Fatalf("endpoint %q not found in upstream list", name)
	return endpointView{}
}

// TestEndpointPATCH_CapabilityWithoutBaseURL 打开 Anthropic 能力但不给 baseURL：
// PATCH 被拒且不落库，upstream 列表全程可读；补齐 baseURL 后同一更新成功。
func TestEndpointPATCH_CapabilityWithoutBaseURL(t *testing.T) {
	t.Parallel()
	baseURL, token := mustE2EEnv(t)

	epName := fmt.Sprintf("e2e-epcap-%d", time.Now().UnixNano())
	status, data := doJSON(t, http.MethodPost, baseURL+"/api/web/v1/endpoint", token,
		[]byte(fmt.Sprintf(`{"name":%q,"apiKey":"sk-e2e","openaiBaseURL":%q,"supportOpenAIChatCompletion":true}`, epName, openaiBaseURL)))
	if status != http.StatusOK {
		t.Fatalf("create endpoint: status=%d body=%s", status, data)
	}
	epID := mustEndpointByName(t, baseURL, token, epName).ID
	t.Cleanup(func() {
		doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/web/v1/endpoint?id=%d", baseURL, epID), token, nil)
	})

	// 1. 复刻事故请求：只开能力，不带 anthropic baseURL → 必须被拒
	status, data = doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/web/v1/endpoint?id=%d", baseURL, epID), token,
		[]byte(fmt.Sprintf(`{"name":%q,"openaiBaseURL":%q,"supportOpenAIChatCompletion":true,"supportAnthropicMessage":true}`, epName, openaiBaseURL)))
	var rejected commandRsp
	if err := sonic.Unmarshal(data, &rejected); err != nil {
		t.Fatalf("unmarshal patch rsp: %v", err)
	}
	if status == http.StatusOK && rejected.Error == nil {
		t.Fatalf("capability without baseURL must be rejected, got status=%d body=%s", status, data)
	}

	// 2. 该端点仍可读，能力标记保持 false（没有写成半坏状态）
	if ep := mustEndpointByName(t, baseURL, token, epName); ep.SupportAnthropicMessage {
		t.Fatalf("rejected update must not be persisted: %+v", ep)
	}

	// 3. 补齐 anthropic baseURL 后同一更新放行
	status, data = doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/web/v1/endpoint?id=%d", baseURL, epID), token,
		[]byte(fmt.Sprintf(`{"anthropicBaseURL":%q,"supportAnthropicMessage":true}`, anthropicBaseURL)))
	var accepted commandRsp
	if err := sonic.Unmarshal(data, &accepted); err != nil {
		t.Fatalf("unmarshal patch rsp: %v", err)
	}
	if status != http.StatusOK || accepted.Error != nil {
		t.Fatalf("capability with baseURL should be accepted, got status=%d body=%s", status, data)
	}
	if ep := mustEndpointByName(t, baseURL, token, epName); !ep.SupportAnthropicMessage || ep.AnthropicBaseURL == "" {
		t.Fatalf("accepted update must be persisted: %+v", ep)
	}
}
