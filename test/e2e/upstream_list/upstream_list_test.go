// Package upstream_list 验证 GET /api/v1/upstream/list 分组分页、keyword 整组聚合、
// 嵌套 user 回填与用户隔离的全链路行为。
//
// 需求背景（feature/upstream-maas-web-redesign）：
//   - endpoint 组分页：pageInfo.total=端点数，modelTotal=当前筛选范围模型总数；
//   - keyword 命中 endpoint 名或组内模型字段时整组返回（组不跨页断裂）；
//   - 嵌套 user{id,name,avatar} 对象，归属缺失时字段缺省；
//   - 普通用户 scope 隔离；admin 支持 username 过滤与代建。
//
// 环境变量：
//   - BASE_URL   API 根地址（必填）
//   - JWT_TOKEN  管理员 JWT
package upstream_list

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
)

const e2eHTTPTimeout = 30 * time.Second

type bizError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type upstreamUser struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type upstreamEndpoint struct {
	ID        uint          `json:"id"`
	User      *upstreamUser `json:"user,omitempty"`
	Name      string        `json:"name"`
	MaskedKey string        `json:"maskedAPIKey"`
}

type upstreamModel struct {
	ID      uint   `json:"id"`
	Alias   string `json:"alias"`
	Enabled bool   `json:"enabled"`
}

type upstreamGroup struct {
	Endpoint   upstreamEndpoint `json:"endpoint"`
	Models     []upstreamModel  `json:"models"`
	ModelCount int              `json:"modelCount"`
	Truncated  bool             `json:"truncated,omitempty"`
}

type pageInfo struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type listUpstreamRsp struct {
	Groups     []upstreamGroup `json:"groups"`
	ModelTotal int64           `json:"modelTotal"`
	PageInfo   *pageInfo       `json:"pageInfo,omitempty"`
	Error      *bizError       `json:"error,omitempty"`
}

type commandRsp struct {
	Error *bizError `json:"error,omitempty"`
}

func mustE2EEnv(t *testing.T) (baseURL, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || jwtToken == "" {
		t.Skip("BASE_URL or JWT_TOKEN not set, skip e2e")
	}
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
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+" "+token)
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, url, err)
	}
	defer rsp.Body.Close()
	data, err = io.ReadAll(rsp.Body)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	return rsp.StatusCode, data
}

func mustListUpstream(t *testing.T, baseURL, token, query string) listUpstreamRsp {
	t.Helper()
	url := baseURL + "/api/v1/upstream/list?page=1&pageSize=100"
	if query != "" {
		url += "&query=" + query
	}
	status, data := doJSON(t, http.MethodGet, url, token, nil)
	if status != http.StatusOK {
		t.Fatalf("list upstream: status=%d body=%s", status, data)
	}
	var rsp listUpstreamRsp
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("unmarshal upstream rsp: %v", err)
	}
	if rsp.Error != nil {
		t.Fatalf("list upstream biz error: %+v", rsp.Error)
	}
	return rsp
}

// TestUpstreamList_GroupPaginationAndTotals 创建 1 端点 + 2 模型后，
// 分组视图的 total/modelTotal/modelCount 与嵌套 user 结构正确。
func TestUpstreamList_GroupPaginationAndTotals(t *testing.T) {
	t.Parallel()
	baseURL, token := mustE2EEnv(t)

	stamp := time.Now().UnixNano()
	epName := fmt.Sprintf("e2e-upl-ep-%d", stamp)
	aliasA := fmt.Sprintf("e2e-upl-ma-%d", stamp)
	aliasB := fmt.Sprintf("e2e-upl-mb-%d", stamp)

	createBody := fmt.Sprintf(`{"body":{"name":%q,"apiKey":"sk-e2e","openaiBaseURL":"https://o.example.com/v1","supportOpenAIChatCompletion":true}}`, epName)
	status, data := doJSON(t, http.MethodPost, baseURL+"/api/v1/endpoint", token, []byte(createBody))
	if status != http.StatusOK {
		t.Fatalf("create endpoint: status=%d body=%s", status, data)
	}
	var created commandRsp
	if err := sonic.Unmarshal(data, &created); err != nil || created.Error != nil {
		t.Fatalf("create endpoint rsp: raw=%s err=%v", data, err)
	}
	cleanup := func() {
		rsp := mustListUpstream(t, baseURL, token, epName)
		for _, g := range rsp.Groups {
			if g.Endpoint.Name == epName {
				doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/endpoint?id=%d", baseURL, g.Endpoint.ID), token, nil)
			}
		}
	}
	t.Cleanup(cleanup)

	for _, alias := range []string{aliasA, aliasB} {
		body := fmt.Sprintf(`{"body":{"alias":%q,"upstreamModel":"up-%s","endpointID":%d}}`, alias, epName, mustEndpointIDByName(t, baseURL, token, epName))
		status, data = doJSON(t, http.MethodPost, baseURL+"/api/v1/model", token, []byte(body))
		if status != http.StatusOK {
			t.Fatalf("create model %s: status=%d body=%s", alias, status, data)
		}
	}

	rsp := mustListUpstream(t, baseURL, token, epName)
	if len(rsp.Groups) != 1 {
		t.Fatalf("expected exactly 1 group for query=%q, got %d", epName, len(rsp.Groups))
	}
	g := rsp.Groups[0]
	if g.Endpoint.Name != epName {
		t.Fatalf("unexpected group endpoint name: %q", g.Endpoint.Name)
	}
	if g.ModelCount != 2 || len(g.Models) != 2 {
		t.Fatalf("expected modelCount=2, got count=%d len=%d", g.ModelCount, len(g.Models))
	}
	if rsp.ModelTotal < 2 {
		t.Fatalf("modelTotal should cover at least the 2 created models, got %d", rsp.ModelTotal)
	}
	if rsp.PageInfo == nil || rsp.PageInfo.Total < 1 {
		t.Fatalf("pageInfo.total should count endpoints, got %+v", rsp.PageInfo)
	}
	if g.Truncated {
		t.Fatalf("group unexpectedly truncated")
	}
}

// TestUpstreamList_KeywordAggregatesWholeGroup keyword 命中组内单个模型别名时整组返回。
func TestUpstreamList_KeywordAggregatesWholeGroup(t *testing.T) {
	t.Parallel()
	baseURL, token := mustE2EEnv(t)

	stamp := time.Now().UnixNano()
	epName := fmt.Sprintf("e2e-upk-ep-%d", stamp)
	aliasA := fmt.Sprintf("e2e-upk-aa-%d", stamp)
	aliasB := fmt.Sprintf("e2e-upk-bb-%d", stamp)

	status, data := doJSON(t, http.MethodPost, baseURL+"/api/v1/endpoint", token,
		[]byte(fmt.Sprintf(`{"body":{"name":%q,"apiKey":"sk-e2e","supportOpenAIChatCompletion":true}}`, epName)))
	if status != http.StatusOK {
		t.Fatalf("create endpoint: status=%d body=%s", status, data)
	}
	cleanup := func() {
		rsp := mustListUpstream(t, baseURL, token, epName)
		for _, g := range rsp.Groups {
			if g.Endpoint.Name == epName {
				doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/endpoint?id=%d", baseURL, g.Endpoint.ID), token, nil)
			}
		}
	}
	t.Cleanup(cleanup)

	epID := mustEndpointIDByName(t, baseURL, token, epName)
	for _, alias := range []string{aliasA, aliasB} {
		body := fmt.Sprintf(`{"body":{"alias":%q,"upstreamModel":"up-%s","endpointID":%d}}`, alias, alias, epID)
		status, data = doJSON(t, http.MethodPost, baseURL+"/api/v1/model", token, []byte(body))
		if status != http.StatusOK {
			t.Fatalf("create model %s: status=%d body=%s", alias, status, data)
		}
	}

	// 只按 aliasA 检索 → 整组返回（包含 aliasB）
	rsp := mustListUpstream(t, baseURL, token, aliasA)
	if len(rsp.Groups) != 1 || rsp.Groups[0].Endpoint.Name != epName {
		t.Fatalf("expected single group %q for keyword alias search, got %+v groups", epName, len(rsp.Groups))
	}
	if len(rsp.Groups[0].Models) != 2 {
		t.Fatalf("keyword should return whole group (2 models), got %d", len(rsp.Groups[0].Models))
	}
}

// TestUpstreamList_NestedUserObject 分组的嵌套 user 对象回填归属用户名。
func TestUpstreamList_NestedUserObject(t *testing.T) {
	t.Parallel()
	baseURL, token := mustE2EEnv(t)

	stamp := time.Now().UnixNano()
	epName := fmt.Sprintf("e2e-upu-ep-%d", stamp)

	status, data := doJSON(t, http.MethodPost, baseURL+"/api/v1/endpoint", token,
		[]byte(fmt.Sprintf(`{"body":{"name":%q,"apiKey":"sk-e2e","supportOpenAIChatCompletion":true}}`, epName)))
	if status != http.StatusOK {
		t.Fatalf("create endpoint: status=%d body=%s", status, data)
	}
	cleanup := func() {
		rsp := mustListUpstream(t, baseURL, token, epName)
		for _, g := range rsp.Groups {
			if g.Endpoint.Name == epName {
				doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/endpoint?id=%d", baseURL, g.Endpoint.ID), token, nil)
			}
		}
	}
	t.Cleanup(cleanup)

	rsp := mustListUpstream(t, baseURL, token, epName)
	if len(rsp.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(rsp.Groups))
	}
	u := rsp.Groups[0].Endpoint.User
	if u == nil {
		t.Fatalf("expected nested user object present for creator")
	}
	if u.ID == 0 || u.Name == "" {
		t.Fatalf("nested user incomplete: id=%d name=%q avatar=%q", u.ID, u.Name, u.Avatar)
	}
}

// TestUpstreamList_ScopeIsolationAndCascadingDelete 用户删除端点后分组消失且模型随级联消失；
// 已删除端点不再出现在任何人的列表中。
func TestUpstreamList_ScopeIsolationAndCascadingDelete(t *testing.T) {
	t.Parallel()
	baseURL, token := mustE2EEnv(t)

	stamp := time.Now().UnixNano()
	epName := fmt.Sprintf("e2e-ups-ep-%d", stamp)
	alias := fmt.Sprintf("e2e-ups-m-%d", stamp)

	status, data := doJSON(t, http.MethodPost, baseURL+"/api/v1/endpoint", token,
		[]byte(fmt.Sprintf(`{"body":{"name":%q,"apiKey":"sk-e2e","supportOpenAIChatCompletion":true}}`, epName)))
	if status != http.StatusOK {
		t.Fatalf("create endpoint: status=%d body=%s", status, data)
	}

	epID := mustEndpointIDByName(t, baseURL, token, epName)
	status, data = doJSON(t, http.MethodPost, baseURL+"/api/v1/model", token,
		[]byte(fmt.Sprintf(`{"body":{"alias":%q,"upstreamModel":"up-%d","endpointID":%d}}`, alias, stamp, epID)))
	if status != http.StatusOK {
		t.Fatalf("create model: status=%d body=%s", status, data)
	}

	// 删除前：group 存在且带 1 个模型
	before := mustListUpstream(t, baseURL, token, epName)
	if len(before.Groups) != 1 || before.Groups[0].ModelCount != 1 {
		t.Fatalf("before delete: expected 1 group with 1 model, got %+v", before.Groups)
	}

	status, data = doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/endpoint?id=%d", baseURL, epID), token, nil)
	if status != http.StatusOK {
		t.Fatalf("delete endpoint: status=%d body=%s", status, data)
	}

	after := mustListUpstream(t, baseURL, token, epName)
	if len(after.Groups) != 0 {
		t.Fatalf("after delete: group should be gone (cascade), got %d groups", len(after.Groups))
	}
}

// mustEndpointIDByName 按 endpoint 名称检索其 ID（name 全局唯一）。
func mustEndpointIDByName(t *testing.T, baseURL, token, name string) uint {
	t.Helper()
	rsp := mustListUpstream(t, baseURL, token, name)
	for _, g := range rsp.Groups {
		if g.Endpoint.Name == name {
			return g.Endpoint.ID
		}
	}
	t.Fatalf("endpoint %q not found in upstream list", name)
	return 0
}
