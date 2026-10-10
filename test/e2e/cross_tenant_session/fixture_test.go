// Package cross_tenant_session 验证会话/数据集归属的跨租户隔离。
//
// 缺陷背景（缺陷 A，此前零覆盖）：
// sessions 表没有 user_id / api_key_id 列，归属的唯一依据是 api_key_name；
// 而 proxy_api_keys 的唯一索引是 (user_id, name, deleted_at)，issue_api_key
// 只校验用户存在性，不禁止跨用户同名。LookupOwnerNamesByUserID 按 user_id
// 取名字列表，仓储侧过滤为 api_key_name IN (?)。
//
// 后果：任意普通用户创建一个与他人同名的 API Key，即可列出、读取全文、
// 删除、评分、公开分享、导出他人会话（共 7 类路径）。
//
// 本包在生产路由装配（RegisterAPIRouter + JWT 中间件 + sqlite 仓储 +
// miniredis）下端到端守护这 7 条路径。改按 api_key_id 判定后必须全绿。
//
// 串行约束：子测试共享同一 sqlite 内存库（cache=shared）与同一 fixture 数据，
// 并发写会触发 `database table is locked`，必须串行执行（沿用
// cross_tenant_reference 包的实测结论）。
//
//nolint:paralleltest // 共享 sqlite 内存库，子测试必须串行，原因见上
package cross_tenant_session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	datasetquery "github.com/hcd233/aris-proxy-api/internal/application/dataset/query"
	sessioncommand "github.com/hcd233/aris-proxy-api/internal/application/session/command"
	sessionquery "github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

// 嵌入接口空结构体：注册期满足方法集、不解引用（cross_tenant_reference 包惯例）
type (
	stubPingHandler       struct{ handler.PingHandler }
	stubTraceHandler      struct{ handler.TraceHandler }
	stubTokenHandler      struct{ handler.TokenHandler }
	stubOauth2Handler     struct{ handler.Oauth2Handler }
	stubUserHandler       struct{ handler.UserHandler }
	stubDemoHandler       struct{ handler.DemoHandler }
	stubAPIKeyHandler     struct{ handler.APIKeyHandler }
	stubEndpointHandler   struct{ handler.EndpointHandler }
	stubModelHandler      struct{ handler.ModelHandler }
	stubUpstreamHandler   struct{ handler.UpstreamHandler }
	stubAuditHandler      struct{ handler.AuditHandler }
	stubCronHandler       struct{ handler.CronHandler }
	stubTriggerHandler    struct{ handler.TriggerHandler }
	stubOpenAIHandler     struct{ handler.OpenAIHandler }
	stubPlaygroundHandler struct{ handler.PlaygroundHandler }
	stubAnthropicHandler  struct{ handler.AnthropicHandler }
	stubMetricsHandler    struct{ handler.MetricsHandler }
	stubClientHandler     struct{ handler.ClientHandler }
)

const e2eHTTPTimeout = 10 * time.Second

// 业务错误码，与 internal/common/ierr/sentinels.go 对齐
const (
	bizCodeNoPermission  int64 = 10002
	bizCodeDataNotExists int64 = 10003
)

// dbSeq 保证每次装配的内存库名唯一：cache=shared 的 sqlite 内存库按名字共享
// 生命周期，-count=N 复跑时复用同名库会读到上一轮残留（bind_id 唯一索引冲突）
var dbSeq atomic.Uint64

// crossTenantSessionFixture 真实装配：生产路由 + JWT + sqlite 仓储 + miniredis
type crossTenantSessionFixture struct {
	db     *gorm.DB
	app    *fiber.App
	signer jwt.TokenSigner

	userA *dbmodel.User
	userB *dbmodel.User
	admin *dbmodel.User

	keyA *dbmodel.ProxyAPIKey
	keyB *dbmodel.ProxyAPIKey

	// sessionA 归属 userA；sessionOrphan 归属为空（模拟 2026-07 前后存量）
	sessionA      *dbmodel.Session
	sessionOrphan *dbmodel.Session

	tokenA     string
	tokenB     string
	tokenAdmin string
}

// sharedKeyName 两个用户共用的 API Key 名称——缺陷 A 的攻击面。
const sharedKeyName = "shared-name"

func newCrossTenantSessionFixture(t *testing.T) *crossTenantSessionFixture {
	t.Helper()
	// config 的 JWT 过期时长默认值可能为 0（token 立即过期），测试环境显式设置
	config.JwtAccessTokenExpired = time.Hour
	config.JwtAccessTokenSecret = "cross-tenant-session-e2e-secret"

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dbName = fmt.Sprintf("%s-%d", dbName, dbSeq.Add(1))
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.User{}, &dbmodel.ProxyAPIKey{},
		&dbmodel.Session{}, &dbmodel.Message{}, &dbmodel.Tool{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	f := &crossTenantSessionFixture{db: db, signer: jwt.NewAccessTokenSigner()}
	f.seed(t)
	f.buildApp(t, db, rdb)

	f.tokenA = f.tokenFor(t, f.userA.ID)
	f.tokenB = f.tokenFor(t, f.userB.ID)
	f.tokenAdmin = f.tokenFor(t, f.admin.ID)
	return f
}

// seed 写入两个同名 Key 的普通用户、一个 admin，以及 A 名下会话与一条空归属会话。
func (f *crossTenantSessionFixture) seed(t *testing.T) {
	t.Helper()
	// github/google_bind_id 各自参与 (bind_id, deleted_at) 唯一索引，零值彼此冲突，须逐个区分
	f.userA = &dbmodel.User{Name: "tenant-a", GithubBindID: "gh-a", GoogleBindID: "gg-a", Permission: enum.PermissionUser}
	f.userB = &dbmodel.User{Name: "tenant-b", GithubBindID: "gh-b", GoogleBindID: "gg-b", Permission: enum.PermissionUser}
	f.admin = &dbmodel.User{Name: "root", GithubBindID: "gh-root", GoogleBindID: "gg-root", Permission: enum.PermissionAdmin}
	for _, u := range []*dbmodel.User{f.userA, f.userB, f.admin} {
		if err := f.db.Create(u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}

	// 关键：两个用户各有一个同名 Key。唯一索引是 (user_id, name)，故这是合法数据。
	f.keyA = &dbmodel.ProxyAPIKey{UserID: f.userA.ID, Name: sharedKeyName, Key: "sk-tenant-a"}
	f.keyB = &dbmodel.ProxyAPIKey{UserID: f.userB.ID, Name: sharedKeyName, Key: "sk-tenant-b"}
	for _, k := range []*dbmodel.ProxyAPIKey{f.keyA, f.keyB} {
		if err := f.db.Create(k).Error; err != nil {
			t.Fatalf("create api key: %v", err)
		}
	}

	msg := &dbmodel.Message{ModelID: "gpt-secret", CheckSum: "ck-secret-msg"}
	if err := f.db.Create(msg).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}

	score := 5
	f.sessionA = &dbmodel.Session{
		APIKeyName: sharedKeyName,
		APIKeyID:   f.keyA.ID,
		MessageIDs: []uint{msg.ID},
		ToolIDs:    []uint{},
		Questions:  []uint{msg.ID},
		ModelIDs:   []string{"gpt-secret"},
		Score:      &score,
	}
	// 空归属会话：api_key_name='' 且 api_key_id=0，仅 admin 可见
	f.sessionOrphan = &dbmodel.Session{
		APIKeyName: "",
		APIKeyID:   0,
		MessageIDs: []uint{msg.ID},
		ToolIDs:    []uint{},
		Questions:  []uint{msg.ID},
		ModelIDs:   []string{"gpt-secret"},
	}
	for _, s := range []*dbmodel.Session{f.sessionA, f.sessionOrphan} {
		if err := f.db.Create(s).Error; err != nil {
			t.Fatalf("create session: %v", err)
		}
	}
}

// buildApp 用生产路由注册装配真实 session 与 dataset handler，其余 handler 用 stub。
func (f *crossTenantSessionFixture) buildApp(t *testing.T, db *gorm.DB, rdb *redis.Client) {
	t.Helper()

	sessionRepo := repository.NewSessionRepository(db)
	readRepo := repository.NewSessionReadRepository(db)
	apiKeyRepo := repository.NewAPIKeyRepository(db)

	shareCache := cache.NewShareCache(rdb)
	detailCache := cache.NewSessionDetailCache(rdb)

	getByUser := sessionquery.NewGetSessionByUserHandler(readRepo, apiKeyRepo)
	getMetaByUser := sessionquery.NewGetSessionMetaByUserHandler(readRepo, apiKeyRepo, detailCache)

	sessionHandler := handler.NewSessionHandler(handler.SessionDependencies{
		ListByUser:         sessionquery.NewListSessionsByUserHandler(readRepo, apiKeyRepo),
		GetByUser:          getByUser,
		ShareCache:         shareCache,
		CreateShare:        sessioncommand.NewCreateShareHandler(getByUser, shareCache),
		GetMetaByUser:      getMetaByUser,
		ListMessages:       sessionquery.NewListSessionMessagesHandler(readRepo, getMetaByUser, detailCache),
		ListTools:          sessionquery.NewListSessionToolsHandler(readRepo, getMetaByUser, detailCache),
		DeleteSession:      sessioncommand.NewDeleteSessionHandler(sessionRepo, apiKeyRepo),
		ScoreSession:       sessioncommand.NewScoreSessionHandler(sessionRepo, apiKeyRepo),
		DeleteScoreSession: sessioncommand.NewDeleteScoreSessionHandler(sessionRepo, apiKeyRepo),
		SessionCache:       detailCache,
		ListOption:         sessionquery.NewListSessionOptionHandler(readRepo, apiKeyRepo),
	})

	datasetHandler := handler.NewDatasetHandler(handler.DatasetDependencies{
		Preview: datasetquery.NewPreviewDatasetHandler(readRepo, apiKeyRepo),
		Export:  datasetquery.NewExportDatasetHandler(readRepo, apiKeyRepo),
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("cross tenant session", "1.0"))
	router.RegisterAPIRouter(api, router.APIRouterDependencies{
		DB:                db,
		Cache:             rdb,
		AccessSigner:      jwt.NewAccessTokenSigner(),
		PingHandler:       &stubPingHandler{},
		TraceHandler:      &stubTraceHandler{},
		TokenHandler:      &stubTokenHandler{},
		Oauth2Handler:     &stubOauth2Handler{},
		UserHandler:       &stubUserHandler{},
		DemoHandler:       &stubDemoHandler{},
		APIKeyHandler:     &stubAPIKeyHandler{},
		SessionHandler:    sessionHandler,
		EndpointHandler:   &stubEndpointHandler{},
		ModelHandler:      &stubModelHandler{},
		UpstreamHandler:   &stubUpstreamHandler{},
		AuditHandler:      &stubAuditHandler{},
		CronHandler:       &stubCronHandler{},
		TriggerHandler:    &stubTriggerHandler{},
		OpenAIHandler:     &stubOpenAIHandler{},
		PlaygroundHandler: &stubPlaygroundHandler{},
		AnthropicHandler:  &stubAnthropicHandler{},
		MetricsHandler:    &stubMetricsHandler{},
		DatasetHandler:    datasetHandler,
		ClientHandler:     &stubClientHandler{},
	})
	f.app = app
}

func (f *crossTenantSessionFixture) tokenFor(t *testing.T, userID uint) string {
	t.Helper()
	token, err := f.signer.EncodeToken(userID)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	return token
}

// do 经真实 fiber app 发起请求（fiber v3 App 未实现 http.Handler，用 app.Test 直连）。
func (f *crossTenantSessionFixture) do(t *testing.T, method, path, token, body string) (status int, data []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), e2eHTTPTimeout)
	defer cancel()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://e2e.local"+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	resp, err := f.app.Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, data
}

// bizErrorBody 统一响应信封的错误体（管理 API 恒 HTTP 200，错误语义由 error.code 承载）
type bizErrorBody struct {
	Error *struct {
		Code    int64  `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// bizErrorCode 提取响应体中的业务错误码；无错误时返回 0
func bizErrorCode(t *testing.T, data []byte) int64 {
	t.Helper()
	var body bizErrorBody
	if err := sonic.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode response body: %v (raw=%s)", err, data)
	}
	if body.Error == nil {
		return 0
	}
	return body.Error.Code
}

// requireRejected 断言请求被归属守卫拒绝：HTTP 200 信封 + 「无权限」或「不存在」业务码。
//
// 两个码都接受：详情/元数据走 ErrNoPermission，trace/demo 风格路径走
// ErrDataNotExists（防遍历）。但**不接受 0（成功）与其它错误码**——
// 后者说明没走到守卫（如 DB 错误兜底），会掩盖真实缺陷。
func requireRejected(t *testing.T, status int, data []byte, what string) {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("%s: expect HTTP 200 envelope, got status=%d body=%s", what, status, data)
	}
	got := bizErrorCode(t, data)
	if got != bizCodeNoPermission && got != bizCodeDataNotExists {
		t.Fatalf("%s: expect reject biz code %d or %d, got %d body=%s",
			what, bizCodeNoPermission, bizCodeDataNotExists, got, data)
	}
}
