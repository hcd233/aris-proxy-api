// sync_history_test.go 验证 modelId 历史记录一键同步（syncHistory）的全链路行为：
//
//   - modelId 变化 + syncHistory=true：该模型归属用户名下 audit/session/message 三表
//     历史数据中的旧 model id 被批量替换，响应返回各表影响行数；
//   - 租户隔离：另一用户同名字符串的历史数据不受影响；
//   - modelId 未变 + syncHistory=true：幂等，三计数为 0；
//   - modelId 变化但未带 syncHistory：历史数据保持旧值。
//
// 装配方式复刻 test/e2e/cross_tenant_reference（生产路由 + JWT + sqlite + miniredis）。
package model_id

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	endpointcommand "github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	modelcommand "github.com/hcd233/aris-proxy-api/internal/application/model/command"
	upstreamquery "github.com/hcd233/aris-proxy-api/internal/application/upstream/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

// syncHistoryFixture 真实装配：生产路由 + JWT + sqlite 仓储 + miniredis
type syncHistoryFixture struct {
	app    *fiber.App
	db     *gorm.DB
	signer jwt.TokenSigner
	userA  *dbmodel.User
	userB  *dbmodel.User
	epA    *dbmodel.Endpoint
	epB    *dbmodel.Endpoint
	modelA *dbmodel.Model
	keyA   *dbmodel.ProxyAPIKey
	keyB   *dbmodel.ProxyAPIKey
	msgA   *dbmodel.Message // A 会话引用的消息
	msgB   *dbmodel.Message // B 的独立消息（不被 A 会话引用，scope 外）
	sessA  *dbmodel.Session
}

// stubSyncHandlers 嵌入接口空结构体：注册期满足方法集、不解引用（cross_tenant 惯例）
type (
	stubSyncPingHandler      struct{ handler.PingHandler }
	stubSyncTraceHandler     struct{ handler.TraceHandler }
	stubSyncTokenHandler     struct{ handler.TokenHandler }
	stubSyncOauth2Handler    struct{ handler.Oauth2Handler }
	stubSyncUserHandler      struct{ handler.UserHandler }
	stubSyncDemoHandler      struct{ handler.DemoHandler }
	stubSyncAPIKeyHandler    struct{ handler.APIKeyHandler }
	stubSyncSessionHandler   struct{ handler.SessionHandler }
	stubSyncAuditHandler     struct{ handler.AuditHandler }
	stubSyncCronHandler      struct{ handler.CronHandler }
	stubSyncTriggerHandler   struct{ handler.TriggerHandler }
	stubSyncOpenAIHandler    struct{ handler.OpenAIHandler }
	stubSyncAnthropicHandler struct{ handler.AnthropicHandler }
	stubSyncMetricsHandler   struct{ handler.MetricsHandler }
	stubSyncDatasetHandler   struct{ handler.DatasetHandler }
	stubSyncClientHandler    struct{ handler.ClientHandler }
)

func newSyncHistoryFixture(t *testing.T) *syncHistoryFixture {
	t.Helper()
	config.JwtAccessTokenExpired = time.Hour
	config.JwtAccessTokenSecret = "sync-history-e2e-secret"

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&dbmodel.Endpoint{}, &dbmodel.Model{}, &dbmodel.User{},
		&dbmodel.ProxyAPIKey{}, &dbmodel.ModelCallAudit{}, &dbmodel.Session{}, &dbmodel.Message{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	endpointRepo := repository.NewEndpointRepository(db)
	modelRepo := repository.NewModelRepository(db)
	userRepo := repository.NewUserRepository(db)

	endpointHandler := handler.NewEndpointHandler(handler.EndpointDependencies{
		Create: endpointcommand.NewCreateEndpointHandler(endpointRepo),
		Update: endpointcommand.NewUpdateEndpointHandler(endpointRepo),
		Delete: endpointcommand.NewDeleteEndpointHandler(endpointRepo),
	})
	modelHandler := handler.NewModelHandler(handler.ModelDependencies{
		Create: modelcommand.NewCreateModelHandler(endpointRepo, modelRepo),
		Update: modelcommand.NewUpdateModelHandler(endpointRepo, modelRepo),
		Delete: modelcommand.NewDeleteModelHandler(modelRepo),
	})
	upstreamHandler := handler.NewUpstreamHandler(handler.UpstreamDependencies{
		List: upstreamquery.NewListUpstreamHandler(endpointRepo, modelRepo, userRepo),
	})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("sync history", "1.0"))
	router.RegisterAPIRouter(api, router.APIRouterDependencies{
		DB:               db,
		Cache:            rdb,
		AccessSigner:     jwt.NewAccessTokenSigner(),
		PingHandler:      &stubSyncPingHandler{},
		TraceHandler:     &stubSyncTraceHandler{},
		TokenHandler:     &stubSyncTokenHandler{},
		Oauth2Handler:    &stubSyncOauth2Handler{},
		UserHandler:      &stubSyncUserHandler{},
		DemoHandler:      &stubSyncDemoHandler{},
		APIKeyHandler:    &stubSyncAPIKeyHandler{},
		SessionHandler:   &stubSyncSessionHandler{},
		EndpointHandler:  endpointHandler,
		ModelHandler:     modelHandler,
		UpstreamHandler:  upstreamHandler,
		AuditHandler:     &stubSyncAuditHandler{},
		CronHandler:      &stubSyncCronHandler{},
		TriggerHandler:   &stubSyncTriggerHandler{},
		OpenAIHandler:    &stubSyncOpenAIHandler{},
		AnthropicHandler: &stubSyncAnthropicHandler{},
		MetricsHandler:   &stubSyncMetricsHandler{},
		DatasetHandler:   &stubSyncDatasetHandler{},
		ClientHandler:    &stubSyncClientHandler{},
	})

	f := &syncHistoryFixture{app: app, db: db, signer: jwt.NewAccessTokenSigner()}
	// github/google_bind_id 各自参与 (bind_id, deleted_at) 唯一索引，零值彼此冲突，须逐个区分
	f.userA = &dbmodel.User{Name: "sync-a", GithubBindID: "gh-sync-a", GoogleBindID: "gg-sync-a", Permission: enum.PermissionUser}
	f.userB = &dbmodel.User{Name: "sync-b", GithubBindID: "gh-sync-b", GoogleBindID: "gg-sync-b", Permission: enum.PermissionUser}
	for _, u := range []*dbmodel.User{f.userA, f.userB} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	f.epA = &dbmodel.Endpoint{UserID: f.userA.ID, Name: "ep-a", APIKey: "sk-a", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true}
	f.epB = &dbmodel.Endpoint{UserID: f.userB.ID, Name: "ep-b", APIKey: "sk-b", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true}
	for _, ep := range []*dbmodel.Endpoint{f.epA, f.epB} {
		if err := db.Create(ep).Error; err != nil {
			t.Fatalf("create endpoint: %v", err)
		}
	}
	// ProxyAPIKey (key, deleted_at) 唯一索引：各 key 赋唯一值
	f.keyA = &dbmodel.ProxyAPIKey{UserID: f.userA.ID, Name: "key-a", Key: "sk-sync-a"}
	f.keyB = &dbmodel.ProxyAPIKey{UserID: f.userB.ID, Name: "key-b", Key: "sk-sync-b"}
	for _, k := range []*dbmodel.ProxyAPIKey{f.keyA, f.keyB} {
		if err := db.Create(k).Error; err != nil {
			t.Fatalf("create api key: %v", err)
		}
	}
	f.msgA = &dbmodel.Message{ModelID: "old-id", CheckSum: "m-sync-a"}
	f.msgB = &dbmodel.Message{ModelID: "old-id", CheckSum: "m-sync-b"}
	for _, m := range []*dbmodel.Message{f.msgA, f.msgB} {
		if err := db.Create(m).Error; err != nil {
			t.Fatalf("create message: %v", err)
		}
	}
	f.sessA = &dbmodel.Session{APIKeyName: "key-a", MessageIDs: []uint{f.msgA.ID}, ModelIDs: []string{"old-id", "other-id"}}
	if err := db.Create(f.sessA).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}
	f.modelA = &dbmodel.Model{UserID: f.userA.ID, Alias: "alias-a", ModelID: "old-id", UpstreamModel: "up-a", EndpointID: f.epA.ID,
		Capabilities: []enum.InputModality{enum.InputModalityText}}
	if err := db.Create(f.modelA).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	auditA := &dbmodel.ModelCallAudit{APIKeyID: f.keyA.ID, ModelID: "old-id"}
	auditB := &dbmodel.ModelCallAudit{APIKeyID: f.keyB.ID, ModelID: "old-id"}
	for _, a := range []*dbmodel.ModelCallAudit{auditA, auditB} {
		if err := db.Create(a).Error; err != nil {
			t.Fatalf("create audit: %v", err)
		}
	}
	return f
}

func (f *syncHistoryFixture) tokenFor(t *testing.T, userID uint) string {
	t.Helper()
	token, err := f.signer.EncodeToken(userID)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	return token
}

func (f *syncHistoryFixture) do(t *testing.T, method, path, token, body string) (status int, data []byte) {
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

// modelUpdateRspBody 响应体：huma 对带 Body 字段的输出结构会把 Body 直接作为响应体（扁平化），
// 同时顶层注入 $schema；错误语义由顶层 error 承载（管理 API 恒 HTTP 200）。
type modelUpdateRspBody struct {
	AuditCount   int64     `json:"auditCount"`
	SessionCount int64     `json:"sessionCount"`
	MessageCount int64     `json:"messageCount"`
	Error        *bizError `json:"error,omitempty"`
}

// patchModel 发 PATCH /model?id= 请求并解析三表影响行数
type modelUpdateCounts struct {
	AuditCount   int64
	SessionCount int64
	MessageCount int64
}

func (f *syncHistoryFixture) patchModel(t *testing.T, modelID uint, token, body string) *modelUpdateCounts {
	t.Helper()
	status, data := f.do(t, http.MethodPatch, fmt.Sprintf("%s/model?id=%d", constant.WebAPIPrefix, modelID), token, body)
	var rsp modelUpdateRspBody
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		t.Fatalf("decode response: %v (raw=%s)", err, data)
	}
	if status != http.StatusOK || rsp.Error != nil {
		t.Fatalf("patch model failed: status=%d body=%s", status, data)
	}
	return &modelUpdateCounts{AuditCount: rsp.AuditCount, SessionCount: rsp.SessionCount, MessageCount: rsp.MessageCount}
}

// TestSyncHistory_ReplacesAndIsolates 同步替换 + 租户隔离
func TestSyncHistory_ReplacesAndIsolates(t *testing.T) {
	t.Parallel()
	f := newSyncHistoryFixture(t)
	tokenA := f.tokenFor(t, f.userA.ID)

	counts := f.patchModel(t, f.modelA.ID, tokenA, `{"modelId":"new-id","syncHistory":true}`)
	if counts.AuditCount != 1 || counts.SessionCount != 1 || counts.MessageCount != 1 {
		t.Fatalf("counts = %+v, want audit=1 session=1 message=1", counts)
	}

	var auditNewA, auditOldB int64
	f.db.Model(&dbmodel.ModelCallAudit{}).
		Where("model_id = ? AND api_key_id = ?", "new-id", f.keyA.ID).Count(&auditNewA)
	f.db.Model(&dbmodel.ModelCallAudit{}).
		Where("model_id = ? AND api_key_id = ?", "old-id", f.keyB.ID).Count(&auditOldB)
	if auditNewA != 1 || auditOldB != 1 {
		t.Fatalf("audit isolation broken: A new=%d, B old=%d", auditNewA, auditOldB)
	}

	var sess dbmodel.Session
	if err := f.db.First(&sess, f.sessA.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(sess.ModelIDs) != 2 || sess.ModelIDs[0] != "new-id" || sess.ModelIDs[1] != "other-id" {
		t.Fatalf("session model_ids = %v, want [new-id other-id]", sess.ModelIDs)
	}

	var msgNew, msgOldB int64
	f.db.Model(&dbmodel.Message{}).Where("id = ? AND model_id = ?", f.msgA.ID, "new-id").Count(&msgNew)
	f.db.Model(&dbmodel.Message{}).Where("id = ? AND model_id = ?", f.msgB.ID, "old-id").Count(&msgOldB)
	if msgNew != 1 || msgOldB != 1 {
		t.Fatalf("message isolation broken: A new=%d, B old=%d", msgNew, msgOldB)
	}
}

// TestSyncHistory_NoChangeReturnsZero modelId 未变时幂等，无副作用
func TestSyncHistory_NoChangeReturnsZero(t *testing.T) {
	t.Parallel()
	f := newSyncHistoryFixture(t)
	tokenA := f.tokenFor(t, f.userA.ID)

	counts := f.patchModel(t, f.modelA.ID, tokenA, `{"modelId":"old-id","syncHistory":true}`)
	if counts.AuditCount != 0 || counts.SessionCount != 0 || counts.MessageCount != 0 {
		t.Fatalf("counts = %+v, want all zero", counts)
	}

	var auditOld int64
	f.db.Model(&dbmodel.ModelCallAudit{}).
		Where("model_id = ? AND api_key_id = ?", "old-id", f.keyA.ID).Count(&auditOld)
	if auditOld != 1 {
		t.Fatalf("history must be untouched, audit old=%d", auditOld)
	}
}

// TestSyncHistory_UncheckedKeepsOld 未带 syncHistory 时历史数据保持旧值
func TestSyncHistory_UncheckedKeepsOld(t *testing.T) {
	t.Parallel()
	f := newSyncHistoryFixture(t)
	tokenA := f.tokenFor(t, f.userA.ID)

	counts := f.patchModel(t, f.modelA.ID, tokenA, `{"modelId":"new-id"}`)
	if counts.AuditCount != 0 || counts.SessionCount != 0 || counts.MessageCount != 0 {
		t.Fatalf("counts = %+v, want all zero", counts)
	}

	var auditOld, sessOld int64
	f.db.Model(&dbmodel.ModelCallAudit{}).
		Where("model_id = ? AND api_key_id = ?", "old-id", f.keyA.ID).Count(&auditOld)
	f.db.Model(&dbmodel.Session{}).Where("id = ? AND model_ids LIKE ?", f.sessA.ID, "%old-id%").Count(&sessOld)
	if auditOld != 1 || sessOld != 1 {
		t.Fatalf("history must keep old id: audit=%d session=%d", auditOld, sessOld)
	}
}
