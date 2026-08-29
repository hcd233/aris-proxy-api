// Package model_list 验证 GET /api/web/v1/model/list 平铺模型列表的全链路行为。
//
// 需求背景（feature/upstream-list-redesign）：
//   - 平铺视图有独立于分组视图的真分页：pageInfo.total=模型数（非端点数）；
//   - 每行嵌套 endpoint{id,name} 与 user{id,name,avatar}；
//   - status / capability / endpointID 三个筛选维度；
//   - 排序列走白名单，非法值回退默认列且不报错；
//   - 普通用户 scope 隔离。
//
// 离线部分：路由装配回归（走生产入口 router.RegisterAPIRouter，CI 中生效）。
// 在线部分：由 BASE_URL / JWT_TOKEN 环境变量门控，命中真实环境。
package model_list

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/router"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

const (
	e2eHTTPTimeout = 30 * time.Second
	// webModelListPath 管理端平铺模型列表路径（与 router 中 modelGroup 挂载保持一致）
	webModelListPath = "/api/web/v1/model/list"
)

// ───────────────────────── 离线：路由装配回归 ─────────────────────────

// RegisterAPIRouter 只为注册路由而持有各 handler 的方法值，方法本身不会被调用。
// 用「嵌入接口」的空结构体即可满足所有接口；注册期不会解引用，
// 只有真正请求某条路由时才会 panic。
type (
	stubPingHandler      struct{ handler.PingHandler }
	stubTraceHandler     struct{ handler.TraceHandler }
	stubTokenHandler     struct{ handler.TokenHandler }
	stubOauth2Handler    struct{ handler.Oauth2Handler }
	stubUserHandler      struct{ handler.UserHandler }
	stubDemoHandler      struct{ handler.DemoHandler }
	stubAPIKeyHandler    struct{ handler.APIKeyHandler }
	stubSessionHandler   struct{ handler.SessionHandler }
	stubEndpointHandler  struct{ handler.EndpointHandler }
	stubModelHandler     struct{ handler.ModelHandler }
	stubUpstreamHandler  struct{ handler.UpstreamHandler }
	stubAuditHandler     struct{ handler.AuditHandler }
	stubCronHandler      struct{ handler.CronHandler }
	stubTriggerHandler   struct{ handler.TriggerHandler }
	stubOpenAIHandler    struct{ handler.OpenAIHandler }
	stubAnthropicHandler struct{ handler.AnthropicHandler }
	stubMetricsHandler   struct{ handler.MetricsHandler }
	stubDatasetHandler   struct{ handler.DatasetHandler }
	stubClientHandler    struct{ handler.ClientHandler }
)

func newRouteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.ProxyAPIKey{}, &dbmodel.User{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

// TestModelListRouteRegistered 平铺列表路径必须出现在服务端真实注册的路由表里，且受保护。
//
// 直接调用生产装配入口 router.RegisterAPIRouter（而非复刻分组），因此 group 前缀
// 的任何改动都会被捕获。
//
// 判定方式：无凭据请求被 JWT 中间件拦在 handler 之前，响应是 HTTP 200 +
// body {"error":{"code":10001,"message":"Unauthorized"}}——**本项目的鉴权失败统一走
// 业务错误码而非 HTTP 401**（前端 api-client 对 code=10001 抛 ApiError(401)）。
// 而路由未注册时是 HTTP 404 + 纯文本 "Not Found"。二者可据此区分。
//
// 另注：本路由的 OperationID 是 listWebModels 而非 listModels——后者已被 OpenAI
// 兼容路由 /models 占用，重复会让 huma 直接 panic。
func TestModelListRouteRegistered(t *testing.T) {
	t.Parallel()
	db := newRouteTestDB(t)

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("model list route", "1.0"))
	router.RegisterAPIRouter(api, router.APIRouterDependencies{
		DB:               db,
		PingHandler:      &stubPingHandler{},
		TraceHandler:     &stubTraceHandler{},
		TokenHandler:     &stubTokenHandler{},
		Oauth2Handler:    &stubOauth2Handler{},
		UserHandler:      &stubUserHandler{},
		DemoHandler:      &stubDemoHandler{},
		APIKeyHandler:    &stubAPIKeyHandler{},
		SessionHandler:   &stubSessionHandler{},
		EndpointHandler:  &stubEndpointHandler{},
		ModelHandler:     &stubModelHandler{},
		UpstreamHandler:  &stubUpstreamHandler{},
		AuditHandler:     &stubAuditHandler{},
		CronHandler:      &stubCronHandler{},
		TriggerHandler:   &stubTriggerHandler{},
		OpenAIHandler:    &stubOpenAIHandler{},
		AnthropicHandler: &stubAnthropicHandler{},
		MetricsHandler:   &stubMetricsHandler{},
		DatasetHandler:   &stubDatasetHandler{},
		ClientHandler:    &stubClientHandler{},
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		webModelListPath+"?page=1&pageSize=20", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	// 404 = 路由未注册（此时响应是纯文本 "Not Found"，不是 JSON）
	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("%s is not registered by RegisterAPIRouter (got 404)", webModelListPath)
	}
	var rsp listModelsRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("%s returned non-JSON body %q: %v", webModelListPath, body, err)
	}
	if rsp.Error == nil || rsp.Error.Code != constant.BizErrorCodeUnauthorized {
		t.Fatalf("%s should reject unauthenticated requests with biz code %q, got %+v",
			webModelListPath, constant.BizErrorCodeUnauthorized, rsp.Error)
	}
	// 受保护意味着 handler 未被执行：不得返回任何模型行
	if len(rsp.Items) != 0 {
		t.Fatalf("unauthenticated request must not reach the handler, got %d items", len(rsp.Items))
	}
}

// ───────────────────────── 在线：env 门控 ─────────────────────────

// bizError 业务错误体。Code 是数值（实测响应为 {"code":10001,...}）。
type bizError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type modelListUser struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// modelListEndpoint 平铺行的嵌套端点：只暴露 id/name，不得含 baseURL / apiKey。
type modelListEndpoint struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type modelListItem struct {
	ID              uint               `json:"id"`
	User            *modelListUser     `json:"user,omitempty"`
	Endpoint        *modelListEndpoint `json:"endpoint,omitempty"`
	Alias           string             `json:"alias"`
	ModelID         string             `json:"modelId"`
	UpstreamModel   string             `json:"upstreamModel"`
	Enabled         bool               `json:"enabled"`
	ContextLength   int                `json:"contextLength"`
	MaxOutputTokens int                `json:"maxOutputTokens"`
	Capabilities    []string           `json:"capabilities"`
}

type pageInfo struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type listModelsRsp struct {
	Items    []*modelListItem `json:"items"`
	PageInfo *pageInfo        `json:"pageInfo,omitempty"`
	Error    *bizError        `json:"error,omitempty"`
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
	if token != "" {
		req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+" "+token)
	}
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, url, err)
	}
	defer rsp.Body.Close()
	data, err = io.ReadAll(rsp.Body)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	status = rsp.StatusCode
	return status, data
}

func mustListModels(t *testing.T, baseURL, token, query string) listModelsRsp {
	t.Helper()
	status, data := doJSON(t, http.MethodGet, baseURL+webModelListPath+"?page=1&pageSize=100"+query, token, nil)
	if status != http.StatusOK {
		t.Fatalf("list models: status=%d body=%s", status, data)
	}
	var rsp listModelsRsp
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("unmarshal model list rsp: %v", err)
	}
	if rsp.Error != nil {
		t.Fatalf("list models biz error: %+v", rsp.Error)
	}
	return rsp
}

// setupFixtures 建 1 端点 + 3 模型（a: text 启用 / b: text+image 启用 / c: text 停用），
// 返回端点名与三个别名，并注册清理。
func setupFixtures(t *testing.T, baseURL, token string) (epName string, aliases [3]string) {
	t.Helper()
	stamp := time.Now().UnixNano()
	epName = fmt.Sprintf("e2e-mdl-ep-%d", stamp)
	aliases = [3]string{
		fmt.Sprintf("e2e-mdl-a-%d", stamp),
		fmt.Sprintf("e2e-mdl-b-%d", stamp),
		fmt.Sprintf("e2e-mdl-c-%d", stamp),
	}

	status, data := doJSON(t, http.MethodPost, baseURL+"/api/web/v1/endpoint", token,
		[]byte(fmt.Sprintf(`{"name":%q,"apiKey":"sk-e2e","openaiBaseURL":"https://o.example.com/v1","supportOpenAIChatCompletion":true}`, epName)))
	if status != http.StatusOK {
		t.Fatalf("create endpoint: status=%d body=%s", status, data)
	}
	t.Cleanup(func() {
		for _, g := range listEndpointsByName(t, baseURL, token, epName) {
			doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/web/v1/endpoint?id=%d", baseURL, g.ID), token, nil)
		}
	})

	// 用平铺列表自身定位端点 ID（同时验证新接口可用于寻址）
	var epID uint
	found := false
	for _, it := range mustListModels(t, baseURL, token, "").Items {
		if it.Endpoint != nil && it.Endpoint.Name == epName {
			epID, found = it.Endpoint.ID, true
			break
		}
	}
	if !found {
		// 端点刚建还没有模型，退回 upstream 分组列表拿 ID
		epID = fallbackEndpointID(t, baseURL, token, epName)
	}

	// a: text 启用
	status, data = doJSON(t, http.MethodPost, baseURL+"/api/web/v1/model", token,
		[]byte(fmt.Sprintf(`{"alias":%q,"upstreamModel":"up-a","endpointID":%d,"capabilities":["text"]}`, aliases[0], epID)))
	if status != http.StatusOK {
		t.Fatalf("create model a: status=%d body=%s", status, data)
	}
	// b: text+image 启用
	status, data = doJSON(t, http.MethodPost, baseURL+"/api/web/v1/model", token,
		[]byte(fmt.Sprintf(`{"alias":%q,"upstreamModel":"up-b","endpointID":%d,"capabilities":["text","image"]}`, aliases[1], epID)))
	if status != http.StatusOK {
		t.Fatalf("create model b: status=%d body=%s", status, data)
	}
	// c: text 启用后停用
	status, data = doJSON(t, http.MethodPost, baseURL+"/api/web/v1/model", token,
		[]byte(fmt.Sprintf(`{"alias":%q,"upstreamModel":"up-c","endpointID":%d,"capabilities":["text"]}`, aliases[2], epID)))
	if status != http.StatusOK {
		t.Fatalf("create model c: status=%d body=%s", status, data)
	}
	idC := modelIDByAlias(t, baseURL, token, epName, aliases[2])
	status, data = doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/web/v1/model?id=%d", baseURL, idC), token,
		[]byte(`{"enabled":false}`))
	if status != http.StatusOK {
		t.Fatalf("disable model c: status=%d body=%s", status, data)
	}

	return epName, aliases
}

type endpointRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type upstreamRsp struct {
	Groups []struct {
		Endpoint endpointRef `json:"endpoint"`
	} `json:"groups"`
	Error *bizError `json:"error,omitempty"`
}

func listEndpointsByName(t *testing.T, baseURL, token, name string) []endpointRef {
	t.Helper()
	status, data := doJSON(t, http.MethodGet,
		baseURL+"/api/web/v1/upstream/list?page=1&pageSize=100&query="+name, token, nil)
	if status != http.StatusOK {
		return nil
	}
	var rsp upstreamRsp
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		return nil
	}
	out := make([]endpointRef, 0, len(rsp.Groups))
	for _, g := range rsp.Groups {
		if g.Endpoint.Name == name {
			out = append(out, g.Endpoint)
		}
	}
	return out
}

func fallbackEndpointID(t *testing.T, baseURL, token, name string) uint {
	t.Helper()
	refs := listEndpointsByName(t, baseURL, token, name)
	if len(refs) == 0 {
		t.Fatalf("endpoint %q not found after create", name)
	}
	return refs[0].ID
}

func modelIDByAlias(t *testing.T, baseURL, token, epName, alias string) uint {
	t.Helper()
	for _, it := range mustListModels(t, baseURL, token, "").Items {
		if it.Alias == alias && it.Endpoint != nil && it.Endpoint.Name == epName {
			return it.ID
		}
	}
	t.Fatalf("model alias %q not found", alias)
	return 0
}

// TestModelList_PaginationAndNestedFields 平铺视图的 total 是模型数，每行嵌套 endpoint 与 user。
func TestModelList_PaginationAndNestedFields(t *testing.T) {
	t.Parallel()
	baseURL, token := mustE2EEnv(t)
	epName, aliases := setupFixtures(t, baseURL, token)

	rsp := mustListModels(t, baseURL, token, "")
	if rsp.PageInfo == nil || rsp.PageInfo.Total < 3 {
		t.Fatalf("pageInfo.total should count models (>=3), got %+v", rsp.PageInfo)
	}
	if len(rsp.Items) == 0 {
		t.Fatal("expected non-empty items")
	}

	byAlias := map[string]*modelListItem{}
	for _, it := range rsp.Items {
		byAlias[it.Alias] = it
	}
	for _, a := range aliases {
		it, ok := byAlias[a]
		if !ok {
			t.Fatalf("alias %q missing from flat list", a)
		}
		if it.Endpoint == nil || it.Endpoint.Name != epName {
			t.Fatalf("alias %q: endpoint should be nested with name %q, got %+v", a, epName, it.Endpoint)
		}
		if it.User == nil || it.User.Name == "" {
			t.Fatalf("alias %q: user should be nested, got %+v", a, it.User)
		}
	}
}

// TestModelList_StatusAndCapabilityFilters status 与 capability 两个筛选维度生效。
func TestModelList_StatusAndCapabilityFilters(t *testing.T) {
	t.Parallel()
	baseURL, token := mustE2EEnv(t)
	_, aliases := setupFixtures(t, baseURL, token)

	// status=disabled：只回停用模型（c）
	disabled := mustListModels(t, baseURL, token, "&status=disabled")
	for _, it := range disabled.Items {
		if it.Enabled {
			t.Fatalf("status=disabled returned enabled model %q", it.Alias)
		}
	}
	foundC := false
	for _, it := range disabled.Items {
		if it.Alias == aliases[2] {
			foundC = true
		}
	}
	if !foundC {
		t.Fatalf("status=disabled should include %q", aliases[2])
	}

	// status=enabled：不含 c
	for _, it := range mustListModels(t, baseURL, token, "&status=enabled").Items {
		if it.Alias == aliases[2] {
			t.Fatalf("status=enabled should exclude disabled model %q", aliases[2])
		}
	}

	// capability=image：只回含 image 的模型（b）
	imageOnly := mustListModels(t, baseURL, token, "&capability=image")
	if len(imageOnly.Items) == 0 {
		t.Fatal("capability=image returned nothing")
	}
	for _, it := range imageOnly.Items {
		hasImage := false
		for _, c := range it.Capabilities {
			if c == "image" {
				hasImage = true
			}
		}
		if !hasImage {
			t.Fatalf("capability=image returned model without image: %q (%v)", it.Alias, it.Capabilities)
		}
	}
}

// TestModelList_SortFieldWhitelistFallsBack 非法排序列回退默认列，不得 500。
//
// api_key 属于合法字符集但不在白名单内——若只做字符集校验（util.SafeSortField）
// 就会进入 ORDER BY 并在 Postgres 上报 undefined column。
func TestModelList_SortFieldWhitelistFallsBack(t *testing.T) {
	t.Parallel()
	baseURL, token := mustE2EEnv(t)
	_, aliases := setupFixtures(t, baseURL, token)

	status, data := doJSON(t, http.MethodGet,
		baseURL+webModelListPath+"?page=1&pageSize=100&sortField=api_key&sort=asc", token, nil)
	if status != http.StatusOK {
		t.Fatalf("illegal sortField must fall back, got status=%d body=%s", status, data)
	}
	var rsp listModelsRsp
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rsp.Error != nil {
		t.Fatalf("illegal sortField must not surface biz error: %+v", rsp.Error)
	}
	// 回退排序后数据仍应完整返回
	found := 0
	for _, it := range rsp.Items {
		for _, a := range aliases {
			if it.Alias == a {
				found++
			}
		}
	}
	if found != len(aliases) {
		t.Fatalf("expected all %d fixtures after fallback, got %d", len(aliases), found)
	}
}

// TestModelList_ScopeIsolation 非 admin 用户的 HTTP 层 scope：只回自己名下模型。
//
// admin token 建的 fixture 归属 admin 账户；user 权限 token 的列表里出现任何一个
// 都意味着 scope 隔离在 handler→repo 链路上被击穿。仓储层口径由
// test/unit/model_list_repo 覆盖，本用例补 HTTP 层（scopePtrFor→repo 全链路）。
// 门控 E2E_USER_TOKEN（user 权限 JWT，缺省跳过，不影响无凭据 CI）。
func TestModelList_ScopeIsolation(t *testing.T) {
	t.Parallel()
	baseURL, adminToken := mustE2EEnv(t)
	userToken := os.Getenv("E2E_USER_TOKEN")
	if userToken == "" {
		t.Skip("E2E_USER_TOKEN not set, skip scope isolation e2e")
	}
	_, aliases := setupFixtures(t, baseURL, adminToken)

	for _, it := range mustListModels(t, baseURL, userToken, "").Items {
		for _, a := range aliases {
			if it.Alias == a {
				t.Fatalf("scope leak: admin-owned model %q visible to user token", a)
			}
		}
	}
}

// TestModelList_RequiresAuth 未认证请求不得看到任何模型。
//
// 本项目鉴权失败走 HTTP 200 + 业务错误码 10001（非 HTTP 401），故断言错误码而非状态码。
func TestModelList_RequiresAuth(t *testing.T) {
	t.Parallel()
	baseURL, _ := mustE2EEnv(t)

	status, data := doJSON(t, http.MethodGet, baseURL+webModelListPath+"?page=1&pageSize=20", "", nil)
	var rsp listModelsRsp
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("unmarshal rsp (status=%d body=%s): %v", status, data, err)
	}
	if rsp.Error == nil || rsp.Error.Code != constant.BizErrorCodeUnauthorized {
		t.Fatalf("unauthenticated list must be rejected with code %d, got status=%d err=%+v",
			constant.BizErrorCodeUnauthorized, status, rsp.Error)
	}
	if len(rsp.Items) != 0 {
		t.Fatalf("unauthenticated list must not return rows, got %d", len(rsp.Items))
	}
}
