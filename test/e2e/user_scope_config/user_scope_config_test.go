// Package user_scope_config 验证用户级 endpoint/model 配置的管理链路。
//
// 需求背景（feature/user-level-model-endpoint-multitenancy）：
//   - endpoint/model 管理接口下放至 PermissionUser，普通用户只能操作自己的配置；
//   - admin list 默认返回全量，支持按 username 过滤（用户不存在时返回空列表而非报错）；
//   - 响应项携带归属 username；
//   - model 归属从其 endpoint 继承（跨租户校验在应用层完成，单测覆盖）。
//
// 环境变量：
//   - BASE_URL   API 根地址（必填）
//   - JWT_TOKEN  管理员 JWT（调用配置管理接口）
package user_scope_config

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/test/e2e/e2eguard"
)

const e2eHTTPTimeout = 30 * time.Second

type bizError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

type upstreamGroupItem struct {
	Endpoint struct {
		ID   uint `json:"id"`
		User *struct {
			Name string `json:"name"`
		} `json:"user"`
		Name string `json:"name"`
	} `json:"endpoint"`
	Models []struct {
		Alias string `json:"alias"`
	} `json:"models"`
	ModelCount int `json:"modelCount"`
}

type listUpstreamRsp struct {
	Groups     []upstreamGroupItem `json:"groups"`
	ModelTotal int64               `json:"modelTotal"`
	Error      *bizError           `json:"error,omitempty"`
}

type commandRsp struct {
	Error *bizError `json:"error,omitempty"`
}

type currentUserRsp struct {
	User *struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	} `json:"user,omitempty"`
	Error *bizError `json:"error,omitempty"`
}

// currentUser 动态获取当前 JWT 归属用户（id + name），替代硬编码 ownerUserID=1 / "admin"
func currentUser(t *testing.T, baseURL, token string) (id uint, name string) {
	t.Helper()
	status, data := doJSON(t, http.MethodGet, baseURL+"/api/web/v1/user/current", token, nil)
	if status != http.StatusOK {
		t.Fatalf("get current user: status=%d body=%s", status, data)
	}
	var rsp currentUserRsp
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("unmarshal current user rsp: %v", err)
	}
	if rsp.Error != nil {
		t.Fatalf("current user biz error: %+v", rsp.Error)
	}
	if rsp.User == nil || rsp.User.ID == 0 || rsp.User.Name == "" {
		t.Fatalf("current user rsp missing id/name: %s", data)
	}
	return rsp.User.ID, rsp.User.Name
}

func mustE2EEnv(t *testing.T) (baseURL, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || jwtToken == "" {
		t.Skip("BASE_URL or JWT_TOKEN not set, skip e2e")
	}
	// 本用例会真实增删 endpoint/model 配置，拒绝误打生产
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
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
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

// TestUserScope_ConfigLifecycle 覆盖 admin 代建、username 过滤、空结果与清理全链路。
func TestUserScope_ConfigLifecycle(t *testing.T) {
	t.Parallel()
	baseURL, token := mustE2EEnv(t)

	stamp := time.Now().Unix()
	epName := fmt.Sprintf("e2e-uscope-ep-%d", stamp)
	alias := fmt.Sprintf("e2e-uscope-m-%d", stamp)

	// 归属用户从当前用户接口动态获取（硬编码 admin ID=1 会在其它环境误挂到别的用户）
	ownerUserID, ownerName := currentUser(t, baseURL, token)

	adminName := os.Getenv("ADMIN_USERNAME")
	if adminName == "" {
		adminName = ownerName
	}

	// 1. admin 代建 endpoint（ownerUserID 指定自身）
	createBody := fmt.Sprintf(`{"ownerUserID":%d,"name":%q,"apiKey":"sk-e2e","openaiBaseURL":"https://o.example.com/v1","supportOpenAIChatCompletion":true}`, ownerUserID, epName)
	status, data := doJSON(t, http.MethodPost, baseURL+"/api/web/v1/endpoint", token, []byte(createBody))
	if status != http.StatusOK {
		t.Fatalf("create endpoint: status=%d body=%s", status, data)
	}
	var created commandRsp
	if err := sonic.Unmarshal(data, &created); err != nil {
		t.Fatalf("unmarshal create rsp: %v", err)
	}
	if created.Error != nil {
		t.Fatalf("create endpoint biz error: %+v", created.Error)
	}
	cleanup := func() {
		// 按 name 找到 id 后删除
		_, listData := doJSON(t, http.MethodGet, baseURL+"/api/web/v1/upstream/list?page=1&pageSize=100&query="+epName, token, nil)
		var list listUpstreamRsp
		if err := sonic.Unmarshal(listData, &list); err != nil {
			return
		}
		for _, g := range list.Groups {
			ep := g.Endpoint
			if ep.Name != epName {
				continue
			}
			// 删除该端点下的 models（cascade 由后端处理）
			doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/web/v1/endpoint?id=%d", baseURL, ep.ID), token, nil)
		}
	}
	t.Cleanup(cleanup)

	// 2. username 过滤：命中 admin 名下的端点且响应带归属用户名
	status, data = doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/web/v1/upstream/list?page=1&pageSize=100&query=%s&username=%s", baseURL, epName, adminName), token, nil)
	if status != http.StatusOK {
		t.Fatalf("list upstream by username: status=%d body=%s", status, data)
	}
	var listRsp listUpstreamRsp
	if err := sonic.Unmarshal(data, &listRsp); err != nil {
		t.Fatalf("unmarshal upstream rsp: %v", err)
	}
	if listRsp.Error != nil {
		t.Fatalf("list upstream biz error: %+v", listRsp.Error)
	}
	var found *upstreamGroupItem
	for i := range listRsp.Groups {
		if listRsp.Groups[i].Endpoint.Name == epName {
			found = &listRsp.Groups[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("created endpoint not found under username=%q", adminName)
	}
	// 响应项携带归属 username 是被测点，必须断言失败而非告警
	if found.Endpoint.User == nil {
		t.Errorf("endpoint item must carry owner user, got user=nil (endpointID=%d)", found.Endpoint.ID)
	} else if !strings.EqualFold(found.Endpoint.User.Name, adminName) {
		t.Errorf("endpoint.user.name = %q, want %q", found.Endpoint.User.Name, adminName)
	}
	endpointID := found.Endpoint.ID

	// 3. 不存在的 username → 空列表而非错误
	status, data = doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/web/v1/upstream/list?page=1&pageSize=20&query=%s&username=%s", baseURL, epName, "no-such-user-xyz-"+strconv.FormatInt(stamp, 10)), token, nil)
	if status != http.StatusOK {
		t.Fatalf("list with unknown username: status=%d body=%s", status, data)
	}
	var emptyList listUpstreamRsp
	if err := sonic.Unmarshal(data, &emptyList); err != nil {
		t.Fatalf("unmarshal empty upstream rsp: %v", err)
	}
	if emptyList.Error != nil {
		t.Fatalf("unknown username should return empty groups, got error: %+v", emptyList.Error)
	}
	if len(emptyList.Groups) != 0 {
		t.Fatalf("unknown username should yield 0 endpoint groups, got %d", len(emptyList.Groups))
	}

	// 4. 在该端点上创建 model
	modelBody := fmt.Sprintf(`{"alias":%q,"upstreamModel":"upstream-%d","endpointID":%d}`, alias, stamp, endpointID)
	status, data = doJSON(t, http.MethodPost, baseURL+"/api/web/v1/model", token, []byte(modelBody))
	if status != http.StatusOK {
		t.Fatalf("create model: status=%d body=%s", status, data)
	}

	// 5. username 过滤可见该 model
	status, data = doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/web/v1/upstream/list?page=1&pageSize=100&query=%s&username=%s", baseURL, alias, adminName), token, nil)
	if status != http.StatusOK {
		t.Fatalf("list models via upstream by username: status=%d body=%s", status, data)
	}
	var modelsRsp listUpstreamRsp
	if err := sonic.Unmarshal(data, &modelsRsp); err != nil {
		t.Fatalf("unmarshal upstream rsp for model: %v", err)
	}
	var foundModel bool
	for _, g := range modelsRsp.Groups {
		for _, m := range g.Models {
			if m.Alias == alias {
				foundModel = true
				break
			}
		}
	}
	if !foundModel {
		t.Fatalf("created model alias %q not visible under username=%q", alias, adminName)
	}
}
